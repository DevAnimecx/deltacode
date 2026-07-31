package tui

import (
	"fmt"

	"github.com/DevAnimecx/deltacode/pkg/models"
)

func (m *model) compactSession() {
	if len(m.entries) < 6 {
		m.addSys("Nothing to compact.")
		return
	}
	var old []string
	for _, e := range m.entries {
		if e.role == "user" || e.role == "assistant" {
			old = append(old, e.content)
		}
	}
	if len(old) == 0 {
		m.addSys("Nothing to compact.")
		return
	}
	summary := "Summary of previous conversation:\n"
	for i, c := range old {
		summary += fmt.Sprintf("%d. %s\n", i+1, truncateStr(c, 120))
	}
	m.entries = append([]entry{{role: "system", content: summary}}, m.entries...)
	m.messages = m.messages[:0]
	for _, e := range m.entries {
		switch e.role {
		case "user":
			m.messages = append(m.messages, models.Message{Role: models.RoleUser, Content: e.content})
		case "assistant":
			m.messages = append(m.messages, models.Message{Role: models.RoleAssistant, Content: e.content})
		case "system":
			m.messages = append(m.messages, models.Message{Role: models.RoleSystem, Content: e.content})
		}
	}
	m.render()
	m.toastNow("Session compacted")
}
