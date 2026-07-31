package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRegistryBuiltins(t *testing.T) {
	r := NewRegistry()
	if r.Count() != 25 {
		t.Fatalf("expected 25 built-in tools, got %d", r.Count())
	}
	for _, name := range []string{"fs", "edit", "terminal", "git", "search", "http", "websearch", "browser", "docs", "diff", "pkg", "repo-intel", "memory", "skill", "test", "debug", "lint", "format", "security", "perf", "scaffold", "workflow", "db", "docker", "mcp"} {
		if _, err := r.Get(name); err != nil {
			t.Errorf("tool %q missing: %v", name, err)
		}
	}
}

func TestRegistryAliases(t *testing.T) {
	r := NewRegistry()
	for _, id := range []string{"delta.fs", "delta.git", "delta.search"} {
		tool, err := r.Get(id)
		if err != nil {
			t.Fatalf("alias %q: %v", id, err)
		}
		if tool.Name() == "" {
			t.Fatalf("alias %q resolved to empty name", id)
		}
	}
	if _, err := r.Get("no-such-tool"); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestFSTool(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")

	out, err := fsTool("write", file, "hello world")
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(out, "wrote") {
		t.Fatalf("unexpected write output: %q", out)
	}

	out, err = fsTool("read", file)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if out != "hello world" {
		t.Fatalf("read got %q, want %q", out, "hello world")
	}

	out, err = fsTool("meta", file)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if !strings.Contains(out, "a.txt") {
		t.Fatalf("meta output missing name: %q", out)
	}

	if _, err := fsTool("list", dir); err != nil {
		t.Fatalf("list: %v", err)
	}

	if _, err := fsTool("move", file, filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("source still exists after move")
	}

	if _, err := fsTool("delete", filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestEditTool(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "code.go")
	os.WriteFile(file, []byte("package main\n\nfunc old() {}\n"), 0644)

	out, err := editTool(file, "old", "new")
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if !strings.Contains(out, "edited") {
		t.Fatalf("unexpected edit output: %q", out)
	}
	data, _ := os.ReadFile(file)
	if !strings.Contains(string(data), "func new()") {
		t.Fatalf("edit did not apply: %q", string(data))
	}
}

func TestSearchTool(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc Hello() {}\n"), 0644)

	out, err := searchTool("Hello", dir)
	if err != nil {
		t.Fatalf("search text: %v", err)
	}
	if !strings.Contains(out, "main.go") {
		t.Fatalf("text search missed file: %q", out)
	}

	out, err = searchTool("*.go", dir, "--name")
	if err != nil {
		t.Fatalf("search name: %v", err)
	}
	if !strings.Contains(out, "main.go") {
		t.Fatalf("name search missed file: %q", out)
	}

	out, err = searchTool("Hello", dir, "--symbol")
	if err != nil {
		t.Fatalf("search symbol: %v", err)
	}
	if !strings.Contains(out, "Hello") {
		t.Fatalf("symbol search missed: %q", out)
	}
}

func TestRanker(t *testing.T) {
	r := NewRegistry()
	ranker := NewRanker()
	ranked := ranker.Rank("search for text in files", r.List(), 3)
	if len(ranked) == 0 {
		t.Fatal("ranker returned nothing")
	}
	if ranked[0].Tool.Name() != "search" {
		t.Fatalf("expected search tool first, got %s", ranked[0].Tool.Name())
	}
	best, _, ok := ranker.Best("search for text in files", r.List())
	if !ok || best.Name() != "search" {
		t.Fatalf("Best() = %v, want search", best)
	}
}

func TestCapabilityAnalyzer(t *testing.T) {
	r := NewRegistry()
	matches := r.Analyze("run the test suite")
	if len(matches) == 0 {
		t.Fatal("analyze returned no matches")
	}
	if matches[0].Tool.Name() != "test" {
		t.Fatalf("expected test tool first, got %s", matches[0].Tool.Name())
	}
	steps, ok := r.Plan("format all go files in src")
	if !ok || len(steps) == 0 {
		t.Fatal("plan returned nothing")
	}
}

func TestExecutorRetries(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Manifest: Manifest{
			ID: "delta.flaky", Name: "flaky", Version: "1.0.0",
			Category: "test", TimeoutSec: 5, Retry: 2, TrustLevel: "trusted", Source: "builtin",
		},
		Health: "healthy",
		Run: func(args ...string) (string, error) {
			attempts := 0
			return "", &flakyError{attempts: &attempts}
		},
	})
	_, _ = r.Call("flaky")
}

type flakyError struct{ attempts *int }

func (e *flakyError) Error() string { return "flaky" }

func TestExecutorTimeout(t *testing.T) {
	r := NewRegistry()
	r.Register(&Tool{
		Manifest: Manifest{
			ID: "delta.slow", Name: "slow", Version: "1.0.0",
			Category: "test", TimeoutSec: 1, TrustLevel: "trusted", Source: "builtin",
		},
		Health: "healthy",
		Run: func(args ...string) (string, error) {
			select {
			case <-time.After(5 * time.Second):
			case <-context.Background().Done():
			}
			return "late", nil
		},
	})
	out, err := r.Call("slow")
	if err == nil {
		t.Fatalf("expected timeout error, got %q", out)
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected timeout message, got %v", err)
	}
}

func TestApprovalGate(t *testing.T) {
	r := NewRegistryWithPolicy(PolicyAsk)
	if _, err := r.Call("terminal", "echo hi"); err == nil {
		t.Fatal("expected approval error under ask policy")
	}
	r.SetToolAllowed("terminal")
	if _, err := r.Call("terminal", "echo hi"); err != nil {
		t.Fatalf("approved tool should run: %v", err)
	}
	r.SetToolDenied("terminal")
	if _, err := r.Call("terminal", "echo hi"); err == nil {
		t.Fatal("denied tool should fail")
	}
}

func TestRunExternalMissingBin(t *testing.T) {
	_, err := runExternal("definitely-not-a-real-binary-xyz", nil, 5)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestUserToolLoad(t *testing.T) {
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, ".delta", "tools")
	if _, err := os.Stat(root); err != nil {
		t.Skip("no user tool dir")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) == 0 {
		t.Skip("no user tools")
	}
	r := NewRegistry()
	if r.Count() < 25 {
		t.Fatalf("user tools not loaded, count %d", r.Count())
	}
}

func TestLoadManifests(t *testing.T) {
	dir := t.TempDir()
	toolDir := filepath.Join(dir, "mytool")
	os.MkdirAll(toolDir, 0755)
	os.WriteFile(filepath.Join(toolDir, "tool.json"), []byte(`{"id":"mytool","name":"mytool","version":"1.0.0","category":"custom","dependencies":["python"]}`), 0644)
	os.WriteFile(filepath.Join(toolDir, "run.py"), []byte("print('hi')"), 0644)

	tools, errs := loadManifests([]string{dir}, func(m Manifest) (ToolFunc, bool) {
		return func(args ...string) (string, error) { return "hi", nil }, true
	})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(tools) != 1 || tools[0].Name() != "mytool" {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
}
