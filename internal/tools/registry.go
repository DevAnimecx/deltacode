package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Registry struct {
	mu        sync.RWMutex
	tools     map[string]*Tool
	aliases   map[string]string // name -> canonical name
	userDirs  []string
	statsDir  string
	policy    PolicyMode
	approvals *approvalsStore
}

func NewRegistry() *Registry {
	r := &Registry{
		tools:     make(map[string]*Tool),
		aliases:   make(map[string]string),
		policy:    PolicyAllowAll,
		approvals: newApprovalsStore(),
	}
	r.registerBuiltins()
	r.loadUserTools()
	r.loadStats()
	return r
}

// NewRegistryWithPolicy creates a registry with the given approval policy.
func NewRegistryWithPolicy(policy PolicyMode) *Registry {
	r := NewRegistry()
	r.policy = policy
	return r
}

func (r *Registry) SetPolicy(p PolicyMode) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policy = p
}

func (r *Registry) Policy() PolicyMode {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.policy
}

func (r *Registry) Register(t *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name()] = t
	for _, alias := range []string{t.Manifest.ID} {
		r.aliases[alias] = t.Name()
	}
}

// RegisterFunc is a convenience for built-ins.
func (r *Registry) RegisterFunc(name string, fn ToolFunc) {
	r.Register(&Tool{
		Manifest: Manifest{
			ID: name, Name: name, Version: "1.0.0", Source: "builtin",
			Platform: []string{"all"}, TimeoutSec: 30, TrustLevel: "trusted",
		},
		Run: fn,
	})
}

func (r *Registry) Get(name string) (*Tool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if t, ok := r.tools[name]; ok {
		return t, nil
	}
	if canonical, ok := r.aliases[name]; ok {
		return r.tools[canonical], nil
	}
	return nil, fmt.Errorf("tool %q not found", name)
}

func (r *Registry) GetByName(name string) (*Tool, bool) {
	t, err := r.Get(name)
	if err != nil {
		return nil, false
	}
	return t, true
}

func (r *Registry) Call(name string, args ...string) (string, error) {
	return r.CallWith(name, CallOptions{}, args...)
}

// CallOptions carries execution-engine controls.
type CallOptions struct {
	TimeoutSec int
	Retries    int
	Parallel   bool
	Background bool
	Approver   func(t *Tool, args []string) bool
}

// CallWith runs a tool through the execution engine.
func (r *Registry) CallWith(name string, opts CallOptions, args ...string) (string, error) {
	t, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return r.execute(t, opts, args...)
}

func (r *Registry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name() < result[j].Name() })
	return result
}

// ListHealthy returns tools that are available on this platform and healthy.
func (r *Registry) ListHealthy() []*Tool {
	var result []*Tool
	for _, t := range r.List() {
		if t.Healthy() && matchPlatform(t.Manifest.Platform) {
			result = append(result, t)
		}
	}
	return result
}

func (r *Registry) Search(query string) []*Tool {
	q := strings.ToLower(strings.TrimSpace(query))
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*Tool
	for _, t := range r.tools {
		if !t.Healthy() {
			continue
		}
		if q == "" || strings.Contains(keywordsFor(t), q) {
			result = append(result, t)
		}
	}
	return result
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Categories returns all tool categories with counts.
func (r *Registry) Categories() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := map[string]int{}
	for _, t := range r.tools {
		cat := t.Manifest.Category
		if cat == "" {
			cat = "general"
		}
		out[cat]++
	}
	return out
}

// userToolsHome is the directory scanned for user-installed tools.
func userToolsHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(home, ".delta", "tools")
	os.MkdirAll(dir, 0755)
	return dir
}

func (r *Registry) loadUserTools() {
	home := userToolsHome()
	if home == "" {
		return
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(home, e.Name())
		manifestPath := filepath.Join(dir, "tool.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var m Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		if m.ID == "" {
			m.ID = e.Name()
		}
		if m.Name == "" {
			m.Name = e.Name()
		}
		if m.Source == "" {
			m.Source = "user"
		}
		run := userToolRunner(dir, &m)
		if run == nil {
			continue
		}
		r.mu.Lock()
		if _, exists := r.tools[m.Name]; !exists {
			r.tools[m.Name] = &Tool{
				Manifest: m,
				Stats:    Stats{},
				Health:   "healthy",
				Run:      run,
			}
			r.aliases[m.ID] = m.Name
		}
		r.mu.Unlock()
	}
}

// userToolRunner builds a runner for user-installed tools (scripts with entry_point).
func userToolRunner(dir string, m *Manifest) ToolFunc {
	entry := m.HealthCheck
	if entry == "" {
		return nil
	}
	full := filepath.Join(dir, entry)
	if _, err := os.Stat(full); err != nil {
		return nil
	}
	lang := ""
	if len(m.Dependencies) > 0 {
		lang = strings.ToLower(m.Dependencies[0])
	}
	switch lang {
	case "python", "py":
		return func(args ...string) (string, error) {
			return runExternal("python", append([]string{full}, args...), m.TimeoutSec)
		}
	case "node", "javascript", "js":
		return func(args ...string) (string, error) {
			return runExternal("node", append([]string{full}, args...), m.TimeoutSec)
		}
	case "go":
		return func(args ...string) (string, error) {
			return runExternal("go", append([]string{"run", full}, args...), m.TimeoutSec)
		}
	case "bash", "sh":
		return func(args ...string) (string, error) {
			return runExternal("bash", append([]string{full}, args...), m.TimeoutSec)
		}
	}
	// Fallback: try to execute directly (windows: .cmd/.bat/.exe).
	return func(args ...string) (string, error) {
		return runExternal(full, args, m.TimeoutSec)
	}
}

func (r *Registry) statsPath() string {
	home := userToolsHome()
	return filepath.Join(home, "stats.json")
}

func (r *Registry) loadStats() {
	path := r.statsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var stats map[string]Stats
	if err := json.Unmarshal(data, &stats); err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, st := range stats {
		if t, ok := r.tools[name]; ok {
			t.Stats = st
		}
	}
}

func (r *Registry) saveStats() error {
	r.mu.RLock()
	stats := map[string]Stats{}
	for name, t := range r.tools {
		stats[name] = t.Stats
	}
	r.mu.RUnlock()
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.statsPath(), data, 0600)
}

// Reload rescans user tool directories (hot reload of newly installed tools).
func (r *Registry) Reload() int {
	before := r.Count()
	r.loadUserTools()
	return r.Count() - before
}

func (r *Registry) StatsFor(name string) (Stats, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	if !ok {
		return Stats{}, false
	}
	return t.Stats, true
}

func (r *Registry) TopBy(limit int, scoreFn func(*Tool) float64) []*Tool {
	list := r.List()
	sort.Slice(list, func(i, j int) bool {
		return scoreFn(list[i]) > scoreFn(list[j])
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list
}
