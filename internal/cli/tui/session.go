package tui

import (
	"os"
	"path/filepath"
	"time"
)

func (m *model) forkSession() {
	path := sessionPath()
	b, err := os.ReadFile(path)
	if err != nil {
		m.addSys("No session to fork.")
		return
	}
	newPath := filepath.Join(sessionsDir(), "fork-"+time.Now().Format("20060102-150405")+".json")
	os.WriteFile(newPath, b, 0644)
	m.addSys("Session forked: " + newPath)
}

func (m *model) renameSession(newName string) {
	if newName == "" {
		m.addSys("Usage: /rename <name>")
		return
	}
	m.sessionTitle = newName
	m.saveSession()
	m.toastNow("Renamed to: " + newName)
}

func (m *model) deleteSession() {
	path := sessionPath()
	os.Remove(path)
	m.newSession(true)
	m.addSys("Session deleted.")
}
