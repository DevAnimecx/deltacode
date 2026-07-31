package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/intelligence"
	"github.com/DevAnimecx/deltacode/internal/symbols"
)

// ---------- Repository Intelligence Tool ----------

func repoIntelTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("repointel: action required (index|symbols|graph|calls|imports|top|files)")
	}
	root := "."
	action := args[0]
	if len(args) > 1 {
		root = args[len(args)-1]
		if strings.HasPrefix(args[1], "--") {
			root = "."
		}
	}
	idx := symbols.NewIndexer(root)
	if err := idx.IndexDirectory(root, nil); err != nil {
		return "", err
	}
	g := idx.Graph()

	switch action {
	case "index":
		return fmt.Sprintf("indexed %d symbols, %d files", g.Count(), len(indexedFiles(g))), nil
	case "symbols":
		nameFilter := ""
		if len(args) > 1 && !strings.HasPrefix(args[1], "--") {
			nameFilter = strings.ToLower(args[1])
		}
		var out []string
		for _, s := range g.GetAllSymbols() {
			if nameFilter != "" && !strings.Contains(strings.ToLower(s.Name), nameFilter) {
				continue
			}
			out = append(out, fmt.Sprintf("%s:%d  %-8s %s", s.FilePath, s.Line, s.Kind, s.Name))
		}
		sort.Strings(out)
		if len(out) == 0 {
			return "no symbols", nil
		}
		return strings.Join(out, "\n"), nil
	case "graph":
		var out []string
		for _, e := range g.GetAllSymbols() {
			for _, callee := range g.GetCallees(e.ID) {
				if c, ok := g.GetSymbol(callee); ok {
					out = append(out, fmt.Sprintf("%s -> %s", e.Name, c.Name))
				}
			}
		}
		if len(out) == 0 {
			return "no call edges", nil
		}
		return strings.Join(out, "\n"), nil
	case "calls":
		name := ""
		if len(args) > 1 {
			name = args[1]
		}
		var out []string
		for _, s := range g.GetAllSymbols() {
			if name != "" && !strings.Contains(strings.ToLower(s.Name), strings.ToLower(name)) {
				continue
			}
			callees := g.GetCallees(s.ID)
			callers := g.GetCallers(s.ID)
			if len(callees) == 0 && len(callers) == 0 {
				continue
			}
			out = append(out, fmt.Sprintf("%s: calls=%d called_by=%d", s.Name, len(callees), len(callers)))
		}
		sort.Slice(out, func(i, j int) bool {
			return callsNum(out[i]) > callsNum(out[j])
		})
		if len(out) == 0 {
			return "no call data", nil
		}
		return strings.Join(out, "\n"), nil
	case "imports":
		var out []string
		for _, f := range indexedFiles(g) {
			for _, imp := range g.GetImports(f) {
				out = append(out, fmt.Sprintf("%s -> %s", f, imp.Imported))
			}
		}
		if len(out) == 0 {
			return "no imports", nil
		}
		return strings.Join(out, "\n"), nil
	case "top":
		limit := 10
		if len(args) > 1 {
			fmt.Sscanf(args[1], "%d", &limit)
		}
		var out []string
		for _, s := range g.TopSymbols(limit) {
			out = append(out, fmt.Sprintf("%-40s %s", s.Name, s.FilePath))
		}
		if len(out) == 0 {
			return "no symbols", nil
		}
		return strings.Join(out, "\n"), nil
	case "files":
		files := indexedFiles(g)
		if len(files) == 0 {
			return "no indexed files", nil
		}
		return strings.Join(files, "\n"), nil
	default:
		return "", fmt.Errorf("repointel: unknown action %q", action)
	}
}

func indexedFiles(g *symbols.SymbolGraph) []string {
	seen := map[string]bool{}
	for _, s := range g.GetAllSymbols() {
		seen[s.FilePath] = true
	}
	var out []string
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

func callsNum(line string) int {
	idx := strings.Index(line, "calls=")
	if idx == -1 {
		return 0
	}
	n := 0
	fmt.Sscanf(line[idx+6:], "%d", &n)
	return n
}

// ---------- Memory Tool ----------

func memoryTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("memory: action required (save|get|search|list|expire|pin|compress)")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	mem := intelligence.NewMemoryAt(filepath.Join(home, ".delta", "memory", "tools.json"))
	action := args[0]

	switch action {
	case "save":
		if len(args) < 4 {
			return "", fmt.Errorf("memory save: layer key content required")
		}
		layer := intelligence.Layer(args[1])
		key := args[2]
		content := strings.Join(args[3:], " ")
		if err := mem.StoreEx(layer, key, content, intelligence.StoreOptions{
			Tags: []string{"tool"}, Priority: 0.4, Source: "memory-tool",
		}); err != nil {
			return "", err
		}
		return fmt.Sprintf("saved %s/%s", layer, key), nil
	case "get":
		if len(args) < 3 {
			return "", fmt.Errorf("memory get: layer key required")
		}
		if v, ok := mem.Get(intelligence.Namespace(args[1]), args[2]); ok {
			return v, nil
		}
		return "not found", nil
	case "search":
		if len(args) < 2 {
			return "", fmt.Errorf("memory search: query required")
		}
		query := strings.Join(args[1:], " ")
		results := mem.Search(query, 10)
		if len(results) == 0 {
			return "no memory matches", nil
		}
		var out []string
		for _, r := range results {
			out = append(out, fmt.Sprintf("[%0.2f] %s/%s: %s", r.Score, r.Entry.Layer, r.Entry.Key, truncateErr(r.Entry.Content)))
		}
		return strings.Join(out, "\n"), nil
	case "list":
		layers := mem.ListLayers()
		if len(layers) == 0 {
			return "no memory", nil
		}
		var out []string
		for _, l := range layers {
			entries := mem.RecallRecent(l, 5)
			for _, e := range entries {
				out = append(out, fmt.Sprintf("%-10s %-20s %s", l, e.Key, truncateErr(e.Content)))
			}
		}
		return strings.Join(out, "\n"), nil
	case "expire":
		n := mem.PruneExpired()
		return fmt.Sprintf("expired %d entries", n), nil
	case "pin":
		if len(args) < 3 {
			return "", fmt.Errorf("memory pin: layer key required")
		}
		mem.SetConfidence(intelligence.Namespace(args[1]), args[2], 1.0)
		return fmt.Sprintf("pinned %s/%s", args[1], args[2]), nil
	case "compress":
		return "", fmt.Errorf("memory compress: run `delta memory` for full memory management")
	default:
		return "", fmt.Errorf("memory: unknown action %q", action)
	}
}

// ---------- Skill Tool ----------

func skillTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("skill: action required (search|list|export|rate)")
	}
	eng := intelligence.NewSkillEngine()
	switch args[0] {
	case "search":
		if len(args) < 2 {
			return "", fmt.Errorf("skill search: query required")
		}
		query := strings.Join(args[1:], " ")
		matches := eng.Find(query)
		if len(matches) == 0 {
			return fmt.Sprintf("No skills found for %q", query), nil
		}
		var lines []string
		for _, s := range matches {
			lines = append(lines, fmt.Sprintf("  %s (uses: %d)", s.Name, s.UsageCount))
		}
		return "Skills:\n" + strings.Join(lines, "\n"), nil
	case "list":
		return "Skills managed via `delta skill list` command", nil
	default:
		return "", fmt.Errorf("skill: use the `delta skill` command for full skill management")
	}
}

// ---------- Test Tool ----------

func testTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("test: framework required (go|pytest|npm|jest|cargo) optional: --coverage --watch")
	}
	framework := args[0]
	coverage := false
	watch := false
	var extra []string
	for _, a := range args[1:] {
		switch a {
		case "--coverage":
			coverage = true
		case "--watch":
			watch = true
		default:
			extra = append(extra, a)
		}
	}
	switch framework {
	case "go":
		sub := []string{"test"}
		if coverage {
			sub = append(sub, "-cover")
		}
		sub = append(sub, "./...")
		sub = append(sub, extra...)
		return runExternal("go", sub, 300)
	case "pytest", "python":
		sub := []string{"-m", "pytest"}
		if coverage {
			sub = append(sub, "--cov")
		}
		sub = append(sub, extra...)
		return runExternal("python", sub, 300)
	case "npm":
		sub := []string{"test"}
		if coverage {
			sub = append(sub, "--coverage")
		}
		if watch {
			sub = append(sub, "--watch")
		}
		sub = append(sub, extra...)
		return runExternal("npm", sub, 300)
	case "jest":
		sub := []string{"jest"}
		if coverage {
			sub = append(sub, "--coverage")
		}
		if watch {
			sub = append(sub, "--watch")
		}
		sub = append(sub, extra...)
		return runExternal("npx", sub, 300)
	case "cargo":
		sub := []string{"test"}
		if coverage {
			sub = append(sub, "--coverage")
		}
		sub = append(sub, extra...)
		return runExternal("cargo", sub, 300)
	default:
		return "", fmt.Errorf("test: unsupported framework %q", framework)
	}
}

// ---------- Debug Tool ----------

func debugTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("debug: mode required (stack|log|crash|inspect)")
	}
	mode := args[0]
	if len(args) < 2 {
		return "", fmt.Errorf("debug %s: source required", mode)
	}
	source := args[1]
	switch mode {
	case "stack":
		data, err := os.ReadFile(source)
		if err != nil {
			return "", err
		}
		return analyzeStack(string(data)), nil
	case "log":
		data, err := os.ReadFile(source)
		if err != nil {
			return "", err
		}
		return summarizeLog(string(data)), nil
	case "crash":
		return "", fmt.Errorf("debug crash: attach a core dump or crash log to a stack analysis")
	case "inspect":
		return "", fmt.Errorf("debug inspect: requires a runtime; use terminal tool with your runtime's inspector")
	default:
		return "", fmt.Errorf("debug: unknown mode %q", mode)
	}
}

func analyzeStack(stack string) string {
	var frames []string
	lines := strings.Split(stack, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		isFrame := strings.Contains(trimmed, ".go:") || strings.Contains(trimmed, ".py:") ||
			strings.Contains(trimmed, ".js:") || strings.Contains(trimmed, ".rs:")
		if isFrame && i > 0 {
			frames = append(frames, strings.TrimSpace(lines[i-1])+"  ->  "+trimmed)
		}
		if len(frames) >= 12 {
			break
		}
	}
	firstLine := lines[0]
	root := "unknown"
	if len(lines) > 1 {
		root = strings.TrimSpace(lines[1])
	}
	var out strings.Builder
	out.WriteString("Panic: " + firstLine + "\n")
	out.WriteString("Root frame: " + root + "\n")
	if len(frames) > 0 {
		out.WriteString("Trace:\n" + strings.Join(frames, "\n"))
	}
	return out.String()
}

func summarizeLog(content string) string {
	var errors []string
	var warnings []string
	for _, line := range strings.Split(content, "\n") {
		low := strings.ToLower(line)
		switch {
		case strings.Contains(low, "error"), strings.Contains(low, "fatal"), strings.Contains(low, "exception"):
			errors = append(errors, strings.TrimSpace(line))
		case strings.Contains(low, "warn"), strings.Contains(low, "deprecated"):
			warnings = append(warnings, strings.TrimSpace(line))
		}
	}
	var out strings.Builder
	if len(errors) > 0 {
		out.WriteString(fmt.Sprintf("%d errors:\n", len(errors)))
		for _, e := range errors[:min(len(errors), 20)] {
			out.WriteString("  ! " + truncateErr(e) + "\n")
		}
	}
	if len(warnings) > 0 {
		out.WriteString(fmt.Sprintf("\n%d warnings:\n", len(warnings)))
		for _, w := range warnings[:min(len(warnings), 10)] {
			out.WriteString("  ~ " + truncateErr(w) + "\n")
		}
	}
	if out.Len() == 0 {
		return "no errors or warnings in log"
	}
	return strings.TrimSpace(out.String())
}

// ---------- Linter Tool ----------

func lintTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("lint: language required (go|python|js|rust) optional: --fix")
	}
	lang := args[0]
	fix := false
	for _, a := range args[1:] {
		if a == "--fix" {
			fix = true
		}
	}
	switch lang {
	case "go":
		if fix {
			return "", fmt.Errorf("lint: go vet has no --fix; use format tool")
		}
		return runExternal("go", []string{"vet", "./..."}, 180)
	case "python":
		bin := "ruff"
		sub := []string{"check"}
		if fix {
			sub = append(sub, "--fix")
		}
		sub = append(sub, ".")
		if _, err := runExternal("ruff", []string{"--version"}, 15); err != nil {
			return "", fmt.Errorf("lint: ruff not installed; pip install ruff")
		}
		return runExternal(bin, sub, 120)
	case "js", "javascript", "ts", "typescript":
		if fix {
			return "", fmt.Errorf("lint: use eslint --fix via terminal tool")
		}
		return runExternal("npx", []string{"eslint", "."}, 180)
	case "rust":
		sub := []string{"clippy"}
		if fix {
			sub = append(sub, "--fix")
		}
		return runExternal("cargo", sub, 300)
	default:
		return "", fmt.Errorf("lint: unsupported language %q", lang)
	}
}

// ---------- Formatter Tool ----------

func formatTool(args ...string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("format: language and path required (go|python|js|rust|c|cpp)")
	}
	lang := args[0]
	path := args[1]
	switch lang {
	case "go":
		return runExternal("gofmt", []string{"-w", path}, 30)
	case "python":
		if _, err := runExternal("black", []string{"--version"}, 15); err != nil {
			return "", fmt.Errorf("format: black not installed; pip install black")
		}
		return runExternal("black", []string{"-q", path}, 60)
	case "js", "ts", "jsx", "tsx", "css", "html", "json", "yaml", "md":
		if _, err := runExternal("npx", []string{"prettier", "--version"}, 30); err != nil {
			return "", fmt.Errorf("format: prettier not installed; npm i -g prettier")
		}
		return runExternal("npx", []string{"prettier", "--write", path}, 60)
	case "rust":
		return runExternal("rustfmt", []string{path}, 60)
	case "c", "cpp", "h", "hpp":
		if _, err := runExternal("clang-format", []string{"--version"}, 15); err != nil {
			return "", fmt.Errorf("format: clang-format not installed")
		}
		return runExternal("clang-format", []string{"-i", path}, 60)
	default:
		return "", fmt.Errorf("format: unsupported language %q", lang)
	}
}

// ---------- Security Tool ----------

func securityTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("security: action required (scan|audit|license|sbom)")
	}
	action := args[0]
	switch action {
	case "scan":
		root := "."
		if len(args) > 1 {
			root = args[1]
		}
		return scanSecrets(root), nil
	case "audit":
		manager := "npm"
		if len(args) > 1 {
			manager = args[1]
		}
		switch manager {
		case "npm":
			return runExternal("npm", []string{"audit"}, 180)
		case "pip":
			return runExternal("pip", []string{"audit"}, 180)
		default:
			return "", fmt.Errorf("security audit: unsupported manager %q", manager)
		}
	case "license":
		return "", fmt.Errorf("security license: run `npx license-checker` for npm projects")
	case "sbom":
		return "", fmt.Errorf("security sbom: run `npx @cyclonedx/cyclonedx-npm` for npm projects")
	default:
		return "", fmt.Errorf("security: unknown action %q", action)
	}
}

func scanSecrets(root string) string {
	patterns := []struct {
		name  string
		regex *regexp.Regexp
	}{{
		name:  "AWS key",
		regex: regexpMustCompile(`AKIA[0-9A-Z]{16}`),
	}, {
		name:  "GitHub token",
		regex: regexpMustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`),
	}, {
		name:  "Private key block",
		regex: regexpMustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	}, {
		name:  "Generic secret",
		regex: regexpMustCompile(`(?i)(api[_-]?key|secret|token|password)\s*[:=]\s*['"][^'"]{12,}['"]`),
	}, {
		name:  "Connection string",
		regex: regexpMustCompile(`(?i)(mongodb|postgres|mysql|redis)://[^\s]{6,}:[^\s@]{6,}@`),
	}}
	var findings []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if skipDirOrFile(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) > 2<<20 {
			return nil
		}
		for _, p := range patterns {
			if p.regex.Match(data) {
				findings = append(findings, fmt.Sprintf("%s: possible %s", path, p.name))
			}
		}
		return nil
	})
	if len(findings) == 0 {
		return "no secrets found"
	}
	return strings.Join(findings, "\n")
}

func regexpMustCompile(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}
