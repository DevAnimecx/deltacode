package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func searchFiles(args ...string) (string, error) {
	pattern := ""
	path := "."
	if len(args) > 0 && args[0] != "" {
		pattern = args[0]
	}
	if len(args) > 1 && args[1] != "" {
		path = args[1]
	}
	if pattern == "" {
		return "", fmt.Errorf("search: pattern required")
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("findstr", "/s", "/n", "/i", pattern, path+"\\*.*")
	if path == "." {
		cmd = exec.Command("findstr", "/s", "/n", "/i", pattern, "*.*")
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run()

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return fmt.Sprintf("No matches for %q", pattern), nil
	}

	lines := strings.Split(result, "\n")
	if len(lines) > 50 {
		lines = lines[:50]
		result = strings.Join(lines, "\n") + fmt.Sprintf("\n... and %d more", len(strings.Split(stdout.String(), "\n"))-50)
	}
	return result, nil
}
