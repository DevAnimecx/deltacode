package agents

import (
	"fmt"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/internal/tools"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

type Phase string

const (
	PhaseUnderstand  Phase = "understand"
	PhaseWorldModel  Phase = "world-model"
	PhaseDecompose   Phase = "decompose"
	PhaseDynamicPlan Phase = "dynamic-plan"
	PhaseExecute     Phase = "execute"
	PhaseReflect     Phase = "reflect"
)

type Task struct {
	ID          string
	Goal        string
	Description string
	File        string
	Code        string
	Context     string
	RepoInfo    string
	Results     []string
}

type Result struct {
	Output    string
	Files     []string
	Duration  time.Duration
	Validated bool
}

type Context struct {
	Provider provider.Provider
	Model    string
	Tools    *tools.Registry
}

func (c *Context) chat(system, user string, temperature float64, maxTokens int) (string, error) {
	resp, err := c.Provider.Chat(models.ChatRequest{
		Model: c.Model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: system},
			{Role: models.RoleUser, Content: user},
		},
		Temperature: temperature,
		MaxTokens:   maxTokens,
	})
	if err != nil {
		return "", err
	}
	return resp.Message.Content, nil
}

type Agent interface {
	Name() string
	Phase() Phase
	Description() string
	Run(ctx *Context, task Task) (*Result, error)
}

type baseAgent struct {
	name         string
	phase        Phase
	description  string
	systemPrompt string
	temperature  float64
	maxTokens    int
	runFn        func(ctx *Context, task Task, base *baseAgent) (*Result, error)
}

func (b *baseAgent) Name() string        { return b.name }
func (b *baseAgent) Phase() Phase        { return b.phase }
func (b *baseAgent) Description() string { return b.description }

func (b *baseAgent) Run(ctx *Context, task Task) (*Result, error) {
	start := time.Now()
	res, err := b.runFn(ctx, task, b)
	if err != nil {
		return nil, err
	}
	res.Duration = time.Since(start)
	return res, nil
}

func (b *baseAgent) chat(ctx *Context, userPrompt string) (string, error) {
	return ctx.chat(b.systemPrompt, userPrompt, b.temperature, b.maxTokens)
}

func buildUserPrompt(task Task) string {
	var b strings.Builder
	b.WriteString("Goal: " + task.Goal + "\n\n")
	if task.Description != "" {
		b.WriteString("Task: " + task.Description + "\n\n")
	}
	if task.Context != "" {
		b.WriteString("Project Context:\n" + truncate(task.Context, 4000) + "\n\n")
	}
	if task.RepoInfo != "" {
		b.WriteString("Repo Info:\n" + task.RepoInfo + "\n\n")
	}
	if task.Code != "" {
		b.WriteString("Code:\n" + truncate(task.Code, 8000) + "\n\n")
	}
	if len(task.Results) > 0 {
		b.WriteString("Previous Results:\n")
		for _, r := range task.Results {
			b.WriteString(truncate(r, 2000) + "\n---\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (b *baseAgent) run(ctx *Context, task Task) (*Result, error) {
	out, err := b.chat(ctx, buildUserPrompt(task))
	if err != nil {
		return nil, err
	}
	return &Result{Output: out}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func ExtractCodeBlock(text string) string {
	if !strings.Contains(text, "```") {
		return strings.TrimSpace(text)
	}
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
	return strings.TrimSpace(text)
}

var _ = fmt.Sprintf
