package autonomous

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/config"
	ctxeng "github.com/DevAnimecx/deltacode/internal/context"
	"github.com/DevAnimecx/deltacode/internal/intelligence"
	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/internal/repository"
	"github.com/DevAnimecx/deltacode/internal/tools"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

type Task struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on"`
	Tool        string   `json:"tool"`
	Model       string   `json:"model,omitempty"`
	File        string   `json:"file,omitempty"`
	Status      string   `json:"status"`
	Result      string   `json:"result,omitempty"`
	Error       string   `json:"error,omitempty"`
	Attempts    int      `json:"attempts"`
	OutputFile  string   `json:"output_file,omitempty"`
}

type Plan struct {
	Goal        string `json:"goal"`
	Tasks       []Task `json:"tasks"`
	Model       string `json:"model"`
	Provider    string `json:"provider"`
	CreatedAt   time.Time `json:"created_at"`
}

type Engine struct {
	cfg        *config.Manager
	provider   models.ProviderConfig
	model      string
	ctxEng     *ctxeng.Engine
	mem        *intelligence.Memory
	skills     *intelligence.SkillEngine
	toolReg    *tools.Registry
	repo       *repository.Analyzer
	maxRetries int
	sessionDir string
}

func NewEngine(cfg *config.Manager) *Engine {
	conf := cfg.GetConfig()
	provCfg, _ := cfg.GetProvider(conf.DefaultProvider)

	e := &Engine{
		cfg:        cfg,
		model:      conf.DefaultModel,
		maxRetries: 3,
	}
	if provCfg != nil {
		e.provider = *provCfg
	}
	e.ctxEng, _ = ctxeng.NewEngine()
	e.mem = intelligence.NewMemory()
	e.skills = intelligence.NewSkillEngine()
	e.toolReg = tools.NewRegistry()
	e.repo = repository.NewAnalyzer(".")
	e.sessionDir, _ = os.MkdirTemp("", "delta-session-*")
	return e
}

func (e *Engine) Execute(goal string) error {
	fmt.Printf("\n  Δ Autonomous Engine\n")
	fmt.Printf("  %s\n", strings.Repeat("━", 50))
	fmt.Printf("  Goal: %s\n", goal)
	fmt.Printf("  Provider: %s | Model: %s\n", e.provider.Name, e.model)
	fmt.Printf("  %s\n\n", strings.Repeat("━", 50))

	p, err := provider.NewProvider(e.provider)
	if err != nil {
		return fmt.Errorf("provider error: %w", err)
	}

	repoInfo := e.repo.Analyze()
	if repoInfo.ProjectType != "" {
		fmt.Printf("  📁 %s project detected (%d files)\n", repoInfo.ProjectType, repoInfo.FileCount)
		if len(repoInfo.EntryPoints) > 0 {
			fmt.Printf("  📍 Entry: %s\n", repoInfo.EntryPoints[0])
		}
	}

	matches := e.skills.Find(goal)
	if len(matches) > 0 && matches[0].UsageCount > 2 {
		fmt.Printf("  ⚡ Using cached skill: %s\n", matches[0].Name)
		fmt.Printf("  %s\n\n", matches[0].Output)
		return nil
	}

	fmt.Print("  Phase 1/6: Planning")
	plan, err := e.buildPlan(p, goal, repoInfo)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}
	fmt.Printf(" — %d tasks\n", len(plan.Tasks))

	for i, task := range plan.Tasks {
		fmt.Printf("  Phase 2-4/6: Task %d/%d: %s\n", i+1, len(plan.Tasks), task.Description)

		var result string
		var execErr error
		for attempt := 0; attempt <= e.maxRetries; attempt++ {
			if attempt > 0 {
				fmt.Printf("    Retry %d/%d\n", attempt, e.maxRetries)
			}
			result, execErr = e.executeTask(p, &task, plan)
			if execErr == nil {
				task.Status = "done"
				task.Result = result
				task.Attempts = attempt
				break
			}
			if attempt < e.maxRetries {
				result, execErr = e.fixExecution(p, goal, task.Description, result, execErr)
			}
		}

		if execErr != nil {
			fmt.Printf("    ✗ Failed after %d attempts: %s\n", task.Attempts+1, execErr)
			task.Status = "failed"
			task.Error = execErr.Error()
		} else {
			task.Status = "done"
			task.Result = result
			fmt.Printf("    ✓ Completed\n")
		}
		plan.Tasks[i] = task
	}

	fmt.Print("  Phase 5/6: Validation")
	valid := e.validate(plan)
	if valid {
		fmt.Println(" — ✓ All checks passed")
	} else {
		fmt.Println(" — ⚠ Some checks incomplete")
	}

	fmt.Print("  Phase 6/6: Learning")
	e.skills.Learn(goal, plan)
	e.mem.Store("workflow", "global", goal, fmt.Sprintf("plan:%s", plan.Model))
	fmt.Println(" — ✓ Saved")

	e.cleanup()
	fmt.Printf("\n  %s\n", strings.Repeat("━", 50))
	fmt.Printf("  ✓ Autonomous session complete\n")
	return nil
}

func (e *Engine) buildPlan(p provider.Provider, goal string, repo *repository.ProjectInfo) (*Plan, error) {
	sysPrompt := fmt.Sprintf(`You are a software architect. Analyze the goal and create an execution plan.

Project context:
- Type: %s
- Languages: %v
- Files: %d
- Entry points: %v

Return a JSON plan with this structure:
{"tasks":[{"id":"1","description":"what to do","depends_on":[],"tool":"write","file":"path/to/file"}]}
Keep tasks small, ordered, and dependency-aware. Use tools: read, write, edit, delete, exec, search.`, repo.ProjectType, repo.Languages, repo.FileCount, repo.EntryPoints)

	resp, err := p.Chat(models.ChatRequest{
		Model: e.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: sysPrompt},
			{Role: models.RoleUser, Content: goal},
		},
		Temperature: 0.2,
		MaxTokens:   4096,
	})
	if err != nil {
		return &Plan{Goal: goal, Model: e.model, Provider: e.provider.Name, CreatedAt: time.Now()}, nil
	}

	plan := &Plan{
		Goal:      goal,
		Model:     e.model,
		Provider:  e.provider.Name,
		CreatedAt: time.Now(),
	}
	if !strings.Contains(resp.Message.Content, "tasks") {
		plan.Tasks = []Task{{
			ID: "1", Description: goal,
			DependsOn: []string{}, Tool: "write", Status: "pending",
		}}
		return plan, nil
	}

	if err := json.Unmarshal([]byte(resp.Message.Content), plan); err != nil {
		plan.Tasks = []Task{{
			ID: "1", Description: goal,
			DependsOn: []string{}, Tool: "write", Status: "pending",
		}}
	}
	for i := range plan.Tasks {
		if plan.Tasks[i].ID == "" {
			plan.Tasks[i].ID = fmt.Sprintf("%d", i+1)
		}
		if plan.Tasks[i].Status == "" {
			plan.Tasks[i].Status = "pending"
		}
	}
	return plan, nil
}

func (e *Engine) executeTask(p provider.Provider, task *Task, plan *Plan) (string, error) {
	switch task.Tool {
	case "read":
		return e.toolReg.Call("read", task.File)
	case "write":
		resp, err := p.Chat(models.ChatRequest{
			Model: e.model,
			Messages: []models.Message{
				{Role: models.RoleSystem, Content: "You are an expert engineer. Write production-ready code. Return ONLY the code in a single code block."},
				{Role: models.RoleUser, Content: fmt.Sprintf("Goal: %s\n\nTask: %s\n\nWrite the complete implementation.", plan.Goal, task.Description)},
			},
			Temperature: 0.3,
			MaxTokens:   16384,
		})
		if err != nil {
			return "", err
		}
		code := resp.Message.Content
		if task.File != "" {
			dir := filepath.Dir(task.File)
			if dir != "." {
				os.MkdirAll(dir, 0755)
			}
			extracted := extractCode(code)
			os.WriteFile(task.File, []byte(extracted), 0644)
		}
		return code, nil

	case "edit":
		resp, err := p.Chat(models.ChatRequest{
			Model: e.model,
			Messages: []models.Message{
				{Role: models.RoleSystem, Content: "Return the complete updated file content in a code block."},
				{Role: models.RoleUser, Content: task.Description},
			},
			Temperature: 0.3,
		})
		if err != nil {
			return "", err
		}
		if task.File != "" {
			code := extractCode(resp.Message.Content)
			os.WriteFile(task.File, []byte(code), 0644)
		}
		return resp.Message.Content, nil

	case "exec":
		return e.toolReg.Call("exec", task.Description)

	case "search":
		return e.toolReg.Call("search", task.Description)

	case "delete":
		if task.File != "" {
			if err := os.Remove(task.File); err != nil {
				return "", err
			}
			return "Deleted " + task.File, nil
		}
		return "", fmt.Errorf("no file specified")

	default:
		resp, err := p.Chat(models.ChatRequest{
			Model: e.model,
			Messages: []models.Message{
				{Role: models.RoleSystem, Content: "You are an expert engineer."},
				{Role: models.RoleUser, Content: fmt.Sprintf("Goal: %s\n\nTask: %s\n\nExecute and return the result.", plan.Goal, task.Description)},
			},
			Temperature: 0.3,
		})
		if err != nil {
			return "", err
		}
		return resp.Message.Content, nil
	}
}

func (e *Engine) fixExecution(p provider.Provider, goal, desc, prevResult string, execErr error) (string, error) {
	resp, err := p.Chat(models.ChatRequest{
		Model: e.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: "You are a debugger. Fix the issue and return the corrected code."},
			{Role: models.RoleUser, Content: fmt.Sprintf("Goal: %s\nTask: %s\n\nError: %s\n\nPrevious result:\n%s\n\nReturn the fixed version.", goal, desc, execErr, prevResult)},
		},
		Temperature: 0.3,
		MaxTokens:   16384,
	})
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

func (e *Engine) validate(plan *Plan) bool {
	for _, task := range plan.Tasks {
		if task.Status != "done" {
			return false
		}
		if task.File != "" {
			if _, err := os.Stat(task.File); os.IsNotExist(err) {
				return false
			}
		}
	}
	return true
}

func (e *Engine) cleanup() {
	os.RemoveAll(e.sessionDir)
}

func (e *Engine) RunCode(code string) (string, int) {
	extracted := extractCode(code)
	tmpFile := filepath.Join(e.sessionDir, "script.py")
	os.WriteFile(tmpFile, []byte(extracted), 0644)
	cmd := exec.Command("python", tmpFile)
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return string(output), exitCode
}

func extractCode(text string) string {
	if strings.Contains(text, "```") {
		parts := strings.Split(text, "```")
		for i, part := range parts {
			if i%2 == 1 {
				code := strings.TrimSpace(part)
				if idx := strings.Index(code, "\n"); idx != -1 {
					code = code[idx+1:]
				}
				if code != "" {
					return strings.TrimSpace(code)
				}
			}
		}
	}
	return strings.TrimSpace(text)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}

