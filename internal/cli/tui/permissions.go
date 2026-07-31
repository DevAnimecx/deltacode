package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type permissionAction string

const (
	permShell     permissionAction = "shell"
	permSandbox   permissionAction = "sandbox"
	permFileWrite permissionAction = "file_write"
	permNetwork   permissionAction = "network"
)

type permissionPrompt struct {
	Action  permissionAction
	Details string
	Allowed bool
}

func (m *model) askPermission(action permissionAction, details string) bool {
	persistPath := m.permissionPath()
	granted := m.loadGrantedPermissions(persistPath)
	key := string(action) + ":" + details
	if granted[key] {
		return true
	}
	m.addSys(fmt.Sprintf("Permission required: %s — %s (press Y to allow, N to deny)", action, details))
	m.confirmed = false
	m.confirmAction = "perm:" + key
	return false
}

func (m *model) grantPermission(action permissionAction) {
	m.addSys(fmt.Sprintf("Permission granted: %s", action))
	persistPath := m.permissionPath()
	granted := m.loadGrantedPermissions(persistPath)
	key := string(action) + ":*"
	granted[key] = true
	b, _ := json.MarshalIndent(granted, "", "  ")
	os.WriteFile(persistPath, b, 0600)
}

func (m *model) denyPermission(action permissionAction) {
	m.addSys(fmt.Sprintf("Permission denied: %s", action))
}

func (m *model) permissionPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".delta", "permissions.json")
}

func (m *model) loadGrantedPermissions(path string) map[string]bool {
	granted := map[string]bool{}
	data, err := os.ReadFile(path)
	if err != nil {
		return granted
	}
	json.Unmarshal(data, &granted)
	return granted
}
