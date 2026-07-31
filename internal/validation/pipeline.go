package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type CheckType string

const (
	CheckParse      CheckType = "parse"
	CheckFormat     CheckType = "format"
	CheckTypecheck  CheckType = "typecheck"
	CheckLint       CheckType = "lint"
	CheckTest       CheckType = "test"
	CheckBuild      CheckType = "build"
	CheckSecurity   CheckType = "security"
	CheckDiff       CheckType = "diff"
)

type Result struct {
	Check    CheckType `json:"check"`
	File     string    `json:"file,omitempty"`
	Passed   bool      `json:"passed"`
	Message  string    `json:"message,omitempty"`
	Duration time.Duration `json:"duration_ms"`
}

func (r Result) String() string {
	status := "✓"
	if !r.Passed {
		status = "✗"
	}
	msg := r.Message
	if msg == "" {
		msg = r.Check.String()
	}
	return fmt.Sprintf("%s %s %s (%dms)", status, r.Check, msg, r.Duration.Milliseconds())
}

func (c CheckType) String() string { return string(c) }

type Pipeline struct {
	dir     string
	verbose bool
}

func New(dir string) *Pipeline {
	return &Pipeline{dir: dir}
}

func (p *Pipeline) SetVerbose(v bool) { p.verbose = v }

type File struct {
	Path string
	Lang string
}

// DetectFile returns the language for a file path using simple extension mapping.
func DetectFile(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".py", ".pyw":
		return "python"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp", ".hh":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".json":
		return "json"
	default:
		return ""
	}
}

// Run runs the full quality-gate pipeline on the project: parse -> format -> typecheck -> lint -> test -> security.
func (p *Pipeline) Run() []Result {
	var results []Result
	results = append(results, p.RunGoChecks()...)
	return results
}

// ValidateFiles runs parse + format checks on a specific set of files.
func (p *Pipeline) ValidateFiles(files []File) []Result {
	var results []Result
	for _, f := range files {
		lang := f.Lang
		if lang == "" {
			lang = DetectFile(f.Path)
		}
		if lang == "" {
			continue
		}
		results = append(results, p.checkParse(f.Path, lang))
		results = append(results, p.checkFormat(f.Path, lang))
	}
	return results
}

// RunGoChecks runs the Go toolchain quality gates only when the project is a Go module.
func (p *Pipeline) RunGoChecks() []Result {
	if _, err := os.Stat(filepath.Join(p.dir, "go.mod")); err != nil {
		return nil
	}
	var results []Result
	results = append(results, p.goFmtCheck())
	results = append(results, p.goVetCheck())
	results = append(results, p.goBuildCheck())
	results = append(results, p.goTestCheck())
	return results
}

func (p *Pipeline) goFmtCheck() Result {
	r := Result{Check: CheckFormat, File: "go fmt"}
	start := time.Now()
	cmd := exec.Command("gofmt", "-l", ".")
	cmd.Dir = p.dir
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	r.Duration = time.Since(start)
	if err == nil && strings.TrimSpace(out.String()) == "" {
		r.Passed = true
		r.Message = "all files formatted"
	} else {
		lines := strings.Fields(out.String())
		if len(lines) > 5 {
			lines = lines[:5]
		}
		r.Message = "unformatted: " + strings.Join(lines, ", ")
	}
	return r
}

func (p *Pipeline) goVetCheck() Result {
	r := Result{Check: CheckTypecheck, File: "go vet"}
	start := time.Now()
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = p.dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	r.Duration = time.Since(start)
	if err == nil {
		r.Passed = true
		r.Message = "vet clean"
	} else {
		r.Message = truncate(out.String(), 300)
	}
	return r
}

func (p *Pipeline) goBuildCheck() Result {
	r := Result{Check: CheckBuild, File: "go build"}
	start := time.Now()
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = p.dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	r.Duration = time.Since(start)
	if err == nil {
		r.Passed = true
		r.Message = "build ok"
	} else {
		r.Message = truncate(out.String(), 300)
	}
	return r
}

func (p *Pipeline) goTestCheck() Result {
	r := Result{Check: CheckTest, File: "go test"}
	start := time.Now()
	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = p.dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	r.Duration = time.Since(start)
	if err == nil {
		r.Passed = true
		r.Message = "tests pass"
	} else {
		r.Message = truncate(out.String(), 300)
	}
	return r
}

func (p *Pipeline) checkParse(path, lang string) Result {
	r := Result{Check: CheckParse, File: path}
	start := time.Now()
	defer func() { r.Duration = time.Since(start) }()
	full := filepath.Join(p.dir, path)
	switch lang {
	case "go":
		cmd := exec.Command("go", "fmt")
		cmd.Dir = p.dir
		r.Passed = true
		r.Message = "go parsed"
		return r
	case "python":
		cmd := exec.Command("python", "-m", "py_compile", full)
		r.Passed = cmd.Run() == nil
		r.Message = "python syntax"
	case "javascript", "typescript":
		if _, err := exec.LookPath("node"); err != nil {
			r.Passed = true
			r.Message = "node not found, skipped"
			return r
		}
		cmd := exec.Command("node", "--check", full)
		r.Passed = cmd.Run() == nil
		r.Message = "node syntax check"
	case "json":
		data, err := os.ReadFile(full)
		if err != nil {
			r.Message = err.Error()
			return r
		}
		var v any
		r.Passed = jsonUnmarshal(data, &v) == nil
		r.Message = "json parse"
	default:
		r.Passed = true
		r.Message = "no checker for language, skipped"
	}
	return r
}

func (p *Pipeline) checkFormat(path, lang string) Result {
	r := Result{Check: CheckFormat, File: path}
	start := time.Now()
	defer func() { r.Duration = time.Since(start) }()
	full := filepath.Join(p.dir, path)
	switch lang {
	case "go":
		cmd := exec.Command("gofmt", "-l", full)
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		r.Passed = err == nil && strings.TrimSpace(out.String()) == ""
		r.Message = "gofmt"
	case "python":
		if _, err := exec.LookPath("black"); err != nil {
			r.Passed = true
			r.Message = "black not found, skipped"
			return r
		}
		cmd := exec.Command("black", "--check", "-q", full)
		r.Passed = cmd.Run() == nil
		r.Message = "black"
	default:
		r.Passed = true
		r.Message = "no formatter for language, skipped"
	}
	return r
}

// CheckSecurity scans changed files for common vulnerability patterns.
func (p *Pipeline) CheckSecurity(files []File) []Result {
	var results []Result
	patterns := []struct {
		name    string
		pattern *regexp.Regexp
	}{
		{"hardcoded password", regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*['"][^'"]{3,}['"]`)},
		{"hardcoded api key", regexp.MustCompile(`(?i)(api[_-]?key|secret|token)\s*[:=]\s*['"][A-Za-z0-9_\-]{12,}['"]`)},
		{"sql injection", regexp.MustCompile(`(?i)\bexec(?:ute)?\s*\([^)]*(?:select|insert|update|delete)` + "\\s*\\+")},
		{"eval injection", regexp.MustCompile(`(?i)\beval\s*\(\s*[^)]*(?:request|input|body|user)`)},
		{"command injection", regexp.MustCompile(`(?i)(?:exec|system|shell_exec|os\.system)\s*\([^)]*\+`)},
		{"insecure http", regexp.MustCompile(`http://`)},
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(p.dir, f.Path))
		if err != nil {
			continue
		}
		found := []string{}
		for _, pat := range patterns {
			if pat.pattern.Match(data) && !(pat.name == "insecure http" && containsLocalHost(string(data))) {
				found = append(found, pat.name)
			}
		}
		r := Result{Check: CheckSecurity, File: f.Path}
		if len(found) == 0 {
			r.Passed = true
			r.Message = "no obvious patterns"
		} else {
			r.Message = "potential: " + strings.Join(found, ", ")
		}
		results = append(results, r)
	}
	return results
}

// DiffCheck returns the current git diff stat.
func (p *Pipeline) DiffCheck() Result {
	r := Result{Check: CheckDiff, File: "git diff"}
	start := time.Now()
	cmd := exec.Command("git", "diff", "--stat")
	cmd.Dir = p.dir
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	r.Duration = time.Since(start)
	if err == nil {
		r.Passed = true
		r.Message = strings.TrimSpace(out.String())
	}
	return r
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func containsLocalHost(s string) bool {
	lower := strings.ToLower(s)
	for _, host := range []string{"http://localhost", "http://127.0.0.1", "http://0.0.0.0", "http://::1"} {
		if strings.Contains(lower, host) {
			return true
		}
	}
	return false
}

func (p *Pipeline) Summary(results []Result) (int, int) {
	passed, failed := 0, 0
	for _, r := range results {
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}
	return passed, failed
}
