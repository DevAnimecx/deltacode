package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	maxHistory  = 200
	timeRFC3339 = time.RFC3339
)

func historyPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".delta", "history.json")
}

// saveHistory persists the prompt history (fuzzy-completable).
func (m *model) saveHistory() {
	if len(m.inputHistory) == 0 {
		return
	}
	if n := len(m.inputHistory); n > maxHistory {
		m.inputHistory = m.inputHistory[n-maxHistory:]
	}
	data, err := json.MarshalIndent(m.inputHistory, "", "  ")
	if err != nil {
		return
	}
	if p := historyPath(); p != "" {
		os.MkdirAll(filepath.Dir(p), 0700)
		os.WriteFile(p, data, 0600)
	}
}

// loadHistory restores previously saved prompts.
func (m *model) loadHistory() {
	p := historyPath()
	if p == "" {
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var hist []string
	if json.Unmarshal(data, &hist) == nil {
		m.inputHistory = hist
	}
}

// exportJSON writes the session transcript as JSON.
func (m *model) exportJSON(path string) error {
	type msg struct {
		Role      string  `json:"role"`
		Content   string  `json:"content"`
		Reasoning string  `json:"reasoning,omitempty"`
		Model     string  `json:"model,omitempty"`
		Tokens    int     `json:"tokens,omitempty"`
		Cost      float64 `json:"cost,omitempty"`
		Time      string  `json:"time,omitempty"`
	}
	var out []msg
	for _, e := range m.entries {
		if e.role == "system" || e.role == "error" {
			continue
		}
		m := msg{Role: e.role, Content: e.content, Reasoning: e.reasoning,
			Model: e.model, Tokens: e.tokens, Cost: e.cost}
		if !e.ts.IsZero() {
			m.Time = e.ts.Format(timeRFC3339)
		}
		out = append(out, m)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
