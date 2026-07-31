package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func gitStatus(args ...string) (string, error) {
	cmd := exec.Command("git", "status", "--short")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}
	result := strings.TrimSpace(string(out))
	if result == "" {
		return "Clean working tree", nil
	}
	return result, nil
}

func gitDiff(args ...string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	result := strings.TrimSpace(string(out))
	if result == "" {
		return "No unstaged changes", nil
	}
	cmd2 := exec.Command("git", "diff")
	out2, err2 := cmd2.Output()
	if err2 == nil && len(out2) > 0 {
		content := string(out2)
		if len(content) > 2000 {
			content = content[:2000] + "\n... (truncated)"
		}
		result += "\n\n" + content
	}
	return result, nil
}

func gitCommit(args ...string) (string, error) {
	msg := "Delta Auto Commit"
	if len(args) > 0 && args[0] != "" {
		msg = strings.Join(args, " ")
	}

	var stderr bytes.Buffer
	cmd := exec.Command("git", "add", "-A")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git add: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	stderr.Reset()
	cmd2 := exec.Command("git", "commit", "-m", msg)
	cmd2.Stderr = &stderr
	out, err := cmd2.Output()
	if err != nil {
		errStr := strings.TrimSpace(stderr.String())
		if strings.Contains(errStr, "nothing to commit") {
			return "Nothing to commit", nil
		}
		return "", fmt.Errorf("git commit: %s: %w", errStr, err)
	}
	return strings.TrimSpace(string(out)), nil
}
