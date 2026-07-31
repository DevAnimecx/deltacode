package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func (m *model) shareSession() {
	data := map[string]any{
		"title":     m.sessionTitle,
		"model":     m.modelName,
		"provider":  m.provName,
		"messages":  m.messages,
		"cost":      m.cost,
		"tokens":    m.tok,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	b, _ := json.MarshalIndent(data, "", "  ")
	path := filepath.Join(sessionsDir(), "share-"+time.Now().Format("20060102-150405")+".json")
	os.WriteFile(path, b, 0644)
	m.toastNow("Session shared: " + path)
}

func (m *model) unshareSession() {
	m.addSys("Unshare: remove shared session links.")
}
