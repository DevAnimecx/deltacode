package tui

import (
	"encoding/json"
	"fmt"
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
	if cfg.Keybinds != nil {
		m.applyKeybinds(cfg.Keybinds)
	}
	if cfg.Attention.Enabled {
		os.Setenv("DELTA_ATTENTION", "1")
	}
}

func (m *model) applyKeybinds(binds map[string]string) {
	for action, key := range binds {
		switch action {
		case "undo":
			m.toastNow(fmt.Sprintf("Keybind: %s → undo", key))
		case "redo":
			m.toastNow(fmt.Sprintf("Keybind: %s → redo", key))
		case "compact":
			m.toastNow(fmt.Sprintf("Keybind: %s → compact", key))
		}
	}
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
