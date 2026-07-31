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

// Permission describes a capability a tool needs.
type Permission string

const (
	PermReadFiles   Permission = "read_files"
	PermWriteFiles  Permission = "write_files"
	PermExecProcess Permission = "execute_processes"
	PermNetwork     Permission = "network_access"
	PermBrowser     Permission = "browser_automation"
	PermGitOps      Permission = "git_operations"
	PermDatabase    Permission = "database_access"
	PermEnvVars     Permission = "environment_variables"
)

// Schema is a lightweight JSON-schema-like description.
type Schema struct {
	Type        string         `json:"type"`
	Properties  map[string]any `json:"properties,omitempty"`
	Required    []string       `json:"required,omitempty"`
	Description string         `json:"description,omitempty"`
}

// Manifest is the declarative definition of a tool.
type Manifest struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Version        string       `json:"version"`
	Author         string       `json:"author,omitempty"`
	Description    string       `json:"description"`
	Category       string       `json:"category"`
	Permissions    []Permission `json:"permissions"`
	Platform       []string     `json:"platform"`
	TimeoutSec     int          `json:"timeout"`
	Retry          int          `json:"retry"`
	Streaming      bool         `json:"supports_streaming"`
	Parallel       bool         `json:"supports_parallel"`
	Dependencies   []string     `json:"dependencies,omitempty"`
	RequiredTools  []string     `json:"required_tools,omitempty"`
	InputSchema    *Schema      `json:"input_schema,omitempty"`
	OutputSchema   *Schema      `json:"output_schema,omitempty"`
	Examples       []Example    `json:"examples,omitempty"`
	HealthCheck    string       `json:"health_check,omitempty"`
	BenchmarkScore float64      `json:"benchmark_score"`
	TrustLevel     string       `json:"trust_level,omitempty"`
	Source         string       `json:"source,omitempty"` // builtin | user | marketplace
	LastUpdated    time.Time    `json:"last_updated"`
}

type Example struct {
	Title  string   `json:"title"`
	Input  string   `json:"input"`
	Output string   `json:"output,omitempty"`
	Args   []string `json:"args,omitempty"`
}

// Stats tracks execution history for ranking and learning.
type Stats struct {
	Calls         int       `json:"calls"`
	Successes     int       `json:"successes"`
	Failures      int       `json:"failures"`
	TotalDuration float64   `json:"total_duration_ms"`
	AvgRuntimeMs  float64   `json:"avg_runtime_ms"`
	LastUsed      time.Time `json:"last_used"`
	LastUpdated   time.Time `json:"last_updated"`
	CommonParams  []string  `json:"common_params,omitempty"`
	Patterns      []string  `json:"patterns,omitempty"`
	RecoverySteps []string  `json:"recovery_steps,omitempty"`
}

func (s *Stats) SuccessRate() float64 {
	if s.Calls == 0 {
		return 1.0
	}
	return float64(s.Successes) / float64(s.Calls)
}

func (s *Stats) Reliability() float64 {
	if s.Calls < 3 {
		return 0.5
	}
	return s.SuccessRate()
}

func (s *Stats) Record(durationMs float64, ok bool, params []string) {
	s.Calls++
	s.TotalDuration += durationMs
	if ok {
		s.Successes++
	} else {
		s.Failures++
	}
	s.AvgRuntimeMs = s.TotalDuration / float64(s.Calls)
	s.LastUsed = time.Now()
	s.LastUpdated = time.Now()
	for _, p := range params {
		if p == "" {
			continue
		}
		found := false
		for _, c := range s.CommonParams {
			if c == p {
				found = true
				break
			}
		}
		if !found && len(s.CommonParams) < 8 {
			s.CommonParams = append(s.CommonParams, p)
		}
	}
}

// ToolFunc is the execution handler for a tool.
type ToolFunc func(args ...string) (string, error)

// Tool is a registered, executable capability.
type Tool struct {
	Manifest Manifest `json:"manifest"`
	Stats    Stats    `json:"stats"`
	Health   string   `json:"health"` // healthy | degraded | unavailable
	Run      ToolFunc `json:"-"`
}

func (t *Tool) Name() string { return t.Manifest.Name }

func (t *Tool) ID() string {
	if t.Manifest.ID != "" {
		return t.Manifest.ID
	}
	return t.Manifest.Name
}

func (t *Tool) Healthy() bool { return t.Health != "unavailable" }

func (t *Tool) NeedsApproval(policy PolicyMode) bool {
	if policy == PolicyAllowAll {
		return false
	}
	if policy == PolicyDenyAll {
		return true
	}
	for _, perm := range t.Manifest.Permissions {
		switch perm {
		case PermExecProcess, PermNetwork, PermBrowser, PermDatabase, PermWriteFiles, PermGitOps, PermEnvVars:
			return true
		}
	}
	return false
}

// PolicyMode controls how tool approvals are handled.
type PolicyMode string

const (
	PolicyAsk      PolicyMode = "ask"
	PolicyAllowAll PolicyMode = "allow_all"
	PolicyDenyAll  PolicyMode = "deny_all"
	PolicyRemember PolicyMode = "remember"
)

type ToolDir struct {
	Path  string
	Mtime time.Time
}

// loadManifests scans directories for tool.json manifests and returns tools
// whose handler functions can be resolved from the provided factory.
func loadManifests(dirs []string, factory func(Manifest) (ToolFunc, bool)) ([]*Tool, []error) {
	var tools []*Tool
	var errs []error
	seen := map[string]bool{}

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			manifestPath := filepath.Join(dir, e.Name(), "tool.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				continue
			}
			var m Manifest
			if err := json.Unmarshal(data, &m); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", manifestPath, err))
				continue
			}
			if m.ID == "" {
				m.ID = e.Name()
			}
			if m.Name == "" {
				m.Name = e.Name()
			}
			if seen[m.Name] {
				continue
			}
			seen[m.Name] = true
			if m.Source == "" {
				m.Source = "user"
			}
			if m.LastUpdated.IsZero() {
				m.LastUpdated = time.Now()
			}
			run, ok := factory(m)
			if !ok {
				errs = append(errs, fmt.Errorf("tool %s: no handler for language %q", m.Name, m.Dependencies))
				continue
			}
			tools = append(tools, &Tool{
				Manifest: m,
				Health:   "healthy",
				Run:      run,
			})
		}
	}

	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name() < tools[j].Name()
	})
	return tools, errs
}

func userToolDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	root := filepath.Join(home, ".delta", "tools")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(root, e.Name()))
		}
	}
	return dirs
}

func matchPlatform(platforms []string) bool {
	if len(platforms) == 0 {
		return true
	}
	osName := "windows"
	for _, p := range platforms {
		if p == "all" || p == osName {
			return true
		}
	}
	return false
}

func keywordsFor(t *Tool) string {
	var parts []string
	parts = append(parts, t.Manifest.Name, t.Manifest.ID, t.Manifest.Description, t.Manifest.Category)
	parts = append(parts, t.Manifest.RequiredTools...)
	parts = append(parts, t.Manifest.Dependencies...)
	for _, e := range t.Manifest.Examples {
		parts = append(parts, e.Title)
	}
	return strings.ToLower(strings.Join(parts, " "))
}
