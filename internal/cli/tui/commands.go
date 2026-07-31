package tui

import (
	"os"
	"path/filepath"
	"strings"
)

type customCommand struct {
	Name        string
	Template    string
	Description string
	Agent       string
	Model       string
}

func (m *model) loadCustomCommands() []customCommand {
	var cmds []customCommand
	dirs := []string{
		".opencode/commands",
		".delta/commands",
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				continue
			}
			cmds = append(cmds, parseCommand(name, string(b)))
		}
	}
	return cmds
}

func parseCommand(name, content string) customCommand {
	cmd := customCommand{Name: name}
	lines := strings.Split(content, "\n")
	var inFront bool
	var body []string
	for _, line := range lines {
		if strings.HasPrefix(line, "---") {
			inFront = !inFront
			continue
		}
		if inFront {
			if strings.HasPrefix(line, "description:") {
				cmd.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			}
			if strings.HasPrefix(line, "agent:") {
				cmd.Agent = strings.TrimSpace(strings.TrimPrefix(line, "agent:"))
			}
			if strings.HasPrefix(line, "model:") {
				cmd.Model = strings.TrimSpace(strings.TrimPrefix(line, "model:"))
			}
			continue
		}
		body = append(body, line)
	}
	cmd.Template = strings.TrimSpace(strings.Join(body, "\n"))
	return cmd
}

func (m *model) resolveCommandTemplate(template string) string {
	template = strings.ReplaceAll(template, "$ARGUMENTS", "")
	return template
}
