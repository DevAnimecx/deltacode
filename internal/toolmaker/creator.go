package toolmaker

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Source      string `json:"source"`
	EntryPoint  string `json:"entry_point"`
	InstallCmd  string `json:"install_cmd"`
}

type Creator struct {
	toolsDir string
	provider models.ProviderConfig
	model    string
}

func NewCreator(prov models.ProviderConfig, model string) (*Creator, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".delta", "tools")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Creator{
		toolsDir: dir,
		provider: prov,
		model:    model,
	}, nil
}

func (c *Creator) Create(task string) (*Tool, error) {
	fmt.Println("Δ Auto Tool Creator")
	fmt.Println(strings.Repeat("─", 50))

	// Step 1: Determine language & approach
	fmt.Println("1/5: Analyzing task...")
	lang, approach := c.analyze(task)
	fmt.Printf("  Language: %s\n", lang)
	fmt.Printf("  Approach: %s\n", approach)

	// Step 2: Search for existing solutions
	fmt.Println("2/5: Searching for existing solutions...")
	existing := c.search(lang, task)
	if existing != "" {
		fmt.Printf("  Found: %s\n", truncate(existing, 100))
	}

	// Step 3: Generate tool
	fmt.Println("3/5: Generating tool...")
	tool, err := c.generate(task, lang, existing)
	if err != nil {
		return nil, err
	}

	// Step 4: Test tool
	fmt.Println("4/5: Testing tool...")
	if err := c.test(tool); err != nil {
		fmt.Printf("  ⚠ Test failed: %v\n", err)
		fmt.Println("  Attempting fix...")
		if fixed := c.fix(tool, err.Error()); fixed != "" {
			tool.Source = fixed
			c.test(tool)
		}
	}

	// Step 5: Install & register
	fmt.Println("5/5: Installing & registering...")
	if err := c.install(tool); err != nil {
		return nil, fmt.Errorf("install failed: %w", err)
	}

	fmt.Println("✓ Tool created:", tool.Name)
	return tool, nil
}

func (c *Creator) analyze(task string) (string, string) {
	p, err := provider.NewProvider(c.provider)
	if err != nil {
		return "python", "script"
	}

	resp, err := p.Chat(models.ChatRequest{
		Model: c.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: `Analyze this task and return JSON with:
- language: best language (python, node, go, bash)
- approach: script, cli, library, api
- package: recommended package from npm/pip/go if needed
- name: short tool name
Return ONLY valid JSON.`},
			{Role: models.RoleUser, Content: task},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return "python", "script"
	}

	var analysis struct {
		Language string `json:"language"`
		Approach string `json:"approach"`
		Package  string `json:"package"`
		Name     string `json:"name"`
	}
	content := cleanJSON(resp.Message.Content)
	if err := json.Unmarshal([]byte(content), &analysis); err != nil {
		return "python", "script"
	}
	if analysis.Language == "" {
		analysis.Language = "python"
	}
	if analysis.Approach == "" {
		analysis.Approach = "script"
	}
	return analysis.Language, analysis.Approach
}

func (c *Creator) search(lang, task string) string {
	var cmd *exec.Cmd
	switch lang {
	case "node", "javascript", "typescript":
		cmd = exec.Command("npm", "search", task, "--json")
	case "python":
		cmd = exec.Command("pip", "search", task)
	default:
		return ""
	}
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func (c *Creator) generate(task, lang, existing string) (*Tool, error) {
	p, err := provider.NewProvider(c.provider)
	if err != nil {
		return nil, err
	}

	var systemPrompt string
	switch lang {
	case "python":
		systemPrompt = `Write a Python script that solves the given task. 
- Use only stdlib or the specific package mentioned
- Include a main() function and if __name__ block
- Add error handling
- Return ONLY the code`
	case "node", "javascript":
		systemPrompt = `Write a Node.js script that solves the given task.
- Use only stdlib or the specific npm package mentioned
- Include proper error handling
- Return ONLY the code`
	case "go":
		systemPrompt = `Write a Go program that solves the given task.
- Single file main package
- Include proper error handling
- Return ONLY the code`
	default:
		systemPrompt = `Write a script that solves the given task. Return ONLY the code.`
	}

	resp, err := p.Chat(models.ChatRequest{
		Model: c.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: systemPrompt},
			{Role: models.RoleUser, Content: fmt.Sprintf("Task: %s\n\nExisting solutions context:\n%s", task, existing)},
		},
		Temperature: 0.3,
		MaxTokens:   8192,
	})
	if err != nil {
		return nil, err
	}

	source := extractCode(resp.Message.Content)

	// Determine name from task
	name := sanitizeName(task)
	entryPoint := name
	switch lang {
	case "python":
		entryPoint = name + ".py"
	case "node", "javascript":
		entryPoint = name + ".js"
	case "go":
		entryPoint = "main.go"
	}

	return &Tool{
		Name:        name,
		Description: task,
		Language:    lang,
		Source:      source,
		EntryPoint:  entryPoint,
	}, nil
}

func (c *Creator) test(tool *Tool) error {
	dir := filepath.Join(c.toolsDir, tool.Name)
	os.MkdirAll(dir, 0755)

	srcPath := filepath.Join(dir, tool.EntryPoint)
	os.WriteFile(srcPath, []byte(tool.Source), 0644)

	switch tool.Language {
	case "python":
		cmd := exec.Command("python", srcPath)
		cmd.Dir = dir
		return cmd.Run()
	case "node", "javascript":
		cmd := exec.Command("node", srcPath)
		cmd.Dir = dir
		return cmd.Run()
	}
	return nil
}

func (c *Creator) fix(tool *Tool, errorMsg string) string {
	p, err := provider.NewProvider(c.provider)
	if err != nil {
		return ""
	}

	resp, err := p.Chat(models.ChatRequest{
		Model: c.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: "Fix this code. Return ONLY the corrected code."},
			{Role: models.RoleUser, Content: fmt.Sprintf("Code:\n%s\n\nError:\n%s\n\nFix it.", tool.Source, errorMsg)},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return ""
	}
	return extractCode(resp.Message.Content)
}

func (c *Creator) install(tool *Tool) error {
	dir := filepath.Join(c.toolsDir, tool.Name)
	srcPath := filepath.Join(dir, tool.EntryPoint)
	os.WriteFile(srcPath, []byte(tool.Source), 0644)

	manifest := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Language    string `json:"language"`
		EntryPoint  string `json:"entry_point"`
	}{
		Name:        tool.Name,
		Description: tool.Description,
		Language:    tool.Language,
		EntryPoint:  tool.EntryPoint,
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(dir, "tool.json"), data, 0644)

	fmt.Printf("  Installed: %s\n", dir)
	return nil
}

func (c *Creator) ListTools() ([]Tool, error) {
	entries, err := os.ReadDir(c.toolsDir)
	if err != nil {
		return nil, err
	}

	var tools []Tool
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(c.toolsDir, entry.Name(), "tool.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}
		var tool Tool
		if err := json.Unmarshal(data, &tool); err != nil {
			continue
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func sanitizeName(task string) string {
	words := strings.Fields(task)
	var clean []string
	for _, w := range words {
		w = strings.TrimFunc(w, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9')
		})
		if w != "" {
			clean = append(clean, w)
		}
		if len(clean) >= 3 {
			break
		}
	}
	name := strings.Join(clean, "-")
	if name == "" {
		return "delta-tool"
	}
	return strings.ToLower(name)
}

func extractCode(text string) string {
	if strings.Contains(text, "```") {
		parts := strings.Split(text, "```")
		for i, part := range parts {
			if i%2 == 0 {
				continue
			}
			code := strings.TrimSpace(part)
			if idx := strings.Index(code, "\n"); idx != -1 {
				code = code[idx+1:]
			}
			return code
		}
	}
	return text
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
