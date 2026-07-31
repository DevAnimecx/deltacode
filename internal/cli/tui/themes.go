package tui

import "github.com/charmbracelet/lipgloss"

var themes = []func() theme{
	defTheme,
	monoTheme,
	minimalTheme,
	solarTheme,
}

var themeNames = []string{"default", "mono", "minimal", "solar"}

func monoTheme() theme {
	t := defTheme()
	c := lipgloss.Color("255")
	s := lipgloss.Color("245")
	t.head = t.head.Foreground(c)
	t.uP = t.uP.Foreground(c)
	t.aP = t.aP.Foreground(c)
	t.fk = t.fk.Foreground(c)
	t.badge = t.badge.Foreground(c)
	t.codeL = t.codeL.Foreground(s)
	return t
}

func minimalTheme() theme {
	t := defTheme()
	t.uP = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	t.aP = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	t.sep = lipgloss.NewStyle()
	t.brd = lipgloss.NewStyle()
	t.meta = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Padding(0, 2)
	t.badge = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Padding(0, 1)
	t.scr = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	t.head = t.head.Foreground(lipgloss.Color("255"))
	return t
}

func solarTheme() theme {
	t := defTheme()
	c := lipgloss.Color("215")
	ac := lipgloss.Color("220")
	t.head = lipgloss.NewStyle().Bold(true).Foreground(c).Padding(0, 1)
	t.uP = lipgloss.NewStyle().Bold(true).Foreground(ac).SetString("┃ You ")
	t.aP = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("215")).SetString("Δ ")
	t.fk = lipgloss.NewStyle().Foreground(ac).Bold(true)
	t.badge = lipgloss.NewStyle().Background(lipgloss.Color("238")).Foreground(ac).Padding(0, 1)
	t.codeL = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(c).Padding(0, 1)
	t.scr = lipgloss.NewStyle().Background(lipgloss.Color("237")).Foreground(ac).Padding(0, 1)
	return t
}

func (m *model) cycleTheme() {
	m.themeIdx = (m.themeIdx + 1) % len(themes)
	m.t = themes[m.themeIdx]()
	m.addSys("Theme: " + themeNames[m.themeIdx])
	m.render()
}
