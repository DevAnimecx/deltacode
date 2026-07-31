package tools

import (
	"fmt"
	"strings"
	"sync"
)

type ToolFunc func(args ...string) (string, error)

type ToolDef struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Args        []string `json:"args"`
	Platforms   []string `json:"platforms"`
	Timeout     int      `json:"timeout"`
	Run         ToolFunc `json:"-"`
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]*ToolDef
}

func NewRegistry() *Registry {
	r := &Registry{tools: make(map[string]*ToolDef)}
	r.registerBuiltins()
	return r
}

func (r *Registry) Register(t *ToolDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name] = t
}

func (r *Registry) Get(name string) (*ToolDef, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	return t, nil
}

func (r *Registry) Call(name string, args ...string) (string, error) {
	t, err := r.Get(name)
	if err != nil {
		return "", err
	}
	return t.Run(args...)
}

func (r *Registry) List() []*ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*ToolDef
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

func (r *Registry) Search(query string) []*ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	query = strings.ToLower(query)
	var result []*ToolDef
	for _, t := range r.tools {
		if strings.Contains(strings.ToLower(t.Name), query) ||
			strings.Contains(strings.ToLower(t.Description), query) {
			result = append(result, t)
		}
	}
	return result
}

func (r *Registry) registerBuiltins() {
	r.Register(&ToolDef{
		Name: "read", Description: "Read file contents",
		Args: []string{"path"}, Platforms: []string{"all"}, Timeout: 10, Run: readFile,
	})
	r.Register(&ToolDef{
		Name: "write", Description: "Write content to file",
		Args: []string{"path", "content"}, Platforms: []string{"all"}, Timeout: 10, Run: writeFile,
	})
	r.Register(&ToolDef{
		Name: "edit", Description: "Edit file content",
		Args: []string{"path", "old", "new"}, Platforms: []string{"all"}, Timeout: 10, Run: editFile,
	})
	r.Register(&ToolDef{
		Name: "delete", Description: "Delete file or directory",
		Args: []string{"path"}, Platforms: []string{"all"}, Timeout: 10, Run: deleteFile,
	})
	r.Register(&ToolDef{
		Name: "exec", Description: "Execute shell command",
		Args: []string{"command"}, Platforms: []string{"all"}, Timeout: 60, Run: execCommand,
	})
	r.Register(&ToolDef{
		Name: "search", Description: "Search files by pattern",
		Args: []string{"pattern", "path"}, Platforms: []string{"all"}, Timeout: 30, Run: searchFiles,
	})
	r.Register(&ToolDef{
		Name: "git_status", Description: "Show git status",
		Args: []string{}, Platforms: []string{"all"}, Timeout: 10, Run: gitStatus,
	})
	r.Register(&ToolDef{
		Name: "git_diff", Description: "Show git diff",
		Args: []string{}, Platforms: []string{"all"}, Timeout: 10, Run: gitDiff,
	})
	r.Register(&ToolDef{
		Name: "git_commit", Description: "Create git commit",
		Args: []string{"message"}, Platforms: []string{"all"}, Timeout: 10, Run: gitCommit,
	})
	r.Register(&ToolDef{
		Name: "list", Description: "List directory contents",
		Args: []string{"path"}, Platforms: []string{"all"}, Timeout: 10, Run: listDir,
	})
}
