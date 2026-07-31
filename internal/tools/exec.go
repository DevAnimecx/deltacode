package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func execCommand(args ...string) (string, error) {
	if len(args) < 1 || args[0] == "" {
		return "", fmt.Errorf("exec: command required")
	}

	command := strings.Join(args, " ")
	var stdout, stderr bytes.Buffer

	cmd := exec.Command("cmd", "/c", command)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		result := stdout.String()
		if stderr.String() != "" {
			if result != "" {
				result += "\n"
			}
			result += "STDERR: " + stderr.String()
		}
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result += fmt.Sprintf("\nExit code: %d", exitErr.ExitCode())
			}
			return result, nil
		}
		return strings.TrimSpace(result), nil

	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		return "", fmt.Errorf("exec: command timed out after 60s")
	}
}
