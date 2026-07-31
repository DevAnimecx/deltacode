package tui

import (
	"fmt"
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
	m.addSys("Model favorites not implemented yet.")
}
