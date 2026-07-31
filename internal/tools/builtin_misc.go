package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------- Performance Tool ----------

func perfTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("perf: action required (profile|analyze|startup|bundle|query)")
	}
	action := args[0]
	switch action {
	case "profile":
		if len(args) < 2 {
			return "", fmt.Errorf("perf profile: target required (go|python|node)")
		}
		switch args[1] {
		case "go":
			return runExternal("go", []string{"tool", "pprof", "-top", "profile.pprof"}, 60)
		case "python":
			return "", fmt.Errorf("perf profile python: run `python -m cProfile -o out.prof script.py` then use analyze")
		default:
			return "", fmt.Errorf("perf profile: unsupported target %q", args[1])
		}
	case "analyze":
		if len(args) < 2 {
			return "", fmt.Errorf("perf analyze: profile file required")
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			return "", err
		}
		lines := strings.Split(string(data), "\n")
		var top []string
		for _, line := range lines {
			if strings.Contains(line, "function calls") || strings.Contains(line, "ncalls") {
				top = append(top, strings.TrimSpace(line))
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if _, err := fmt.Sscanf(fields[0], "%f", new(float64)); err == nil {
					top = append(top, strings.TrimSpace(line))
				}
			}
			if len(top) >= 15 {
				break
			}
		}
		if len(top) == 0 {
			return "unrecognized profile format", nil
		}
		return strings.Join(top, "\n"), nil
	case "startup":
		return "", fmt.Errorf("perf startup: run your binary with timing and use analyze")
	case "bundle":
		return runExternal("npx", []string{"webpack", "--analyze"}, 180)
	case "query":
		return "", fmt.Errorf("perf query: use the db tool EXPLAIN for query plans")
	default:
		return "", fmt.Errorf("perf: unknown action %q", action)
	}
}

// ---------- Project Generator Tool ----------

type scaffoldTemplate struct {
	Name  string
	Files map[string]string
}

var scaffoldTemplates = []scaffoldTemplate{
	{
		Name: "go-cli",
		Files: map[string]string{
			"go.mod": "module {name}\n\ngo 1.26\n",
			"main.go": `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: {name} <arg>")
		os.Exit(1)
	}
	fmt.Println("hello from {name}:", os.Args[1])
}
`,
			"main_test.go": `package main

import "testing"

func TestMain(t *testing.T) {
	t.Log("smoke test")
}
`,
			".gitignore": "bin/\ndist/\n",
		},
	},
	{
		Name: "python-cli",
		Files: map[string]string{
			"main.py": `#!/usr/bin/env python3
import argparse

def main() -> None:
    parser = argparse.ArgumentParser(description="{name}")
    parser.add_argument("--version", action="version", version="0.1.0")
    parser.add_argument("input", nargs="*")
    args = parser.parse_args()
    print("hello from {name}", args.input)

if __name__ == "__main__":
    main()
`,
			"test_main.py": `def test_smoke():
    assert True
`,
			"requirements.txt": "",
			".gitignore": "__pycache__/\n.venv/\n",
		},
	},
	{
		Name: "node-cli",
		Files: map[string]string{
			"package.json": `{
  "name": "{name}",
  "version": "0.1.0",
  "description": "",
  "main": "index.js",
  "bin": { "{name}": "./index.js" },
  "scripts": {
    "test": "node --test"
  }
}
`,
			"index.js": `#!/usr/bin/env node
const args = process.argv.slice(2);
if (args.length === 0) {
  console.error("usage: {name} <arg>");
  process.exit(1);
}
console.log("hello from {name}:", args.join(" "));
`,
			"index.test.js": `const test = require("node:test");
test("smoke", () => {});
`,
			".gitignore": "node_modules/\ndist/\n",
		},
	},
	{
		Name: "go-api",
		Files: map[string]string{
			"go.mod": "module {name}\n\ngo 1.26\n",
			"main.go": `package main

import (
	"fmt"
	"log"
	"net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "hello from {name}")
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", helloHandler)
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
`,
			"main_test.go": `package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHello(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	helloHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
`,
		},
	},
}

func scaffoldTool(args ...string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("scaffold: template and target dir required (templates: go-cli, python-cli, node-cli, go-api)")
	}
	name := args[0]
	target := args[1]
	var tmpl *scaffoldTemplate
	for i := range scaffoldTemplates {
		if scaffoldTemplates[i].Name == name {
			tmpl = &scaffoldTemplates[i]
			break
		}
	}
	if tmpl == nil {
		var names []string
		for _, t := range scaffoldTemplates {
			names = append(names, t.Name)
		}
		return "", fmt.Errorf("scaffold: unknown template %q (available: %s)", name, strings.Join(names, ", "))
	}
	if err := os.MkdirAll(target, 0755); err != nil {
		return "", err
	}
	projName := filepath.Base(target)
	for rel, content := range tmpl.Files {
		full := filepath.Join(target, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			return "", err
		}
		content = strings.ReplaceAll(content, "{name}", projName)
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("scaffolded %s project at %s", name, target), nil
}

// ---------- Workflow Tool ----------

type Workflow struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Steps   []WorkflowStep `json:"steps"`
	Created time.Time    `json:"created_at"`
}

type WorkflowStep struct {
	Tool string   `json:"tool"`
	Args []string `json:"args"`
}

func (r *Registry) workflowPath() string {
	home := userToolsHome()
	return filepath.Join(home, "workflows")
}

func (r *Registry) workflowTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("workflow: action required (run|save|list|replay|delete)")
	}
	dir := r.workflowPath()
	os.MkdirAll(dir, 0755)
	action := args[0]
	switch action {
	case "save":
		if len(args) < 2 {
			return "", fmt.Errorf("workflow save: json required ({\"name\":\"...\",\"steps\":[{\"tool\":\"...\",\"args\":[...]}]})")
		}
		var wf Workflow
		if err := json.Unmarshal([]byte(args[1]), &wf); err != nil {
			return "", fmt.Errorf("workflow save: invalid json: %w", err)
		}
		if wf.Name == "" {
			return "", fmt.Errorf("workflow save: name required")
		}
		wf.ID = fmt.Sprintf("wf-%d", time.Now().UnixNano())
		wf.Created = time.Now()
		data, _ := json.MarshalIndent(wf, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, wf.Name+".json"), data, 0600); err != nil {
			return "", err
		}
		return fmt.Sprintf("saved workflow %s (%d steps)", wf.Name, len(wf.Steps)), nil
	case "list":
		entries, err := os.ReadDir(dir)
		if err != nil {
			return "no workflows", nil
		}
		var out []string
		for _, e := range entries {
			out = append(out, strings.TrimSuffix(e.Name(), ".json"))
		}
		sort.Strings(out)
		if len(out) == 0 {
			return "no workflows", nil
		}
		return strings.Join(out, "\n"), nil
	case "run", "replay":
		if len(args) < 2 {
			return "", fmt.Errorf("workflow %s: workflow name required", action)
		}
		data, err := os.ReadFile(filepath.Join(dir, args[1]+".json"))
		if err != nil {
			return "", fmt.Errorf("workflow %s not found", args[1])
		}
		var wf Workflow
		if err := json.Unmarshal(data, &wf); err != nil {
			return "", err
		}
		var out []string
		for i, step := range wf.Steps {
			result, err := r.Call(step.Tool, step.Args...)
			if err != nil {
				return strings.Join(out, "\n"), fmt.Errorf("step %d (%s) failed: %w", i+1, step.Tool, err)
			}
			out = append(out, fmt.Sprintf("[%d/%d] %s: %s", i+1, len(wf.Steps), step.Tool, truncateErr(result)))
		}
		return strings.Join(out, "\n"), nil
	case "delete":
		if len(args) < 2 {
			return "", fmt.Errorf("workflow delete: name required")
		}
		if err := os.Remove(filepath.Join(dir, args[1]+".json")); err != nil {
			return "", err
		}
		return fmt.Sprintf("deleted workflow %s", args[1]), nil
	default:
		return "", fmt.Errorf("workflow: unknown action %q", action)
	}
}

// ---------- Database Tool ----------

func dbTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("db: engine required (sqlite|postgres|mysql|mongodb|redis) plus action (query|explain|schema|backup)")
	}
	engine := args[0]
	if len(args) < 2 {
		return "", fmt.Errorf("db %s: action required", engine)
	}
	action := args[1]
	switch engine {
	case "sqlite":
		if len(args) < 3 {
			return "", fmt.Errorf("db sqlite: database file required")
		}
		dbfile := args[2]
		switch action {
		case "query":
			if len(args) < 4 {
				return "", fmt.Errorf("db sqlite query: sql required")
			}
			return runExternal("sqlite3", []string{dbfile, strings.Join(args[3:], " ")}, 60)
		case "schema":
			return runExternal("sqlite3", []string{dbfile, ".schema"}, 60)
		case "tables":
			return runExternal("sqlite3", []string{dbfile, ".tables"}, 60)
		default:
			return "", fmt.Errorf("db sqlite: unknown action %q", action)
		}
	default:
		return "", fmt.Errorf("db %s: engine not configured; use sqlite (requires sqlite3 CLI)", engine)
	}
}

// ---------- Docker Tool ----------

func dockerTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("docker: subcommand required (build|run|stop|logs|ps|images|compose|health)")
	}
	switch args[0] {
	case "health":
		out, err := runExternal("docker", []string{"info"}, 30)
		if err != nil {
			return "docker unavailable", nil
		}
		return "docker healthy:\n" + out, nil
	case "build":
		return runExternal("docker", append([]string{"build", "-t"}, args[1:]...), 600)
	case "run":
		return runExternal("docker", append([]string{"run"}, args[1:]...), 300)
	case "stop":
		return runExternal("docker", append([]string{"stop"}, args[1:]...), 60)
	case "logs":
		return runExternal("docker", append([]string{"logs"}, args[1:]...), 60)
	case "ps":
		return runExternal("docker", append([]string{"ps"}, args[1:]...), 60)
	case "images":
		return runExternal("docker", []string{"images"}, 60)
	case "compose":
		return runExternal("docker", append([]string{"compose"}, args[1:]...), 300)
	case "rm":
		return runExternal("docker", append([]string{"rm"}, args[1:]...), 60)
	default:
		return runExternal("docker", args, 120)
	}
}

// ---------- MCP Tool ----------

func mcpTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("mcp: action required (health|discover)")
	}
	switch args[0] {
	case "health":
		return "mcp: no servers configured; configure MCP servers in config", nil
	case "discover":
		return "mcp: discovery requires configured MCP servers", nil
	default:
		return "", fmt.Errorf("mcp: use `delta mcp` for MCP server management")
	}
}
