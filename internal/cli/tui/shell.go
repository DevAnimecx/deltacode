package tui

import (
	"os/exec"
	"strings"
)

func (m *model) injectShell(prompt string) string {
	if !strings.HasPrefix(prompt, "!") {
		return prompt
	}
	cmdStr := strings.TrimPrefix(prompt, "!")
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return prompt
	}
	cmd := exec.Command("sh", "-c", cmdStr)
	if m.wsData != nil && m.wsData.project != "" {
		cmd.Dir = m.wsData.project
	}
	out, err := cmd.CombinedOutput()
	result := string(out)
	if err != nil {
		result = "[shell_error] " + err.Error() + "\n" + result
	}
	return "[tool_result]\n" + result + "\n[end_tool_result]"
}
