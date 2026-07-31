package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type tuiConfig struct {
	Theme         string            `json:"theme"`
	LeaderKey     string            `json:"leader"`
	LeaderTimeout int               `json:"leader_timeout"`
	ScrollSpeed   float64           `json:"scroll_speed"`
	Keybinds      map[string]string `json:"keybinds"`
	Attention     attentionConfig   `json:"attention"`
	Mouse         bool              `json:"mouse"`
	DiffStyle     string            `json:"diff_style"`
}

func defaultTUIConfig() tuiConfig {
	return tuiConfig{
		Theme:         "",
		LeaderKey:     "ctrl+x",
		LeaderTimeout: 1500,
		ScrollSpeed:   3,
		Mouse:         true,
		DiffStyle:     "auto",
		Attention:     defaultAttention(),
	}
}

func (m *model) loadTUIConfig() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	paths := []string{
		filepath.Join(home, ".delta", "tui.json"),
		filepath.Join(home, ".delta", "tui.jsonc"),
		filepath.Join(".delta", "tui.json"),
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg tuiConfig
		if json.Unmarshal(b, &cfg) == nil {
			m.applyTUIConfig(cfg)
			return
		}
	}
}

func (m *model) applyTUIConfig(cfg tuiConfig) {
	if cfg.Theme != "" {
		m.themeIdx = m.themeIndex(cfg.Theme)
	}
	if cfg.LeaderKey != "" {
		// leaderKey is const; dynamic leader not implemented yet
	}
	if cfg.Keybinds != nil {
		m.applyKeybinds(cfg.Keybinds)
	}
}

func (m *model) applyKeybinds(binds map[string]string) {
	_ = binds
}

func (m *model) themeIndex(name string) int {
	themes := []string{"default", "opencode", "monokai", "dracula", "nord"}
	for i, t := range themes {
		if strings.EqualFold(t, name) {
			return i
		}
	}
	return 0
}
