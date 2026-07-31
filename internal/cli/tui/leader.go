package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const leaderKey = "ctrl+x"
const leaderTimeout = 1500 * time.Millisecond

type leaderState int

const (
	leaderIdle leaderState = iota
	leaderWaiting
)

type leaderMsg struct{}

func (m *model) initLeader() {
	m.leaderState = leaderIdle
}

func (m *model) handleLeader(msg string) tea.Cmd {
	if msg != leaderKey {
		return nil
	}
	m.leaderState = leaderWaiting
	return tea.Tick(leaderTimeout, func(t time.Time) tea.Msg {
		return leaderMsg{}
	})
}

func (m *model) consumeLeader(msg tea.Msg) (tea.Cmd, bool) {
	if m.leaderState != leaderWaiting {
		return nil, false
	}
	if _, ok := msg.(leaderMsg); ok {
		m.leaderState = leaderIdle
		return nil, false
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}
	k := key.String()
	m.leaderState = leaderIdle
	switch k {
	case "n":
		return m.newSession(true), true
	case "u":
		return m.undoLast(), true
	case "r":
		return m.redoLast(), true
	case "c":
		return m.slash("/compact"), true
	case "l":
		m.openDropdown(ddCommand)
		return nil, true
	case "m":
		m.openDropdown(ddModel)
		return nil, true
	case "t":
		m.cycleTheme()
		return nil, true
	case "e":
		return m.openEditor(), true
	case "x":
		return m.exportTranscript(), true
	case "q":
		m.saveSession()
		m.saveHistory()
		m.addSys("Session saved. Goodbye.")
		return tea.Quit, true
	case "h":
		m.helpShown = !m.helpShown
		return nil, true
	case "p":
		m.openDropdown(ddCommand)
		return nil, true
	case "s":
		m.showStats()
		return nil, true
	default:
		return nil, false
	}
}

func (m *model) openEditor() tea.Cmd {
	return nil
}

func (m *model) redoLast() tea.Cmd {
	m.addSys("Redo is not implemented yet.")
	return nil
}
