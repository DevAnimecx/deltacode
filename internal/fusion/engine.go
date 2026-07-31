package fusion

import (
	"fmt"
	"strings"
	"sync"

	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

type TaskType string

const (
	TaskFrontend TaskType = "frontend"
	TaskBackend  TaskType = "backend"
	TaskDatabase TaskType = "database"
	TaskAPI      TaskType = "api"
	TaskTest     TaskType = "test"
	TaskBugFix   TaskType = "bugfix"
	TaskReview   TaskType = "review"
	TaskOptimize TaskType = "optimize"
	TaskDocs     TaskType = "docs"
	TaskConfig   TaskType = "config"
	TaskDocker   TaskType = "docker"
	TaskCI       TaskType = "ci"
	TaskSecurity TaskType = "security"
	TaskUI       TaskType = "ui"
)

type GraphNode struct {
	ID          string   `json:"id"`
	TaskType    TaskType `json:"task_type"`
	Description string   `json:"description"`
	Prompt      string   `json:"prompt"`
	Provider    string   `json:"provider"`
	Model       string   `json:"model"`
	Deps        []string `json:"deps"`
	Output      string   `json:"output"`
	Status      string   `json:"status"`
}

type TaskGraph struct {
	Nodes []*GraphNode        `json:"nodes"`
	Edges map[string][]string `json:"edges"`
}

type FusionEngine struct {
	providers map[string]models.ProviderConfig
}

func NewFusionEngine(providers []models.ProviderConfig) *FusionEngine {
	pm := make(map[string]models.ProviderConfig)
	for _, p := range providers {
		pm[p.Name] = p
	}
	return &FusionEngine{providers: pm}
}

func (fe *FusionEngine) Plan(prompt string) *TaskGraph {
	p := strings.ToLower(prompt)
	graph := &TaskGraph{Edges: make(map[string][]string)}

	// Detect project type and decompose
	if containsAny(p, []string{"fullstack", "full stack", "app", "web app", "clone", "dashboard"}) {
		graph.Nodes = append(graph.Nodes,
			&GraphNode{ID: "arch", TaskType: TaskBackend, Description: "Design architecture", Prompt: fmt.Sprintf("Design the architecture for: %s", prompt), Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
			&GraphNode{ID: "frontend", TaskType: TaskFrontend, Description: "Build frontend", Prompt: fmt.Sprintf("Build the frontend for: %s", prompt), Provider: "deepseek", Model: "deepseek-chat", Deps: []string{"arch"}},
			&GraphNode{ID: "backend", TaskType: TaskBackend, Description: "Build backend", Prompt: fmt.Sprintf("Build the backend for: %s", prompt), Provider: "deepseek", Model: "deepseek-chat", Deps: []string{"arch"}},
			&GraphNode{ID: "api", TaskType: TaskAPI, Description: "Build API layer", Prompt: fmt.Sprintf("Build the API layer connecting frontend and backend for: %s", prompt), Provider: "deepseek", Model: "deepseek-chat", Deps: []string{"frontend", "backend"}},
			&GraphNode{ID: "tests", TaskType: TaskTest, Description: "Write tests", Prompt: fmt.Sprintf("Write tests for: %s", prompt), Provider: "google", Model: "gemini-2.0-flash", Deps: []string{"api"}},
		)
	} else if containsAny(p, []string{"fix", "bug", "debug", "error", "issue"}) {
		graph.Nodes = append(graph.Nodes,
			&GraphNode{ID: "analyze", TaskType: TaskBugFix, Description: "Analyze bug", Prompt: fmt.Sprintf("Analyze and fix this issue: %s", prompt), Provider: "anthropic", Model: "claude-sonnet-4-20250514"},
			&GraphNode{ID: "fix", TaskType: TaskBugFix, Description: "Apply fix", Prompt: fmt.Sprintf("Implement the fix for: %s", prompt), Provider: "deepseek", Model: "deepseek-chat", Deps: []string{"analyze"}},
			&GraphNode{ID: "verify", TaskType: TaskTest, Description: "Verify fix", Prompt: fmt.Sprintf("Verify the fix works for: %s", prompt), Provider: "google", Model: "gemini-2.0-flash", Deps: []string{"fix"}},
		)
	} else if containsAny(p, []string{"test", "unit", "integration"}) {
		graph.Nodes = append(graph.Nodes,
			&GraphNode{ID: "tests", TaskType: TaskTest, Description: "Generate tests", Prompt: prompt, Provider: "google", Model: "gemini-2.0-flash"},
		)
	} else if containsAny(p, []string{"docker", "deploy", "ci", "cd"}) {
		graph.Nodes = append(graph.Nodes,
			&GraphNode{ID: "infra", TaskType: TaskDocker, Description: "Setup infrastructure", Prompt: prompt, Provider: "deepseek", Model: "deepseek-chat"},
		)
	} else {
		graph.Nodes = append(graph.Nodes,
			&GraphNode{ID: "code", TaskType: TaskBackend, Description: "Generate code", Prompt: prompt, Provider: "deepseek", Model: "deepseek-chat"},
			&GraphNode{ID: "review", TaskType: TaskReview, Description: "Review output", Prompt: fmt.Sprintf("Review this code: %s", prompt), Provider: "openai", Model: "gpt-4o-mini", Deps: []string{"code"}},
		)
	}

	return graph
}

func (fe *FusionEngine) Execute(graph *TaskGraph) error {
	if len(graph.Nodes) == 0 {
		return fmt.Errorf("empty task graph")
	}

	fmt.Printf("Δ Fusion Engine — Executing %d tasks\n", len(graph.Nodes))
	fmt.Println(strings.Repeat("─", 50))

	for _, node := range graph.Nodes {
		fmt.Printf("  ▶ %s [%s] → %s/%s\n", node.ID, node.Description, node.Provider, node.Model)
	}
	fmt.Println(strings.Repeat("─", 50))

	var mu sync.Mutex
	var wg sync.WaitGroup
	completed := make(map[string]string)

	for _, node := range graph.Nodes {
		node.Status = "pending"
	}

	for {
		allDone := true
		var runBatch []*GraphNode

		for _, node := range graph.Nodes {
			if node.Status == "completed" {
				continue
			}
			allDone = false

			depsMet := true
			for _, dep := range node.Deps {
				if completed[dep] == "" {
					depsMet = false
					break
				}
			}
			if depsMet && node.Status != "running" {
				node.Status = "running"
				runBatch = append(runBatch, node)
			}
		}

		if allDone {
			break
		}
		if len(runBatch) == 0 && !allDone {
			for _, node := range graph.Nodes {
				if node.Status == "pending" {
					fmt.Printf("  ! %s blocked by dependencies\n", node.ID)
				}
			}
			break
		}

		for _, node := range runBatch {
			wg.Add(1)
			go func(n *GraphNode) {
				defer wg.Done()

				// Inject dependency outputs
				fullPrompt := n.Prompt
				if len(n.Deps) > 0 {
					var depContext strings.Builder
					depContext.WriteString("Context from previous steps:\n")
					for _, dep := range n.Deps {
						if out := completed[dep]; out != "" {
							depContext.WriteString(fmt.Sprintf("\n--- %s output ---\n%s\n", dep, truncate(out, 2000)))
						}
					}
					fullPrompt = depContext.String() + "\n\n" + n.Prompt
				}

				provCfg, ok := fe.providers[n.Provider]
				if !ok {
					n.Output = fmt.Sprintf("Error: provider %q not configured", n.Provider)
					n.Status = "error"
					return
				}

				p, err := provider.NewProvider(provCfg)
				if err != nil {
					n.Output = fmt.Sprintf("Error: %v", err)
					n.Status = "error"
					return
				}

				resp, err := p.Chat(models.ChatRequest{
					Model: n.Model,
					Messages: []models.Message{
						{Role: models.RoleSystem, Content: "You are an expert software engineer. Write production-ready code. Be concise."},
						{Role: models.RoleUser, Content: fullPrompt},
					},
					Temperature: 0.3,
					MaxTokens:   8192,
				})
				if err != nil {
					n.Output = fmt.Sprintf("Error: %v", err)
					n.Status = "error"
					return
				}

				mu.Lock()
				n.Output = resp.Message.Content
				n.Status = "completed"
				completed[n.ID] = resp.Message.Content
				fmt.Printf("  ✓ %s completed (%d tokens)\n", n.ID, resp.Usage.TotalTokens)
				mu.Unlock()
			}(node)
		}
		wg.Wait()
	}

	return nil
}

func (fe *FusionEngine) Merge(graph *TaskGraph) string {
	var b strings.Builder
	b.WriteString("# Delta Fusion Output\n\n")

	for _, node := range graph.Nodes {
		if node.Status != "completed" {
			continue
		}
		b.WriteString(fmt.Sprintf("## %s: %s\n\n", node.ID, node.Description))
		b.WriteString(node.Output)
		b.WriteString("\n\n---\n\n")
	}

	return b.String()
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
