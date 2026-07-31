package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func slashItems() []ddItem {
	return []ddItem{
		{label: "/clear", desc: "Clear conversation", value: "/clear"},
		{label: "/copy", desc: "Copy entire transcript", value: "/copy"},
		{label: "/cost", desc: "Show session cost & tokens", value: "/cost"},
		{label: "/export", desc: "Save transcript to file", value: "/export"},
		{label: "/export-json", desc: "Export transcript as JSON", value: "/export-json"},
		{label: "/agent", desc: "Toggle agent task mode", value: "/agent"},
		{label: "/agent-task", desc: "Set agent task type", value: "/agent-task"},
		{label: "/auto-fix", desc: "Run autonomous fix loop", value: "/auto-fix"},
		{label: "/help", desc: "Show help", value: "/help"},
		{label: "/keys", desc: "Keyboard shortcuts overlay", value: "/keys"},
		{label: "/minimal", desc: "Toggle minimal mode", value: "/minimal"},
		{label: "/model", desc: "Switch model", value: "/model"},
		{label: "/new", desc: "Start a new session", value: "/new"},
		{label: "/provider", desc: "Switch provider", value: "/provider"},
		{label: "/run", desc: "Run last code block in sandbox", value: "/run"},
		{label: "/search", desc: "Search conversation", value: "/search"},
		{label: "/sessions", desc: "List & resume sessions", value: "/sessions"},
		{label: "/stats", desc: "Session statistics", value: "/stats"},
		{label: "/summary", desc: "Session summary report", value: "/summary"},
		{label: "/theme", desc: "Cycle color themes", value: "/theme"},
		{label: "/think", desc: "Toggle reasoning display", value: "/think"},
		{label: "/tips", desc: "Show usage tips", value: "/tips"},
		{label: "/undo", desc: "Remove last exchange", value: "/undo"},
		{label: "/workspace", desc: "Open engineering workspace", value: "/workspace"},
		{label: "/wrap", desc: "Toggle word wrap", value: "/wrap"},
	}
}

func (m *model) openDropdown(kind dropdownKind) {
	m.dd.kind = kind
	m.dd.query = ""
	m.dd.cursor = 0
	m.ddInput = ""

	switch kind {
	case ddCommand:
		m.dd.title = "Commands"
		m.dd.items = slashItems()
		m.applyFilter()
	case ddModel:
		m.dd.title = "Model"
		conf := m.cfg.GetConfig()
		provCfg, err := m.cfg.GetProvider(m.provName)
		var models []string
		if err == nil && provCfg != nil {
			models = provCfg.Models
		}
		if len(models) == 0 {
			models = []string{conf.DefaultModel}
		}
		seen := map[string]bool{}
		var items []ddItem
		for _, mod := range models {
			if mod == "" || seen[mod] {
				continue
			}
			seen[mod] = true
			items = append(items, ddItem{
				label:    mod,
				value:    mod,
				desc:     "select model",
				selected: mod == m.modelName,
			})
		}
		if len(items) == 0 {
			items = append(items, ddItem{label: conf.DefaultModel, value: conf.DefaultModel})
		}
		sort.Slice(items, func(i, j int) bool {
			if items[i].selected {
				return true
			}
			if items[j].selected {
				return false
			}
			return items[i].label < items[j].label
		})
		m.dd.items = items
		m.applyFilter()
	case ddProvider:
		m.dd.title = "Provider"
		conf := m.cfg.GetConfig()
		var items []ddItem
		for _, p := range conf.Providers {
			items = append(items, ddItem{
				label:    p.Name,
				value:    p.Name,
				desc:     fmt.Sprintf("%s %s", p.BaseURL, m.healthLatency(p.Name)),
				selected: p.Name == m.provName,
			})
		}
		if len(items) == 0 {
			items = append(items, ddItem{label: m.provName, value: m.provName})
		}
		m.dd.items = items
		m.applyFilter()
	}
	m.dd.setOpen(true)
}

func (m *model) applyFilter() {
	q := strings.ToLower(m.ddInput)
	if q == "" {
		m.dd.filtered = make([]ddItem, len(m.dd.items))
		copy(m.dd.filtered, m.dd.items)
	} else {
		var filtered []ddItem
		for _, it := range m.dd.items {
			if fuzzyContains(it.label, q) || fuzzyContains(it.desc, q) {
				filtered = append(filtered, it)
			}
		}
		m.dd.filtered = filtered
	}
	if m.dd.cursor >= len(m.dd.filtered) {
		m.dd.cursor = 0
	}
}

// fuzzyContains reports whether the query chars appear in order in s.
func fuzzyContains(s, q string) bool {
	s = strings.ToLower(s)
	i := 0
	for _, r := range q {
		idx := strings.IndexRune(s[i:], r)
		if idx < 0 {
			return false
		}
		i += idx + 1
	}
	return true
}

func (m *model) onDropdownKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.dd.setOpen(false)
		m.ta.Focus()
		return nil
	case "enter":
		return m.applyDropdownSelection()
	case "up", "shift+tab":
		if m.dd.cursor > 0 {
			m.dd.cursor--
		}
		return nil
	case "down", "tab":
		if m.dd.cursor < len(m.dd.filtered)-1 {
			m.dd.cursor++
		}
		return nil
	case "pgup":
		m.dd.cursor -= 5
		if m.dd.cursor < 0 {
			m.dd.cursor = 0
		}
		return nil
	case "pgdown":
		m.dd.cursor += 5
		if m.dd.cursor >= len(m.dd.filtered) {
			m.dd.cursor = len(m.dd.filtered) - 1
		}
		return nil
	case "home":
		m.dd.cursor = 0
		return nil
	case "end":
		m.dd.cursor = len(m.dd.filtered) - 1
		return nil
	case "ctrl+c":
		m.dd.setOpen(false)
		return nil
	case "backspace":
		if len(m.ddInput) > 0 {
			m.ddInput = m.ddInput[:len(m.ddInput)-1]
			m.applyFilter()
		}
		return nil
	case "ctrl+u":
		m.ddInput = ""
		m.applyFilter()
		return nil
	default:
		// Treat printable input as a filter query.
		if len(msg.String()) == 1 && msg.String()[0] >= 32 {
			m.ddInput += msg.String()
			m.applyFilter()
			return nil
		}
	}
	return nil
}

func (m *model) applyDropdownSelection() tea.Cmd {
	items := m.dd.filtered
	if len(items) == 0 {
		m.dd.setOpen(false)
		return nil
	}
	sel := items[m.dd.cursor]
	m.dd.setOpen(false)
	m.ta.SetValue("")

	switch m.dd.kind {
	case ddCommand:
		return m.slash(sel.value)
	case ddSlash:
		return m.slash(sel.value)
	case ddModel:
		m.modelName = sel.value
		m.addSys("Model → " + sel.value)
	case ddProvider:
		m.provName = sel.value
		m.addSys("Provider → " + sel.value)
	}
	m.ta.Focus()
	return nil
}

func (m *model) dropdownView() string {
	if !m.dd.visible() {
		return ""
	}

	sw, _ := m.ta.Width(), m.ta.Height()
	width := sw
	if width < 40 {
		width = 40
	}
	if width > 60 {
		width = 60
	}
	height := len(m.dd.filtered)
	if height > 8 {
		height = 8
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("43")).
		Padding(0, 1).Width(width)

	var lines []string
	title := m.dd.title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43"))
	if m.ddInput != "" {
		title += "  " + m.ddInput + "▌"
	}
	lines = append(lines, titleStyle.Render(title))
	lines = append(lines, "")

	start := 0
	if m.dd.cursor >= height {
		start = m.dd.cursor - height + 1
	}
	for i := start; i < start+height && i < len(m.dd.filtered); i++ {
		it := m.dd.filtered[i]
		sel := i == m.dd.cursor
		prefix := "  "
		label := it.label
		if it.selected {
			label += " •"
		}
		if sel {
			prefix = "▶ "
			label = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(label)
		}
		desc := ""
		if it.desc != "" && !m.minimal {
			desc = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("  " + it.desc)
		}
		lines = append(lines, fmt.Sprintf("%s%s%s", prefix, label, desc))
	}
	if len(m.dd.filtered) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  no matches"))
	}

	return style.Render(strings.Join(lines, "\n"))
}
