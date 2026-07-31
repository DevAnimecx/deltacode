package tui

import (
	"encoding/json"
	"fmt"
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
	dir := sessionsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		m.addSys("No shared sessions found")
		return
	}
	removed := 0
	for _, e := range entries {
		if !e.IsDir() && (e.Name() == "share-"+m.sessionTitle+".json" || e.Name() == "share-"+time.Now().Format("20060102")+"*.json") {
			os.Remove(filepath.Join(dir, e.Name()))
			removed++
		}
	}
	if removed == 0 {
		shareFiles := []string{}
		for _, e := range entries {
			if !e.IsDir() && len(e.Name()) > 6 && e.Name()[:6] == "share-" {
				shareFiles = append(shareFiles, e.Name())
			}
		}
		if len(shareFiles) > 0 {
			os.Remove(filepath.Join(dir, shareFiles[0]))
			m.addSys("Removed shared file: " + shareFiles[0])
		} else {
			m.addSys("No shared files to remove")
		}
	} else {
		m.addSys(fmt.Sprintf("Removed %d shared file(s)", removed))
	}
}
