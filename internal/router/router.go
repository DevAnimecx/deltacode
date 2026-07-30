package router

import (
	"strings"

	"github.com/DevAnimecx/deltacode/pkg/models"
)

type TaskType string

const (
	TaskGeneral     TaskType = "general"
	TaskCode        TaskType = "code"
	TaskArchitect   TaskType = "architecture"
	TaskDebug       TaskType = "debug"
	TaskRefactor    TaskType = "refactor"
	TaskTest        TaskType = "test"
	TaskExplain     TaskType = "explain"
	TaskReview      TaskType = "review"
	TaskPlan        TaskType = "plan"
	TaskUI          TaskType = "ui"
	TaskBackend     TaskType = "backend"
	TaskDocs        TaskType = "docs"
	TaskOptimize    TaskType = "optimize"
	TaskSecurity    TaskType = "security"
)

type Router struct {
	defaultProvider string
	defaultModel    string
	rules           []Rule
}

type Rule struct {
	TaskType TaskType
	Provider string
	Model    string
	Priority int
}

func NewRouter(defaultProvider, defaultModel string) *Router {
	r := &Router{
		defaultProvider: defaultProvider,
		defaultModel:    defaultModel,
	}

	r.rules = []Rule{
		{TaskType: TaskArchitect, Provider: "anthropic", Model: "claude-sonnet-4-20250514", Priority: 100},
		{TaskType: TaskPlan, Provider: "anthropic", Model: "claude-sonnet-4-20250514", Priority: 90},
		{TaskType: TaskSecurity, Provider: "anthropic", Model: "claude-sonnet-4-20250514", Priority: 85},
		{TaskType: TaskReview, Provider: "openai", Model: "gpt-4o", Priority: 80},
		{TaskType: TaskDebug, Provider: "google", Model: "gemini-2.0-flash", Priority: 70},
		{TaskType: TaskCode, Provider: "deepseek", Model: "deepseek-chat", Priority: 60},
		{TaskType: TaskRefactor, Provider: "deepseek", Model: "deepseek-chat", Priority: 50},
		{TaskType: TaskTest, Provider: "google", Model: "gemini-2.0-flash", Priority: 40},
		{TaskType: TaskUI, Provider: "deepseek", Model: "deepseek-chat", Priority: 35},
		{TaskType: TaskBackend, Provider: "deepseek", Model: "deepseek-chat", Priority: 30},
		{TaskType: TaskExplain, Provider: "openai", Model: "gpt-4o-mini", Priority: 25},
		{TaskType: TaskDocs, Provider: "openai", Model: "gpt-4o-mini", Priority: 20},
		{TaskType: TaskOptimize, Provider: "deepseek", Model: "deepseek-chat", Priority: 15},
		{TaskType: TaskGeneral, Provider: defaultProvider, Model: defaultModel, Priority: 0},
	}

	return r
}

func (r *Router) Classify(prompt string) TaskType {
	p := strings.ToLower(prompt)

	if matchesAny(p, []string{"architecture", "design", "system design", "architect", "component diagram", "data flow"}) {
		return TaskArchitect
	}
	if matchesAny(p, []string{"plan", "roadmap", "milestone", "step by step", "break down"}) {
		return TaskPlan
	}
	if matchesAny(p, []string{"debug", "fix", "bug", "error", "issue", "not working", "broken"}) {
		return TaskDebug
	}
	if matchesAny(p, []string{"refactor", "clean up", "improve", "restructure", "simplify"}) {
		return TaskRefactor
	}
	if matchesAny(p, []string{"test", "unit test", "integration test", "spec", "coverage"}) {
		return TaskTest
	}
	if matchesAny(p, []string{"explain", "what does", "how does", "understand", "why"}) {
		return TaskExplain
	}
	if matchesAny(p, []string{"review", "code review", "audit", "check"}) {
		return TaskReview
	}
	if matchesAny(p, []string{"ui", "frontend", "component", "react", "vue", "css", "html", "interface"}) {
		return TaskUI
	}
	if matchesAny(p, []string{"api", "backend", "server", "endpoint", "database", "rest", "graphql"}) {
		return TaskBackend
	}
	if matchesAny(p, []string{"doc", "readme", "documentation", "comment", "help"}) {
		return TaskDocs
	}
	if matchesAny(p, []string{"optimize", "performance", "slow", "fast", "cache", "bottleneck"}) {
		return TaskOptimize
	}
	if matchesAny(p, []string{"security", "vulnerability", "injection", "xss", "csrf", "auth"}) {
		return TaskSecurity
	}
	if matchesAny(p, []string{"write", "create", "implement", "build", "generate", "add", "make"}) {
		return TaskCode
	}

	return TaskGeneral
}

func (r *Router) Route(prompt string, providers []models.ProviderConfig) (string, string) {
	taskType := r.Classify(prompt)

	for _, rule := range r.rules {
		if rule.TaskType != taskType {
			continue
		}
		if providerExists(rule.Provider, providers) {
			return rule.Provider, rule.Model
		}
	}

	return r.defaultProvider, r.defaultModel
}

func (r *Router) GetTaskType(prompt string) TaskType {
	return r.Classify(prompt)
}

func matchesAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func (r *Router) Rules() []Rule {
	return r.rules
}

func providerExists(name string, providers []models.ProviderConfig) bool {
	for _, p := range providers {
		if p.Name == name {
			return true
		}
	}
	return false
}
