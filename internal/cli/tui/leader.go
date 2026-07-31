package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	case "g":
		m.showTimeline()
		return nil, true
	default:
		return nil, false
	}
}

func (m *model) openEditor() tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "notepad"
	}
	path := filepath.Join(sessionsDir(), "edit-"+time.Now().Format("20060102150405")+".md")
	var b strings.Builder
	for _, e := range m.entries {
		switch e.role {
		case "user":
			b.WriteString("## User\n" + e.content + "\n\n")
		case "assistant":
			b.WriteString("## Assistant\n" + e.content + "\n\n")
		}
	}
	os.WriteFile(path, []byte(b.String()), 0644)
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	data, err := os.ReadFile(path)
	if err == nil {
		content := strings.TrimSpace(string(data))
		if content != "" {
			m.addSys("Content loaded from editor")
			m.ta.SetValue(content)
			m.ta.Focus()
		}
	}
	os.Remove(path)
	return nil
}

func (m *model) redoLast() tea.Cmd {
	e, msg, ok := m.popUndo()
	if !ok {
		m.addSys("Nothing to redo")
		return nil
	}
	m.entries = append(m.entries, e)
	if msg.Role != "" {
		m.messages = append(m.messages, msg)
	}
	m.render()
	m.toastNow("Redo restored")
	return nil
}
