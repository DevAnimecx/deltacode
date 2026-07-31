package tui

import (
	"os/exec"
	"strings"
)

func (m *model) resolveFileRefs(prompt string) string {
	var out []string
	words := strings.Fields(prompt)
	for _, w := range words {
		if !strings.HasPrefix(w, "@") {
			out = append(out, w)
			continue
		}
		query := strings.TrimPrefix(w, "@")
		files := m.fuzzyFiles(query)
		if len(files) == 0 {
			out = append(out, w)
			continue
		}
		content := m.readFiles(files)
		out = append(out, "[context:"+strings.Join(files, ",")+"]")
		out = append(out, content)
	}
	return strings.Join(out, " ")
}

func (m *model) fuzzyFiles(query string) []string {
	q := strings.ToLower(query)
	cmd := exec.Command("git", "ls-files")
	if m.wsData != nil && m.wsData.project != "" {
		cmd.Dir = m.wsData.project
	}
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var matches []string
	for _, f := range strings.Split(string(out), "\n") {
		if f == "" {
			continue
		}
		lower := strings.ToLower(f)
		if strings.Contains(lower, q) {
			matches = append(matches, f)
		}
	}
	if len(matches) > 5 {
		return matches[:5]
	}
	return matches
}

func (m *model) readFiles(files []string) string {
	var parts []string
	for _, f := range files {
		cmd := exec.Command("git", "show", "HEAD:"+f)
		if m.wsData != nil && m.wsData.project != "" {
			cmd.Dir = m.wsData.project
		}
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		parts = append(parts, "=== "+f+" ===\n"+string(out))
	}
	if len(parts) == 0 {
		return ""
	}
	return "[file_context]\n" + strings.Join(parts, "\n\n") + "\n[end_file_context]"
}
