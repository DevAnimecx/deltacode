package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type paletteItem struct {
	label    string
	value    string
	desc     string
	category string
	selected bool
}

type palette struct {
	kind     paletteKind
	open     bool
	items    []paletteItem
	filtered []paletteItem
	cursor   int
	query    string
	title    string
}

type paletteKind int

const (
	palNone paletteKind = iota
	palCommand
	palModel
	palProvider
	palSlash
	palFile
	palSearch
)

func (p *palette) setOpen(open bool) { p.open = open }

func (p *palette) visible() bool { return p.open && len(p.filtered) > 0 }

func (m *model) openPalette(kind paletteKind) {
	m.pal.kind = kind
	m.pal.query = ""
	m.pal.cursor = 0
	m.palInput = ""

	switch kind {
	case palCommand:
		m.pal.title = "Command Palette"
		m.pal.items = m.paletteCommands()
		m.applyPaletteFilter()
	case palModel:
		m.pal.title = "Model"
		m.pal.items = m.paletteModels()
		m.applyPaletteFilter()
	case palProvider:
		m.pal.title = "Provider"
		m.pal.items = m.paletteProviders()
		m.applyPaletteFilter()
	case palSlash:
		m.pal.title = "Commands"
		m.pal.items = m.paletteSlashItems()
		m.applyPaletteFilter()
	case palFile:
		m.pal.title = "Files"
		m.pal.items = m.paletteFiles("")
		m.applyPaletteFilter()
	case palSearch:
		m.pal.title = "Search"
		m.pal.items = m.paletteSearch("")
		m.applyPaletteFilter()
	}
	m.pal.setOpen(true)
}

func (m *model) paletteCommands() []paletteItem {
	items := []paletteItem{
		{label: "/help", value: "/help", desc: "Show help", category: "commands"},
		{label: "/new", value: "/new", desc: "New session", category: "commands"},
		{label: "/clear", value: "/clear", desc: "Clear conversation", category: "commands"},
		{label: "/undo", value: "/undo", desc: "Undo last exchange", category: "commands"},
		{label: "/redo", value: "/redo", desc: "Redo last undone", category: "commands"},
		{label: "/export", value: "/export", desc: "Export transcript", category: "commands"},
		{label: "/sessions", value: "/sessions", desc: "List sessions", category: "commands"},
		{label: "/theme", value: "/theme", desc: "Cycle theme", category: "commands"},
		{label: "/model", value: "/model", desc: "Switch model", category: "commands"},
		{label: "/provider", value: "/provider", desc: "Switch provider", category: "commands"},
		{label: "/thinking", value: "/thinking", desc: "Toggle reasoning", category: "commands"},
		{label: "/compact", value: "/compact", desc: "Summarize session", category: "commands"},
		{label: "/init", value: "/init", desc: "Initialize AGENTS.md", category: "commands"},
		{label: "/agent", value: "/agent", desc: "Toggle agent mode", category: "agent"},
		{label: "/auto-fix", value: "/auto-fix", desc: "Autonomous fix loop", category: "agent"},
		{label: "/checkpoint", value: "/checkpoint", desc: "Save checkpoint", category: "tools"},
		{label: "/run", value: "/run", desc: "Run last code block", category: "tools"},
		{label: "/workspace", value: "/workspace", desc: "Workspace view", category: "tools"},
		{label: "/mcp", value: "/mcp", desc: "List MCP servers", category: "tools"},
		{label: "/share", value: "/share", desc: "Share session", category: "session"},
		{label: "/unshare", value: "/unshare", desc: "Unshare session", category: "session"},
		{label: "/fork", value: "/fork", desc: "Fork session", category: "session"},
		{label: "/rename", value: "/rename", desc: "Rename session", category: "session"},
		{label: "/delete", value: "/delete", desc: "Delete session", category: "session"},
	}
	for i := range items {
		items[i].selected = items[i].value == "/"+items[i].label[1:]
	}
	return items
}

func (m *model) paletteModels() []paletteItem {
	conf := m.cfg.GetConfig()
	provCfg, _ := m.cfg.GetProvider(m.provName)
	var models []string
	if provCfg != nil {
		models = provCfg.Models
	}
	if len(models) == 0 {
		models = []string{conf.DefaultModel}
	}
	seen := map[string]bool{}
	var items []paletteItem
	for _, mod := range models {
		if mod == "" || seen[mod] {
			continue
		}
		seen[mod] = true
		items = append(items, paletteItem{
			label:    mod,
			value:    mod,
			desc:     "select model",
			selected: mod == m.modelName,
		})
	}
	return items
}

func (m *model) paletteProviders() []paletteItem {
	conf := m.cfg.GetConfig()
	var items []paletteItem
	for _, p := range conf.Providers {
		items = append(items, paletteItem{
			label:    p.Name,
			value:    p.Name,
			desc:     p.BaseURL,
			selected: p.Name == m.provName,
		})
	}
	if len(items) == 0 {
		items = append(items, paletteItem{label: m.provName, value: m.provName})
	}
	return items
}

func (m *model) paletteSlashItems() []paletteItem {
	all := slashItems()
	items := make([]paletteItem, len(all))
	for i, it := range all {
		items[i] = paletteItem{
			label:    it.label,
			value:    it.value,
			desc:     it.desc,
			category: "slash",
		}
	}
	return items
}

func (m *model) paletteFiles(query string) []paletteItem {
	files := m.fuzzyFiles(query)
	items := make([]paletteItem, len(files))
	for i, f := range files {
		items[i] = paletteItem{
			label:    f,
			value:    "@" + f,
			desc:     "insert file context",
			category: "files",
		}
	}
	return items
}

func (m *model) paletteSearch(query string) []paletteItem {
	if query == "" {
		return nil
	}
	all := slashItems()
	var items []paletteItem
	for _, it := range all {
		if strings.Contains(strings.ToLower(it.label), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(it.desc), strings.ToLower(query)) {
			items = append(items, paletteItem{
				label:    it.label,
				value:    it.value,
				desc:     it.desc,
				category: "search",
			})
		}
	}
	return items
}

func (m *model) applyPaletteFilter() {
	q := strings.ToLower(m.pal.query)
	if q == "" {
		m.pal.filtered = make([]paletteItem, len(m.pal.items))
		copy(m.pal.filtered, m.pal.items)
	} else {
		var filtered []paletteItem
		for _, it := range m.pal.items {
			if strings.Contains(strings.ToLower(it.label), q) ||
				strings.Contains(strings.ToLower(it.desc), q) ||
				strings.Contains(strings.ToLower(it.category), q) {
				filtered = append(filtered, it)
			}
		}
		m.pal.filtered = filtered
	}
	if m.pal.cursor >= len(m.pal.filtered) {
		m.pal.cursor = 0
	}
}

func (m *model) onPaletteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.pal.setOpen(false)
		m.ta.Focus()
		return nil
	case "enter":
		return m.applyPaletteSelection()
	case "up", "shift+tab":
		if m.pal.cursor > 0 {
			m.pal.cursor--
		}
		return nil
	case "down", "tab":
		if m.pal.cursor < len(m.pal.filtered)-1 {
			m.pal.cursor++
		}
		return nil
	case "pgup":
		m.pal.cursor -= 5
		if m.pal.cursor < 0 {
			m.pal.cursor = 0
		}
		return nil
	case "pgdown":
		m.pal.cursor += 5
		if m.pal.cursor >= len(m.pal.filtered) {
			m.pal.cursor = len(m.pal.filtered) - 1
		}
		return nil
	case "home":
		m.pal.cursor = 0
		return nil
	case "end":
		m.pal.cursor = len(m.pal.filtered) - 1
		return nil
	case "ctrl+c":
		m.pal.setOpen(false)
		return nil
	case "backspace":
		if len(m.palInput) > 0 {
			m.palInput = m.palInput[:len(m.palInput)-1]
			m.applyPaletteFilter()
		}
		return nil
	case "ctrl+u":
		m.palInput = ""
		m.applyPaletteFilter()
		return nil
	default:
		if len(msg.String()) == 1 && msg.String()[0] >= 32 {
			m.palInput += msg.String()
			m.applyPaletteFilter()
			return nil
		}
	}
	return nil
}

func (m *model) applyPaletteSelection() tea.Cmd {
	items := m.pal.filtered
	if len(items) == 0 {
		m.pal.setOpen(false)
		return nil
	}
	sel := items[m.pal.cursor]
	m.pal.setOpen(false)
	m.ta.SetValue("")

	switch m.pal.kind {
	case palCommand, palSlash:
		return m.slash(sel.value)
	case palModel:
		m.modelName = sel.value
		m.addSys("Model → " + sel.value)
	case palProvider:
		m.provName = sel.value
		m.addSys("Provider → " + sel.value)
	case palFile:
		m.ta.SetValue(m.ta.Value() + " " + sel.value)
	case palSearch:
		m.ta.SetValue(sel.value)
	}
	m.ta.Focus()
	return nil
}

func (m *model) paletteView() string {
	if !m.pal.visible() {
		return ""
	}

	sw := m.w
	if sw < 40 {
		sw = 40
	}
	if sw > 70 {
		sw = 70
	}
	height := len(m.pal.filtered)
	if height > 10 {
		height = 10
	}
	if height < 3 {
		height = 3
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("43")).
		Padding(0, 1).
		Width(sw)

	var lines []string
	title := m.pal.title
	if m.palInput != "" {
		title += "  " + m.palInput + "▌"
	}
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(title))
	lines = append(lines, "")

	start := 0
	if m.pal.cursor >= height {
		start = m.pal.cursor - height + 1
	}
	for i := start; i < start+height && i < len(m.pal.filtered); i++ {
		it := m.pal.filtered[i]
		sel := i == m.pal.cursor
		prefix := "  "
		label := it.label
		if sel {
			prefix = "▶ "
			label = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("43")).Render(label)
		}
		desc := ""
		if it.desc != "" && !m.minimal {
			desc = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("  " + it.desc)
		}
		cat := ""
		if it.category != "" && !m.minimal {
			cat = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" [" + it.category + "]")
		}
		lines = append(lines, fmt.Sprintf("%s%s%s%s", prefix, label, cat, desc))
	}
	if len(m.pal.filtered) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  no matches"))
	}

	return style.Render(strings.Join(lines, "\n"))
}
