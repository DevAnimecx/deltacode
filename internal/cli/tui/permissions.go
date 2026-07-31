package tui

import (
	"fmt"
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
	m.addSys(fmt.Sprintf("Permission required: %s — %s", action, details))
	m.confirmed = false
	m.confirmAction = string(action)
	return false
}

func (m *model) grantPermission(action permissionAction) {
	m.addSys(fmt.Sprintf("Permission granted: %s", action))
}

func (m *model) denyPermission(action permissionAction) {
	m.addSys(fmt.Sprintf("Permission denied: %s", action))
}
