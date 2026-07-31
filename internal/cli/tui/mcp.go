package tui

import "fmt"

type mcpServer struct {
	Name   string
	Status string
	Tools  []string
}

func (m *model) listMCPServers() []mcpServer {
	return []mcpServer{
		{Name: "filesystem", Status: "enabled", Tools: []string{"read_file", "write_file", "list_directory"}},
		{Name: "git", Status: "enabled", Tools: []string{"git_status", "git_diff", "git_log"}},
	}
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
