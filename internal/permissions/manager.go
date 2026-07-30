package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Policy struct {
	AllowCommands []string `json:"allow_commands"`
	DenyCommands  []string `json:"deny_commands"`
	AllowPaths    []string `json:"allow_paths"`
	DenyPaths     []string `json:"deny_paths"`
	MaxFileSize   int64    `json:"max_file_size"`
	AllowNetwork  bool     `json:"allow_network"`
}

type Manager struct {
	policy Policy
	path   string
	vault  map[string]string
}

func NewManager() (*Manager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".delta")
	os.MkdirAll(dir, 0700)

	m := &Manager{
		path:  filepath.Join(dir, "policy.json"),
		vault: make(map[string]string),
	}

	m.load()
	return m, nil
}

func (m *Manager) load() {
	data, err := os.ReadFile(m.path)
	if err != nil {
		m.policy = Policy{
			AllowCommands: []string{"*"},
			AllowPaths:    []string{"."},
			AllowNetwork:  true,
			MaxFileSize:   10485760,
		}
		m.Save()
		return
	}
	json.Unmarshal(data, &m.policy)
}

func (m *Manager) Save() error {
	data, err := json.MarshalIndent(m.policy, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0600)
}

func (m *Manager) CheckCommand(cmd string) error {
	if len(m.policy.DenyCommands) > 0 {
		for _, denied := range m.policy.DenyCommands {
			if denied == "*" || strings.Contains(cmd, denied) {
				return fmt.Errorf("command %q denied by policy", cmd)
			}
		}
	}
	if len(m.policy.AllowCommands) > 0 && m.policy.AllowCommands[0] != "*" {
		allowed := false
		for _, a := range m.policy.AllowCommands {
			if strings.Contains(cmd, a) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command %q not in allowed list", cmd)
		}
	}
	return nil
}

func (m *Manager) CheckPath(path string) error {
	abs, _ := filepath.Abs(path)
	for _, denied := range m.policy.DenyPaths {
		if strings.Contains(abs, denied) {
			return fmt.Errorf("path %q denied by policy", path)
		}
	}
	return nil
}

func (m *Manager) SetPolicy(p Policy) {
	m.policy = p
	m.Save()
}

func (m *Manager) GetPolicy() Policy {
	return m.policy
}

func (m *Manager) VaultSet(key, value string) {
	m.vault[key] = value
}

func (m *Manager) VaultGet(key string) (string, bool) {
	v, ok := m.vault[key]
	return v, ok
}

func (m *Manager) VaultDelete(key string) {
	delete(m.vault, key)
}
