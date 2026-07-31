package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func (m *model) cycleModel(reverse bool) {
	conf := m.cfg.GetConfig()
	provCfg, _ := m.cfg.GetProvider(m.provName)
	var models []string
	if provCfg != nil {
		models = provCfg.Models
	}
	if len(models) == 0 {
		models = []string{conf.DefaultModel}
	}
	sort.Strings(models)
	cur := m.modelName
	idx := -1
	for i, mod := range models {
		if mod == cur {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = 0
	} else if reverse {
		idx = (idx - 1 + len(models)) % len(models)
	} else {
		idx = (idx + 1) % len(models)
	}
	m.modelName = models[idx]
	m.addSys(fmt.Sprintf("Model → %s", m.modelName))
}

func (m *model) toggleModelFavorite() {
	home, err := os.UserHomeDir()
	if err != nil {
		m.addSys("Cannot access home directory")
		return
	}
	favPath := filepath.Join(home, ".delta", "model-favorites.json")
	favs := map[string]bool{}
	if data, err := os.ReadFile(favPath); err == nil {
		json.Unmarshal(data, &favs)
	}
	if favs[m.modelName] {
		delete(favs, m.modelName)
		m.addSys(fmt.Sprintf("Removed %s from favorites", m.modelName))
	} else {
		favs[m.modelName] = true
		m.addSys(fmt.Sprintf("Added %s to favorites", m.modelName))
	}
	b, _ := json.MarshalIndent(favs, "", "  ")
	os.WriteFile(favPath, b, 0644)
}
