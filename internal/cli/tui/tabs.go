package tui

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type sessionTab struct {
	ID      string
	Title   string
	Model   string
	Cost    float64
	Tokens  int
	Updated time.Time
	Active  bool
}

func (m *model) tabs() []sessionTab {
	var tabs []sessionTab
	seen := map[string]bool{}
	for _, s := range m.sessions {
		if seen[s.File] {
			continue
		}
		seen[s.File] = true
		tabs = append(tabs, sessionTab{
			ID:      s.File,
			Title:   s.Title,
			Model:   s.Model,
			Cost:    s.Cost,
			Tokens:  s.Messages,
			Updated: s.Updated,
			Active:  s.File == sessionPath(),
		})
	}
	sort.Slice(tabs, func(i, j int) bool {
		return tabs[i].Updated.After(tabs[j].Updated)
	})
	if len(tabs) > 12 {
		tabs = tabs[:12]
	}
	return tabs
}

func (m *model) tabBar() string {
	tabs := m.tabs()
	if len(tabs) <= 1 {
		return ""
	}
	var parts []string
	for _, t := range tabs {
		label := t.Title
		if label == "" {
			label = "session"
		}
		if len(label) > 18 {
			label = label[:18]
		}
		style := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("43")).
			Padding(0, 1).
			Foreground(lipgloss.Color("252"))
		if t.Active {
			style = style.
				Background(lipgloss.Color("235")).
				Bold(true)
		}
		parts = append(parts, style.Render(" "+label+" "))
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

func (m *model) switchTab(id string) {
	if id == sessionPath() {
		return
	}
	m.saveSession()
	m.entries = nil
	m.messages = nil
	m.cost = 0
	m.tok = 0
	m.exchanges = 0
	m.sessionTitle = ""
	m.loadSession()
	m.render()
}

func (m *model) closeTab(id string) {
	if id == sessionPath() {
		m.newSession(true)
		return
	}
	os.Remove(id)
	for i, s := range m.sessions {
		if s.File == id {
			m.sessions = append(m.sessions[:i], m.sessions[i+1:]...)
			break
		}
	}
	m.addSys("Tab closed")
	m.render()
}

func (m *model) nextTab() {
	tabs := m.tabs()
	for i, t := range tabs {
		if t.Active && i+1 < len(tabs) {
			m.switchTab(tabs[i+1].ID)
			return
		}
	}
}

func (m *model) prevTab() {
	tabs := m.tabs()
	for i, t := range tabs {
		if t.Active && i > 0 {
			m.switchTab(tabs[i-1].ID)
			return
		}
	}
}

func (m *model) showTimeline() {
	m.addSys("----- Session Timeline -----")
	tabs := m.tabs()
	if len(tabs) == 0 {
		m.addSys("No sessions yet.")
		return
	}
	for _, t := range tabs {
		marker := "  "
		if t.Active {
			marker = "▶ "
		}
		m.addSys(fmt.Sprintf("%s%s  $%.4f  %dtok  %s", marker, t.Title, t.Cost, t.Tokens, t.Updated.Format("Jan 02 15:04")))
	}
}
