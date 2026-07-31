package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type codeBlock struct {
	language string
	code     string
	filename string
}

func extractCodeBlocks(content string) []codeBlock {
	var blocks []codeBlock
	lines := strings.Split(content, "\n")
	var inBlock bool
	var lang string
	var code []string

	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			if inBlock {
				filename := extractFilename(lang, code)
				blocks = append(blocks, codeBlock{
					language: lang,
					code:     strings.Join(code, "\n"),
					filename: filename,
				})
				code = nil
				inBlock = false
			} else {
				inBlock = true
				lang = strings.TrimSpace(strings.TrimPrefix(line, "```"))
			}
			continue
		}
		if inBlock {
			code = append(code, line)
		}
	}
	return blocks
}

func extractFilename(lang string, code []string) string {
	lower := strings.ToLower(lang)
	known := map[string]string{
		"go":         ".go",
		"python":     ".py",
		"javascript": ".js",
		"typescript": ".ts",
		"rust":       ".rs",
		"c":          ".c",
		"cpp":        ".cpp",
		"h":          ".h",
		"java":       ".java",
		"ruby":       ".rb",
		"php":        ".php",
		"html":       ".html",
		"css":        ".css",
		"json":       ".json",
		"yaml":       ".yaml",
		"xml":        ".xml",
		"sql":        ".sql",
		"shell":      ".sh",
		"bash":       ".sh",
		"dockerfile": "Dockerfile",
	}

	if ext, ok := known[lower]; ok {
		for _, line := range code[:min(len(code), 5)] {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "package ") && ext == ".go" {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					return strings.ToLower(parts[1]) + ext
				}
			}
			if ext == "Dockerfile" {
				return "Dockerfile"
			}
		}
		return "main" + ext
	}

	if lower != "" {
		return "main." + lower
	}
	return ""
}

func copyCodeBlock(content string) (string, error) {
	blocks := extractCodeBlocks(content)
	if len(blocks) == 0 {
		return "", fmt.Errorf("no code blocks found")
	}
	block := blocks[len(blocks)-1]
	if err := clipboardWrite(block.code); err != nil {
		return "", err
	}
	return block.language, nil
}

func saveCodeBlock(content, dir string) (string, error) {
	blocks := extractCodeBlocks(content)
	if len(blocks) == 0 {
		return "", fmt.Errorf("no code blocks found")
	}
	block := blocks[len(blocks)-1]
	filename := block.filename
	if filename == "" {
		filename = "output.txt"
	}
	path := filepath.Join(dir, filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(block.code), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *model) copyBlock(idx int) tea.Cmd {
	blocks := m.lastBlocks()
	if len(blocks) == 0 {
		m.toastNow("No code blocks found")
		return nil
	}
	if idx >= len(blocks) {
		m.toastNow(fmt.Sprintf("Only %d code block(s) available", len(blocks)))
		return nil
	}
	if err := clipboardWrite(blocks[idx].code); err != nil {
		m.addSys("Copy failed: " + err.Error())
		return nil
	}
	lang := blocks[idx].language
	if lang == "" {
		lang = "code"
	}
	m.toastNow(fmt.Sprintf("Copied block %d (%s)", idx+1, lang))
	return nil
}

func (m *model) saveBlock(idx int) tea.Cmd {
	blocks := m.lastBlocks()
	if len(blocks) == 0 {
		m.toastNow("No code blocks found")
		return nil
	}
	if idx >= len(blocks) {
		m.toastNow(fmt.Sprintf("Only %d code block(s) available", len(blocks)))
		return nil
	}
	block := blocks[idx]
	filename := block.filename
	if filename == "" {
		filename = "output.txt"
	}
	path := filename
	if _, err := os.Stat(path); err == nil {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		path = fmt.Sprintf("%s-1%s", base, ext)
	}
	if err := os.WriteFile(path, []byte(block.code), 0644); err != nil {
		m.addSys("Save failed: " + err.Error())
		return nil
	}
	m.toastNow("Created " + path)
	return nil
}

func (m *model) lastBlocks() []codeBlock {
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].role == "assistant" && m.entries[i].content != "" {
			return extractCodeBlocks(m.entries[i].content)
		}
	}
	return nil
}
