package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ---------- Filesystem Tool ----------

func fsTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("fs: subcommand required (read|write|mkdir|delete|rename|move|copy|tree|meta|encoding|list)")
	}
	switch args[0] {
	case "read":
		if len(args) < 2 {
			return "", fmt.Errorf("fs read: path required")
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "write":
		if len(args) < 3 {
			return "", fmt.Errorf("fs write: path and content required")
		}
		if err := os.MkdirAll(filepath.Dir(args[1]), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(args[1], []byte(args[2]), 0644); err != nil {
			return "", err
		}
		return "wrote " + args[1], nil
	case "mkdir":
		if len(args) < 2 {
			return "", fmt.Errorf("fs mkdir: path required")
		}
		if err := os.MkdirAll(args[1], 0755); err != nil {
			return "", err
		}
		return "created " + args[1], nil
	case "delete":
		if len(args) < 2 {
			return "", fmt.Errorf("fs delete: path required")
		}
		if err := os.RemoveAll(args[1]); err != nil {
			return "", err
		}
		return "deleted " + args[1], nil
	case "rename":
		if len(args) < 3 {
			return "", fmt.Errorf("fs rename: old and new paths required")
		}
		if err := os.Rename(args[1], args[2]); err != nil {
			return "", err
		}
		return fmt.Sprintf("renamed %s -> %s", args[1], args[2]), nil
	case "move":
		if len(args) < 3 {
			return "", fmt.Errorf("fs move: old and new paths required")
		}
		if err := os.Rename(args[1], args[2]); err != nil {
			return "", err
		}
		return fmt.Sprintf("moved %s -> %s", args[1], args[2]), nil
	case "copy":
		if len(args) < 3 {
			return "", fmt.Errorf("fs copy: src and dst required")
		}
		if err := copyFileOrDir(args[1], args[2]); err != nil {
			return "", err
		}
		return fmt.Sprintf("copied %s -> %s", args[1], args[2]), nil
	case "list":
		return listDir(args[1:]...)
	case "tree":
		if len(args) < 2 {
			return "", fmt.Errorf("fs tree: path required")
		}
		return buildTree(args[1], 3), nil
	case "meta":
		if len(args) < 2 {
			return "", fmt.Errorf("fs meta: path required")
		}
		info, err := os.Stat(args[1])
		if err != nil {
			return "", err
		}
		m := map[string]any{
			"name":  info.Name(),
			"size":  info.Size(),
			"dir":   info.IsDir(),
			"mode":  info.Mode().String(),
			"mtime": info.ModTime().Format("2006-01-02 15:04:05"),
		}
		data, _ := json.MarshalIndent(m, "", "  ")
		return string(data), nil
	case "encoding":
		if len(args) < 2 {
			return "", fmt.Errorf("fs encoding: path required")
		}
		return detectEncoding(args[1]), nil
	default:
		return "", fmt.Errorf("fs: unknown subcommand %q", args[0])
	}
}

func listDir(args ...string) (string, error) {
	path := "."
	if len(args) > 0 && args[0] != "" {
		path = args[0]
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	var out []string
	for _, e := range entries {
		suffix := ""
		if e.IsDir() {
			suffix = "/"
		}
		out = append(out, e.Name()+suffix)
	}
	sort.Strings(out)
	return strings.Join(out, "\n"), nil
}

func buildTree(root string, depth int) string {
	var b strings.Builder
	var walk func(dir string, prefix string, level int)
	walk = func(dir, prefix string, level int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for i, e := range entries {
			isLast := i == len(entries)-1
			branch := "├── "
			childPrefix := prefix + "│   "
			if isLast {
				branch = "└── "
				childPrefix = prefix + "    "
			}
			b.WriteString(prefix + branch + e.Name())
			if e.IsDir() && level > 0 {
				b.WriteString("/")
				b.WriteString("\n")
				walk(filepath.Join(dir, e.Name()), childPrefix, level-1)
			} else {
				b.WriteString("\n")
			}
		}
	}
	b.WriteString(root + "/\n")
	walk(root, "", depth)
	return b.String()
}

func copyFileOrDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		return os.WriteFile(dst, data, info.Mode())
	}
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, fi.Mode())
	})
}

func detectEncoding(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "unknown"
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return "utf-8-bom"
	}
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return "utf-16-le"
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		return "utf-16-be"
	}
	// NUL bytes suggest binary.
	for _, b := range data[:min(len(data), 1024)] {
		if b == 0 {
			return "binary"
		}
	}
	return "utf-8"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------- Smart Edit Tool ----------

func editTool(args ...string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("edit: path, find, and replace required (optional: --regex)")
	}
	path, find, replace := args[0], args[1], args[2]
	isRegex := false
	for _, a := range args[3:] {
		if a == "--regex" {
			isRegex = true
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(data)
	var result string
	if isRegex {
		re, err := compileRegex(find)
		if err != nil {
			return "", fmt.Errorf("invalid regex %q: %w", find, err)
		}
		if !re.MatchString(content) {
			return "", fmt.Errorf("pattern %q not found in %s", find, path)
		}
		result = re.ReplaceAllString(content, replace)
	} else {
		if !strings.Contains(content, find) {
			return "", fmt.Errorf("text %q not found in %s", find, path)
		}
		result = strings.Replace(content, find, replace, 1)
	}
	if err := os.WriteFile(path, []byte(result), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %s", path), nil
}

// ---------- Terminal Tool ----------

func terminalTool(args ...string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("terminal: command required")
	}
	cmd := args[0]
	timeout := 60
	for i, a := range args {
		if a == "--timeout" && i+1 < len(args) {
			fmt.Sscanf(args[i+1], "%d", &timeout)
		}
	}
	if _, err := os.Executable(); err != nil {
		_ = err
	}
	return runShellCommand(cmd, timeout)
}

func runShellCommand(cmdLine string, timeoutSec int) (string, error) {
	return runExternal("powershell", []string{"-NoProfile", "-NonInteractive", "-Command", cmdLine}, timeoutSec)
}
