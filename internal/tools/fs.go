package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readFile(args ...string) (string, error) {
	if len(args) < 1 || args[0] == "" {
		return "", fmt.Errorf("read: path required")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return "", fmt.Errorf("read %s: %w", args[0], err)
	}
	return string(data), nil
}

func writeFile(args ...string) (string, error) {
	if len(args) < 2 || args[0] == "" {
		return "", fmt.Errorf("write: path and content required")
	}
	path := args[0]
	content := strings.Join(args[1:], " ")
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("write: cannot create dir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return fmt.Sprintf("Written %d bytes to %s", len(content), path), nil
}

func editFile(args ...string) (string, error) {
	if len(args) < 3 || args[0] == "" {
		return "", fmt.Errorf("edit: path, old, new required")
	}
	path := args[0]
	old := args[1]
	newStr := strings.Join(args[2:], " ")

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("edit: cannot read %s: %w", path, err)
	}
	content := string(data)
	if !strings.Contains(content, old) {
		return "", fmt.Errorf("edit: pattern not found in %s", path)
	}
	content = strings.Replace(content, old, newStr, 1)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("edit: cannot write %s: %w", path, err)
	}
	return fmt.Sprintf("Edited %s (1 replacement)", path), nil
}

func deleteFile(args ...string) (string, error) {
	if len(args) < 1 || args[0] == "" {
		return "", fmt.Errorf("delete: path required")
	}
	if err := os.RemoveAll(args[0]); err != nil {
		return "", fmt.Errorf("delete %s: %w", args[0], err)
	}
	return fmt.Sprintf("Deleted %s", args[0]), nil
}

func listDir(args ...string) (string, error) {
	path := "."
	if len(args) > 0 && args[0] != "" {
		path = args[0]
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("list %s: %w", path, err)
	}
	var result []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		info, _ := e.Info()
		if info != nil {
			result = append(result, fmt.Sprintf("%s (%d bytes)", name, info.Size()))
		} else {
			result = append(result, name)
		}
	}
	return strings.Join(result, "\n"), nil
}
