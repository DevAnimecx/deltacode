package engine

import (
	"fmt"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/autonomous"
	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/context"
	"github.com/DevAnimecx/deltacode/internal/intelligence"
	"github.com/DevAnimecx/deltacode/internal/memory"
	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/internal/router"
	"github.com/DevAnimecx/deltacode/internal/skill"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

type Engine struct {
	cfg        *config.Manager
	context    *context.Engine
	mem        *memory.ProjectMemory
	vector     *memory.VectorMemory
	skills     *skill.Engine
	intelMem   *intelligence.Memory
	skillEng   *intelligence.SkillEngine
	router     *router.Router
	autoEngine *autonomous.Engine
	sessionID  int64
}

func New(cfg *config.Manager) *Engine {
	e := &Engine{cfg: cfg}

	e.context, _ = context.NewEngine()

	if m, err := memory.NewProjectMemory(); err == nil {
		e.mem = m
		e.sessionID, _ = m.CreateSession("auto-session")
	}

	if v, err := memory.NewVectorMemory(); err == nil {
		e.vector = v
	}

	if s, err := skill.NewEngine(); err == nil {
		e.skills = s
	}

	e.intelMem = intelligence.NewMemory()
	e.skillEng = intelligence.NewSkillEngine()

	conf := cfg.GetConfig()
	e.router = router.NewRouter(conf.DefaultProvider, conf.DefaultModel)
	e.autoEngine = autonomous.NewEngine(cfg)

	return e
}

func (e *Engine) RunPrompt(prompt string, stream bool) error {
	conf := e.cfg.GetConfig()

	if e.skills != nil {
		matches := e.skills.Find(prompt)
		if len(matches) > 0 && matches[0].UsageCount > 3 {
			fmt.Println("Using cached skill:", matches[0].Name)
			fmt.Println(matches[0].Output)
			return nil
		}
	}

	provName, modelName := e.router.Route(prompt, conf.Providers)
	fmt.Printf("→ Task: %s\n", e.router.GetTaskType(prompt))
	fmt.Printf("→ Provider: %s | Model: %s\n", provName, modelName)

	provCfg, err := e.cfg.GetProvider(provName)
	if err != nil {
		provCfg, err = e.cfg.GetProvider(conf.DefaultProvider)
		if err != nil {
			return fmt.Errorf("no provider configured. Add one with `delta provider add`")
		}
	}

	p, err := provider.NewProvider(*provCfg)
	if err != nil {
		return err
	}

	ctxPrompt := prompt
	if e.context != nil {
		ctxPrompt = e.context.BuildPrompt(prompt)
	}

	messages := []models.Message{
		{Role: models.RoleSystem, Content: "You are Delta Code, an expert software engineer. Write production-ready code. Be concise and precise."},
	}

	if e.vector != nil {
		results := e.vector.Search(prompt, 3)
		if len(results) > 0 {
			var memContext strings.Builder
			memContext.WriteString("Relevant context from previous sessions:\n")
			for i, r := range results {
				content := r.Entry.Content
				if len(content) > 500 {
					content = content[:500] + "..."
				}
				memContext.WriteString(fmt.Sprintf("%d. %s\n", i+1, content))
			}
			messages = append(messages, models.Message{Role: models.RoleSystem, Content: memContext.String()})
		}
	}

	messages = append(messages, models.Message{Role: models.RoleUser, Content: ctxPrompt})

	req := models.ChatRequest{
		Model:       modelName,
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   8192,
	}

	var fullResponse string
	if stream {
		ch, err := p.ChatStream(req)
		if err != nil {
			return err
		}
		var sb strings.Builder
		hadContent := false
		for chunk := range ch {
			if chunk.Error != nil {
				return chunk.Error
			}
			if chunk.Content != "" {
				hadContent = true
				sb.WriteString(chunk.Content)
				fmt.Print(chunk.Content)
			}
			if chunk.Done {
				break
			}
		}
		fullResponse = sb.String()
		if !hadContent && fullResponse == "" {
			fmt.Println()
			resp, err := p.Chat(req)
			if err != nil {
				return err
			}
			fullResponse = resp.Message.Content
			fmt.Println(fullResponse)
		} else {
			fmt.Println()
		}
	} else {
		resp, err := p.Chat(req)
		if err != nil {
			return err
		}
		fullResponse = resp.Message.Content
		fmt.Println(fullResponse)
	}

	// Built-in Reviewer: auto-review using secondary model
	if len(fullResponse) > 100 {
		e.autoReview(fullResponse, prompt)
	}

	e.storeMemory(prompt, fullResponse)
	return nil
}

func (e *Engine) autoReview(code, prompt string) {
	conf := e.cfg.GetConfig()

	reviewProv := conf.DefaultProvider
	reviewModel := conf.DefaultModel

	provCfg, err := e.cfg.GetProvider(reviewProv)
	if err != nil {
		return
	}

	p, err := provider.NewProvider(*provCfg)
	if err != nil {
		return
	}

	resp, err := p.Chat(models.ChatRequest{
		Model: reviewModel,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: `Review the following code briefly. Rate 1-10 and list any issues.
Format: "Score: X/10\nIssues:\n- issue1\n- issue2\nOr: No issues found."`},
			{Role: models.RoleUser, Content: fmt.Sprintf("Task: %s\n\nCode:\n%s", prompt, truncate(code, 3000))},
		},
		Temperature: 0.2,
		MaxTokens:   1024,
	})
	if err != nil {
		return
	}

	review := resp.Message.Content
	if !strings.Contains(review, "No issues") && !strings.Contains(review, "Score: 10") {
		fmt.Println("\n--- Auto Review ---")
		fmt.Println(review)
		fmt.Println("-------------------")
	}
}

func (e *Engine) storeMemory(prompt, response string) {
	if e.mem != nil {
		e.mem.AddEntry(e.sessionID, "user", prompt, nil)
		e.mem.AddEntry(e.sessionID, "assistant", response, map[string]any{"auto_reviewed": true})
	}
	if e.vector != nil {
		tags := extractTags(prompt)
		e.vector.Store(fmt.Sprintf("Q: %s\nA: %s", prompt, response), tags)
	}
	if e.skills != nil {
		if len(response) > 50 && strings.Contains(strings.ToLower(prompt), "create") {
			e.skills.Save(
				generateSkillName(prompt),
				prompt,
				response,
				extractTags(prompt),
			)
		}
	}
}

func extractTags(text string) []string {
	words := strings.Fields(strings.ToLower(text))
	var tags []string
	seen := make(map[string]bool)
	for _, w := range words {
		if len(w) > 3 && !seen[w] {
			tags = append(tags, w)
			seen[w] = true
		}
		if len(tags) >= 5 {
			break
		}
	}
	return tags
}

func generateSkillName(prompt string) string {
	words := strings.Fields(prompt)
	if len(words) > 6 {
		return strings.Join(words[:6], "-")
	}
	return strings.Join(words, "-")
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
