package context

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// cacheTTL bounds how stale a background-collected context may be before the
// next BuildPrompt call re-collects synchronously.
const cacheTTL = 60 * time.Second

type Engine struct {
	projectDir string

	mu       sync.Mutex
	cached   *Context
	cachedAt time.Time
}

func NewEngine() (*Engine, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return &Engine{projectDir: dir}, nil
}

type Context struct {
	GitDiff     string            `json:"git_diff"`
	GitLog      string            `json:"git_log"`
	Readme      string            `json:"readme"`
	FileTree    string            `json:"file_tree"`
	PackageJSON string            `json:"package_json"`
	Env         map[string]string `json:"env"`
	Errors      string            `json:"errors"`
}

func (e *Engine) Collect() *Context {
	ctx := &Context{
		Env: make(map[string]string),
	}

	ctx.Readme = e.readFileIfExists("README.md")
	ctx.Readme = firstNonEmpty(ctx.Readme, e.readFileIfExists("README.txt"), e.readFileIfExists("README"))

	ctx.PackageJSON = e.readFileIfExists("package.json")
	ctx.PackageJSON = firstNonEmpty(ctx.PackageJSON,
		e.readFileIfExists("Cargo.toml"),
		e.readFileIfExists("pyproject.toml"),
		e.readFileIfExists("go.mod"),
		e.readFileIfExists("requirements.txt"),
	)

	ctx.GitDiff = e.capture("git", "diff", "--stat")
	ctx.GitLog = e.capture("git", "log", "--oneline", "-10")
	ctx.FileTree = e.buildFileTree()
	ctx.Errors = e.capture("git", "status", "--short")

	return ctx
}

func (e *Engine) CollectWithDiff() *Context {
	ctx := e.Collect()
	fullDiff := e.capture("git", "diff")
	if fullDiff != "" {
		ctx.GitDiff = fullDiff
	}
	return ctx
}

// Refresh collects the project context and caches it. It is safe to call
// from a background goroutine (the TUI does this on a timer so BuildPrompt
// never blocks the UI thread on disk scans or git subprocesses).
func (e *Engine) Refresh() *Context {
	ctx := e.Collect()
	e.mu.Lock()
	e.cached = ctx
	e.cachedAt = time.Now()
	e.mu.Unlock()
	return ctx
}

// BuildPrompt returns a prompt wrapped in the freshest cached project
// context. If the cache is older than cacheTTL (or absent) it collects
// synchronously so headless callers never see stale context.
func (e *Engine) BuildPrompt(userPrompt string) string {
	e.mu.Lock()
	ctx := e.cached
	fresh := ctx != nil && time.Since(e.cachedAt) < cacheTTL
	e.mu.Unlock()
	if !fresh {
		ctx = e.Refresh()
	}
	return e.renderPrompt(ctx, userPrompt)
}

func (e *Engine) renderPrompt(ctx *Context, userPrompt string) string {
	var b strings.Builder

	b.WriteString("Project Context:\n")

	if ctx.Readme != "" {
		b.WriteString(fmt.Sprintf("\nREADME:\n%s\n", truncate(ctx.Readme, 500)))
	}
	if ctx.PackageJSON != "" {
		b.WriteString(fmt.Sprintf("\nDependencies:\n%s\n", truncate(ctx.PackageJSON, 500)))
	}
	if ctx.FileTree != "" {
		b.WriteString(fmt.Sprintf("\nFile Tree:\n%s\n", ctx.FileTree))
	}
	if ctx.GitLog != "" {
		b.WriteString(fmt.Sprintf("\nRecent Git History:\n%s\n", ctx.GitLog))
	}
	if ctx.GitDiff != "" {
		b.WriteString(fmt.Sprintf("\nUncommitted Changes:\n%s\n", truncate(ctx.GitDiff, 1000)))
	}
	if ctx.Errors != "" {
		b.WriteString(fmt.Sprintf("\nWorking Tree State:\n%s\n", ctx.Errors))
	}

	b.WriteString(fmt.Sprintf("\nUser Request:\n%s\n", userPrompt))
	return b.String()
}

func (e *Engine) readFileIfExists(path string) string {
	fullPath := filepath.Join(e.projectDir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func (e *Engine) capture(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	cmd.Dir = e.projectDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

func (e *Engine) buildFileTree() string {
	var b strings.Builder
	filepath.Walk(e.projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			rel, _ := filepath.Rel(e.projectDir, path)
			if info != nil && info.IsDir() && strings.HasPrefix(rel, ".") && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(e.projectDir, path)
		if !strings.HasPrefix(rel, ".") {
			b.WriteString(rel + "\n")
		}
		return nil
	})
	return b.String()
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
