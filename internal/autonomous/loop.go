package autonomous

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/context"
	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

type Loop struct {
	provider    models.ProviderConfig
	model       string
	context     *context.Engine
	maxIter     int
	sandboxDir  string
}

func NewLoop(prov models.ProviderConfig, model string, maxIter int) *Loop {
	ctxEng, _ := context.NewEngine()
	sandbox, _ := os.MkdirTemp("", "delta-autonomous-*")
	return &Loop{
		provider:   prov,
		model:      model,
		context:    ctxEng,
		maxIter:    maxIter,
		sandboxDir: sandbox,
	}
}

func (l *Loop) Execute(task string) error {
	fmt.Printf("Δ Autonomous Loop — %s\n", task)
	fmt.Println(strings.Repeat("─", 50))

	p, err := provider.NewProvider(l.provider)
	if err != nil {
		return err
	}

	// Phase 1: Plan
	fmt.Println("Phase 1/5: Planning...")
	plan, err := l.plan(p, task)
	if err != nil {
		return err
	}
	fmt.Printf("  Plan: %s\n", truncate(plan, 200))

	// Phase 2: Write
	fmt.Println("Phase 2/5: Writing code...")
	code, err := l.write(p, task, plan)
	if err != nil {
		return err
	}
	fmt.Printf("  Generated %d bytes\n", len(code))

	// Phase 3: Run
	fmt.Println("Phase 3/5: Running...")
	runResult := l.runCode(code)
	fmt.Printf("  Exit: %d\n", runResult.exitCode)
	if runResult.stdout != "" {
		fmt.Printf("  Output: %s\n", truncate(runResult.stdout, 300))
	}
	if runResult.stderr != "" {
		fmt.Printf("  Errors: %s\n", truncate(runResult.stderr, 300))
	}

	// Phase 4: Fix loop
	iteration := 0
	for runResult.exitCode != 0 && iteration < l.maxIter {
		iteration++
		fmt.Printf("Phase 4/5: Fixing (iteration %d)...\n", iteration)
		code, err = l.fix(p, task, code, runResult)
		if err != nil {
			return err
		}
		runResult = l.runCode(code)
		fmt.Printf("  Exit: %d\n", runResult.exitCode)
	}

	// Phase 5: Commit
	fmt.Println("Phase 5/5: Committing...")
	l.commit(code, task)

	fmt.Println(strings.Repeat("─", 50))
	fmt.Println("✓ Autonomous task complete")
	return nil
}

func (l *Loop) plan(p provider.Provider, task string) (string, error) {
	resp, err := p.Chat(models.ChatRequest{
		Model: l.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: "You are a software architect. Create a brief implementation plan. List files and key changes. Be concise."},
			{Role: models.RoleUser, Content: task},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

func (l *Loop) write(p provider.Provider, task, plan string) (string, error) {
	fullPrompt := fmt.Sprintf("Task: %s\n\nPlan:\n%s\n\nWrite the complete code. Return ONLY the code, no explanation.", task, plan)
	resp, err := p.Chat(models.ChatRequest{
		Model: l.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: "You are an expert engineer. Write production-ready code. Return ONLY code blocks."},
			{Role: models.RoleUser, Content: fullPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   16384,
	})
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

func (l *Loop) fix(p provider.Provider, task, code string, result *runResult) (string, error) {
	prompt := fmt.Sprintf(`Task: %s

Previous code:
%s

Execution failed:
Exit code: %d
Stdout: %s
Stderr: %s

Fix the code and return the corrected version. Return ONLY the fixed code.`,
		task, code, result.exitCode, result.stdout, result.stderr)

	resp, err := p.Chat(models.ChatRequest{
		Model: l.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: "You are a debugger. Fix the code and return ONLY the fixed code."},
			{Role: models.RoleUser, Content: prompt},
		},
		Temperature: 0.3,
		MaxTokens:   16384,
	})
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

type runResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func (l *Loop) runCode(code string) *runResult {
	extracted := extractCode(code)
	tmpFile := l.sandboxDir + "\\script.py"
	os.WriteFile(tmpFile, []byte(extracted), 0644)

	cmd := exec.Command("python", tmpFile)
	cmd.Dir = l.sandboxDir
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return &runResult{
		exitCode: exitCode,
		stdout:   string(output),
		stderr:   "",
	}
}

func (l *Loop) commit(code, task string) {
	msg := fmt.Sprintf("Delta Auto: %s", truncate(task, 60))
	tmpFile := l.sandboxDir + "\\output.txt"
	os.WriteFile(tmpFile, []byte(code), 0644)
	exec.Command("git", "add", tmpFile).Run()
	exec.Command("git", "commit", "-m", msg).Run()
	fmt.Printf("  Committed: %s\n", msg)
}

func (l *Loop) Cleanup() {
	os.RemoveAll(l.sandboxDir)
}

func extractCode(text string) string {
	if strings.Contains(text, "```") {
		parts := strings.Split(text, "```")
		for i, part := range parts {
			if i%2 == 1 && !strings.HasPrefix(part, "python") && !strings.HasPrefix(part, "\npython") {
				continue
			}
			if i%2 == 1 {
				code := strings.TrimSpace(part)
				if idx := strings.Index(code, "\n"); idx != -1 {
					code = code[idx+1:]
				}
				return code
			}
		}
	}
	return text
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
