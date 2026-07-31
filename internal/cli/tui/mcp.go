package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type mcpServer struct {
	Name   string
	Status string
	Tools  []string
}

type mcpConfig struct {
	Servers map[string]mcpServerEntry `json:"servers"`
}

type mcpServerEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	Tools   []string          `json:"tools,omitempty"`
}

func (m *model) listMCPServers() []mcpServer {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var servers []mcpServer
	paths := []string{
		filepath.Join(home, ".delta", "mcp.json"),
		filepath.Join(".delta", "mcp.json"),
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg mcpConfig
		if json.Unmarshal(data, &cfg) != nil {
			continue
		}
		for name, entry := range cfg.Servers {
			status := "enabled"
			if entry.Command == "" {
				status = "misconfigured"
			}
			tools := entry.Tools
			if len(tools) == 0 {
				tools = []string{"(auto-discovered on connect)"}
			}
			servers = append(servers, mcpServer{
				Name:   name,
				Status: status,
				Tools:  tools,
			})
		}
		break
	}
	if len(servers) == 0 {
		servers = append(servers, mcpServer{
			Name:   "(none configured)",
			Status: "no config",
			Tools:  []string{"Add servers to ~/.delta/mcp.json"},
		})
	}
	return servers
}

func (m *model) showMCP() {
	servers := m.listMCPServers()
	m.addSys("----- MCP Servers -----")
	for _, s := range servers {
		m.addSys(fmt.Sprintf("  %s  [%s]  %d tools", s.Name, s.Status, len(s.Tools)))
		for _, t := range s.Tools {
			m.addSys(fmt.Sprintf("    - %s", t))
		}
	}
}
