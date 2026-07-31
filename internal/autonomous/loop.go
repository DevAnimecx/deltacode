package autonomous

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DevAnimecx/deltacode/internal/agents"
	"github.com/DevAnimecx/deltacode/internal/checkpoint"
	ctxeng "github.com/DevAnimecx/deltacode/internal/context"
	"github.com/DevAnimecx/deltacode/internal/critique"
	"github.com/DevAnimecx/deltacode/internal/intelligence"
	"github.com/DevAnimecx/deltacode/internal/planning"
	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/internal/repository"
	"github.com/DevAnimecx/deltacode/internal/router"
	"github.com/DevAnimecx/deltacode/internal/symbols"
	"github.com/DevAnimecx/deltacode/internal/telemetry"
	"github.com/DevAnimecx/deltacode/internal/tools"
	"github.com/DevAnimecx/deltacode/internal/validation"
	"github.com/DevAnimecx/deltacode/pkg/models"
	"github.com/DevAnimecx/deltacode/internal/config"
)

// Task mirrors planning.Task for JSON persistence.
type Task = planning.Task

// Plan mirrors planning.Plan.
type Plan = planning.Plan

type Phase = agents.Phase

const (
	PhaseUnderstand  = agents.PhaseUnderstand
	PhaseWorldModel  = agents.PhaseWorldModel
	PhaseDecompose   = agents.PhaseDecompose
	PhaseDynamicPlan = agents.PhaseDynamicPlan
	PhaseExecute     = agents.PhaseExecute
	PhaseReflect     = agents.PhaseReflect
)

// EngineState exposes live session state for the TUI.
type EngineState struct {
	mu            sync.RWMutex
	Phase         Phase     `json:"phase"`
	Agent         string    `json:"agent"`
	TaskID        string    `json:"task_id"`
	TaskCount     int       `json:"task_count"`
	TaskDone      int       `json:"task_done"`
	Attempts      int       `json:"attempts"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	TokensUsed    int       `json:"tokens_used"`
	Cost          float64   `json:"cost"`
	LastValidation string   `json:"last_validation"`
	Timeline      []string  `json:"timeline"`
}

func (s *EngineState) Snapshot() EngineState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return EngineState{
		Phase: s.Phase, Agent: s.Agent, TaskID: s.TaskID,
		TaskCount: s.TaskCount, TaskDone: s.TaskDone, Attempts: s.Attempts,
		Provider: s.Provider, Model: s.Model, TokensUsed: s.TokensUsed,
		Cost: s.Cost, LastValidation: s.LastValidation,
		Timeline: append([]string{}, s.Timeline...),
	}
}

func (s *EngineState) set(update func(*EngineState)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	update(s)
}

func (s *EngineState) log(entry string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Timeline = append(s.Timeline, entry)
	if len(s.Timeline) > 50 {
		s.Timeline = s.Timeline[len(s.Timeline)-50:]
	}
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
	symbols    *symbols.Indexer
	router     *router.AdaptiveRouter
	tele       *telemetry.Store
	checkpts   *checkpoint.Manager
	rank       *ctxeng.Ranker
	maxRetries int
	sessionDir string
	workDir    string
	state      EngineState
}

func NewEngine(cfg *config.Manager) *Engine {
	conf := cfg.GetConfig()
	provCfg, _ := cfg.GetProvider(conf.DefaultProvider)

	e := &Engine{
		cfg:        cfg,
		model:      conf.DefaultModel,
		maxRetries: 3,
		workDir:    ".",
	}
	if provCfg != nil {
		e.provider = *provCfg
	}
	e.ctxEng, _ = ctxeng.NewEngine()
	e.mem = intelligence.NewMemory()
	e.skills = intelligence.NewSkillEngine()
	e.toolReg = tools.NewRegistry()
	e.repo = repository.NewAnalyzer(".")
	e.symbols = symbols.NewIndexer(".")
	e.tele, _ = telemetry.NewStore()
	e.router = router.NewAdaptiveRouter(conf.DefaultProvider, conf.DefaultModel, e.tele)
	e.checkpts = checkpoint.New(".")
	e.rank = ctxeng.NewRanker(".", e.symbols.Graph(), e.mem)
	e.sessionDir, _ = os.MkdirTemp("", "delta-session-*")
	return e
}

func (e *Engine) State() EngineState { return e.state.Snapshot() }

func (e *Engine) Execute(goal string) error {
	fmt.Printf("\n  Δ Cognitive Engine\n")
	fmt.Printf("  %s\n", strings.Repeat("━", 50))
	fmt.Printf("  Goal: %s\n", goal)
	fmt.Printf("  Provider: %s | Model: %s\n", e.provider.Name, e.model)
	fmt.Printf("  %s\n\n", strings.Repeat("━", 50))

	e.state.log("session started")
	e.phase(PhaseUnderstand)

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

	// Skill cache check.
	matches := e.skills.Find(goal)
	if len(matches) > 0 && matches[0].UsageCount > 2 {
		fmt.Printf("  ⚡ Using cached skill: %s\n", matches[0].Name)
		fmt.Printf("  %s\n\n", matches[0].Output)
		return nil
	}

	// Phase 1: Understand — collect context and rank relevant files.
	e.phase(PhaseUnderstand)
	fmt.Print("  Phase 1/6: Understand")
	ctx := e.ctxEng.Collect()
	e.mem.StoreEx(intelligence.LayerTask, "understand:"+goal, ctx.GitDiff, intelligence.StoreOptions{
		Tags: []string{"understand", "goal"}, Priority: 0.3, Source: "understand", Confidence: 0.5,
	})
	e.state.log("understood project context")
	fmt.Printf(" — %d context items\n", 1)

	// Phase 2: World model — symbol index + architecture.
	e.phase(PhaseWorldModel)
	fmt.Print("  Phase 2/6: World Model")
	e.symbols.IndexDirectory(".", nil)
	symCount := e.symbols.Graph().Count()
	e.state.log(fmt.Sprintf("indexed %d symbols", symCount))
	fmt.Printf(" — %d symbols indexed\n", symCount)

	// Phase 3: Decompose — planner agent builds the task graph.
	e.phase(PhaseDecompose)
	fmt.Print("  Phase 3/6: Decompose")
	plan := e.decompose(p, goal, repoInfo, ctx)
	if len(plan.Tasks) == 0 {
		return fmt.Errorf("planning failed: no tasks produced")
	}
	e.state.set(func(s *EngineState) { s.TaskCount = len(plan.Tasks) })
	fmt.Printf(" — %d tasks\n", len(plan.Tasks))

	// Phase 4: Dynamic plan — checkpoint + adaptive routing per task.
	e.phase(PhaseDynamicPlan)
	fmt.Print("  Phase 4/6: Dynamic Plan")
	ckpt, ckptErr := e.checkpts.Create("autonomous:" + goal)
	if ckptErr == nil {
		fmt.Printf(" — checkpoint %s", ckpt.ID)
	} else {
		fmt.Printf(" — checkpoint unavailable")
	}
	fmt.Println()

	// Phase 5: Execute — run each task with its agent, retry, validate.
	e.phase(PhaseExecute)
	replanned := map[string]bool{}
	replanBudget := 8
	for done := 0; done < len(plan.Tasks) || hasReady(plan); {
		t, ok := plan.NextTask()
		if !ok {
			plan.MarkFailedTasksBlocked()
			break
		}
		e.state.set(func(s *EngineState) {
			s.TaskID = t.ID
			s.Agent = t.Agent
			s.TaskDone = done
		})
		fmt.Printf("  Task %s/%d: %s [%s]\n", t.ID, len(plan.Tasks), t.Title, t.Agent)

		plan.SetTaskStatus(t.ID, planning.StatusRunning, "", "")

		var result string
		var execErr error
		connectivityErr := false
		for attempt := 0; attempt <= e.maxRetries; attempt++ {
			e.state.set(func(s *EngineState) { s.Attempts = attempt })
			if attempt > 0 {
				fmt.Printf("    ↻ Retry %d/%d\n", attempt, e.maxRetries)
			}
			result, execErr = e.executeTask(p, goal, t, plan, repoInfo)
			if execErr == nil {
				break
			}
			connectivityErr = isConnectivityError(execErr)
			if connectivityErr {
				break
			}
			if attempt < e.maxRetries {
				plan.SetTaskStatus(t.ID, planning.StatusFailed, result, execErr.Error())
			}
		}

		if execErr != nil {
			plan.SetTaskStatus(t.ID, planning.StatusFailed, result, execErr.Error())
			if !connectivityErr && !replanned[t.ID] && replanBudget > 0 {
				// Replan once per task: add a fix task reusing the failed
				// task's dependencies so it becomes immediately ready.
				replanned[t.ID] = true
				replanBudget--
				plan.Replan([]planning.Task{{
					Title:       "Fix: " + t.Title,
					Description: "Fix the failure from task " + t.ID + ": " + execErr.Error() + "\nOriginal task: " + t.Description,
					DependsOn:   t.DependsOn,
					Agent:       "Debugger",
					Files:       t.Files,
				}})
			}
			fmt.Printf("    ✗ Failed: %s\n", truncate(execErr.Error(), 200))
		} else {
			plan.SetTaskStatus(t.ID, planning.StatusDone, result, "")
			done++
			e.state.set(func(s *EngineState) { s.TaskDone = done })
			fmt.Printf("    ✓ Completed\n")
		}
	}

	// Validation gate.
	e.state.log("running validation pipeline")
	fmt.Print("  Validation gate")
	validResults := e.validate(plan)
	passed, failed := 0, 0
	for _, r := range validResults {
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}
	e.state.set(func(s *EngineState) {
		s.LastValidation = fmt.Sprintf("%d passed, %d failed", passed, failed)
	})
	if failed > 0 {
		fmt.Printf(" — %d/%d checks failed\n", failed, passed+failed)
		for _, r := range validResults {
			if !r.Passed {
				fmt.Printf("    ✗ %s\n", r.String())
			}
		}
	} else {
		fmt.Printf(" — ✓ %d checks passed\n", passed)
	}

	// Phase 6: Reflect — self-critique + learning + release summary.
	e.phase(PhaseReflect)
	fmt.Print("  Phase 6/6: Reflect")
	crit, critErr := e.reflect(p, goal, plan)
	if critErr != nil {
		fmt.Printf(" — critique unavailable\n")
	} else {
		fmt.Printf(" — score %0.0f/100 [%s]\n", crit.OverallScore, passWord(crit.Passed))
		for _, rv := range crit.Reviews {
			status := "✓"
			if !rv.Passed {
				status = "✗"
			}
			fmt.Printf("    %s %s: %0.0f/100\n", status, rv.Aspect, rv.Score)
		}
	}

	e.skills.Learn(goal, plan)
	e.mem.StoreEx(intelligence.LayerFeature, "goal:"+goal, plan.String(), intelligence.StoreOptions{
		Tags: []string{"goal", "plan"}, Priority: 0.5, Confidence: 0.7, Source: "autonomous", Verified: true,
	})
	e.state.log("learned and stored memory")
	fmt.Println(" — ✓ Saved")

	fmt.Printf("\n  %s\n", strings.Repeat("─", 50))
	fmt.Printf("  Timeline\n")
	for i, entry := range e.state.Snapshot().Timeline {
		fmt.Printf("   %2d. %s\n", i+1, entry)
	}
	fmt.Printf("  %s\n", strings.Repeat("─", 50))

	e.cleanup()
	snap := e.state.Snapshot()
	fmt.Printf("\n  %s\n", strings.Repeat("━", 50))
	fmt.Printf("  ✓ Cognitive session complete (phase=%s tasks=%d/%d tokens=%d cost=$%.4f)\n",
		snap.Phase, snap.TaskDone, snap.TaskCount, snap.TokensUsed, snap.Cost)
	return nil
}

func (e *Engine) phase(p Phase) {
	e.state.set(func(s *EngineState) { s.Phase = p })
}

func passWord(ok bool) string {
	if ok {
		return "passed"
	}
	return "needs work"
}

// decompose uses the Planner agent; falls back to a heuristic plan on failure.
func (e *Engine) decompose(p provider.Provider, goal string, repo *repository.ProjectInfo, ctx *ctxeng.Context) *planning.Plan {
	plan := planning.NewPlan(goal)

	// Try the planner agent.
	planner, ok := agents.Find("Planner")
	if ok {
		agCtx := &agents.Context{Provider: p, Model: e.model, Tools: e.toolReg}
		res, err := planner.Run(agCtx, agents.Task{
			ID:          "decompose",
			Goal:        goal,
			Description: "Decompose this goal into an execution plan.",
			RepoInfo:    fmt.Sprintf("Type: %s | Languages: %v | Files: %d | Entry: %v", repo.ProjectType, repo.Languages, repo.FileCount, repo.EntryPoints),
			Context:     formatContext(ctx),
		})
		if err == nil {
			e.state.log("planner decomposed goal")
			if tasks := extractPlanTasks(res.Output); len(tasks) > 0 {
				for _, t := range tasks {
					plan.AddTask(t)
				}
				return plan
			}
		}
	}

	// Heuristic fallback: single execute task.
	e.state.log("planner unavailable; heuristic fallback")
	plan.AddTask(planning.Task{
		ID:          "1",
		Title:       goal,
		Description: goal,
		Agent:       "Coder",
		Status:      planning.StatusPending,
	})
	return plan
}

func providerToAny(p provider.Provider) provider.Provider {
	return p
}

func formatContext(ctx *ctxeng.Context) string {
	var b strings.Builder
	b.WriteString("Project Context:\n")
	if ctx.Readme != "" {
		fmt.Fprintf(&b, "\nREADME:\n%s\n", truncate(ctx.Readme, 500))
	}
	if ctx.PackageJSON != "" {
		fmt.Fprintf(&b, "\nDependencies:\n%s\n", truncate(ctx.PackageJSON, 500))
	}
	if ctx.FileTree != "" {
		fmt.Fprintf(&b, "\nFile Tree:\n%s\n", truncate(ctx.FileTree, 2000))
	}
	if ctx.GitLog != "" {
		fmt.Fprintf(&b, "\nRecent Git History:\n%s\n", ctx.GitLog)
	}
	if ctx.GitDiff != "" {
		fmt.Fprintf(&b, "\nUncommitted Changes:\n%s\n", truncate(ctx.GitDiff, 1000))
	}
	return b.String()
}

func extractPlanTasks(out string) []planning.Task {
	clean := stripCodeFence(out)
	if i := strings.Index(clean, "{"); i != -1 {
		if j := strings.LastIndex(clean, "}"); j > i {
			clean = clean[i : j+1]
		}
	}
	var parsed struct {
		Tasks []struct {
			ID          string   `json:"id"`
			Title       string   `json:"title"`
			Description string   `json:"description"`
			DependsOn   []string `json:"depends_on"`
			Agent       string   `json:"agent"`
			File        string   `json:"file"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(clean), &parsed); err != nil {
		return nil
	}
	var tasks []planning.Task
	for i, t := range parsed.Tasks {
		agent := t.Agent
		if agent == "" {
			agent = "Coder"
		}
		if _, ok := agents.Find(agent); !ok {
			agent = "Coder"
		}
		id := t.ID
		if id == "" {
			id = fmt.Sprintf("%d", i+1)
		}
		tasks = append(tasks, planning.Task{
			ID:          id,
			Title:       firstNonEmpty(t.Title, truncate(t.Description, 80)),
			Description: t.Description,
			DependsOn:   t.DependsOn,
			Agent:       agent,
			Files:       []string{t.File},
			Status:      planning.StatusPending,
		})
	}
	return tasks
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

func stripCodeFence(s string) string {
	if strings.Contains(s, "```") {
		parts := strings.Split(s, "```")
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
	return strings.TrimSpace(s)
}

func (e *Engine) executeTask(p provider.Provider, goal string, t planning.Task, plan *planning.Plan, repoInfo *repository.ProjectInfo) (string, error) {
	agentName := t.Agent
	if agentName == "" {
		agentName = "Coder"
	}

	// Record telemetry around the agent call.
	start := time.Now()
	ok := false
	var result *agents.Result
	var err error

	agent, found := agents.Find(agentName)
	if !found {
		return "", fmt.Errorf("unknown agent %q", agentName)
	}

	agCtx := &agents.Context{Provider: providerToAny(p), Model: e.model, Tools: e.toolReg}
	task := agents.Task{
		ID:          t.ID,
		Goal:        goal,
		Description: t.Description,
		Context:     plan.String(),
		RepoInfo:    fmt.Sprintf("Type: %s | Languages: %v | Files: %d", repoInfo.ProjectType, repoInfo.Languages, repoInfo.FileCount),
		Results:     e.collectTaskResults(plan, t),
	}
	if len(t.Files) > 0 && t.Files[0] != "" {
		task.File = t.Files[0]
		if data, rerr := os.ReadFile(filepath.Join(e.workDir, task.File)); rerr == nil {
			task.Code = string(data)
		}
	}

	e.state.set(func(s *EngineState) { s.Agent = agentName })
	result, err = agent.Run(agCtx, task)
	dur := time.Since(start)
	if err == nil {
		ok = true
	}

	// Persist files written by the agent.
	if result != nil && len(result.Files) > 0 {
		for _, f := range result.Files {
			code := agents.ExtractCodeBlock(result.Output)
			if code == "" {
				continue
			}
			full := filepath.Join(e.workDir, f)
			os.MkdirAll(filepath.Dir(full), 0755)
			if werr := os.WriteFile(full, []byte(code), 0644); werr != nil {
				return "", werr
			}
			e.autoFormat(full)
		}
	}

	// If the agent named a file but produced no output, apply the whole output.
	if result != nil && len(result.Files) > 0 {
		for _, f := range result.Files {
			full := filepath.Join(e.workDir, f)
			if data, rerr := os.ReadFile(full); rerr == nil && len(strings.TrimSpace(string(data))) > 0 {
				continue
			}
			code := agents.ExtractCodeBlock(result.Output)
			if code == "" {
				continue
			}
			os.MkdirAll(filepath.Dir(full), 0755)
			if werr := os.WriteFile(full, []byte(code), 0644); werr != nil {
				return "", werr
			}
			e.autoFormat(full)
		}
	}

	// Telemetry recording (approximate cost).
	tokens := estimateTokens(result, err)
	cost := estimateCost(e.provider, tokens)
	e.tele.RecordCall(e.provider.Name, e.model, float64(dur.Milliseconds()), tokens, cost, ok, 0)
	e.tele.RecordEvent(telemetry.Event{
		Provider: e.provider.Name, Model: e.model, Agent: agentName, Phase: string(PhaseExecute),
		TaskID: t.ID, Type: "agent_call", LatencyMs: float64(dur.Milliseconds()),
		Tokens: tokens, Cost: cost, OK: ok,
	})
	e.state.set(func(s *EngineState) {
		s.TokensUsed += tokens
		s.Cost += cost
	})

	if err != nil {
		return "", err
	}
	// Empty output means the model produced nothing usable — treat as failure so retry kicks in.
	if strings.TrimSpace(result.Output) == "" {
		return "", fmt.Errorf("agent %s returned empty output", agentName)
	}
	return result.Output, nil
}

func hasReady(plan *planning.Plan) bool {
	_, ok := plan.NextTask()
	return ok
}

func isConnectivityError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "connectex")
}

func (e *Engine) autoFormat(path string) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		if _, err := exec.LookPath("gofmt"); err == nil {
			exec.Command("gofmt", "-w", path).Run()
		}
	case ".py":
		if _, err := exec.LookPath("black"); err == nil {
			exec.Command("black", "-q", path).Run()
		}
	}
}

func (e *Engine) collectTaskResults(plan *planning.Plan, t planning.Task) []string {
	var results []string
	for _, dep := range t.DependsOn {
		if dt, ok := plan.GetTask(dep); ok && dt.Result != "" {
			results = append(results, dt.Result)
		}
	}
	return results
}

func (e *Engine) validate(plan *planning.Plan) []validation.Result {
	pipeline := validation.New(e.workDir)
	var files []validation.File
	for _, t := range plan.Tasks {
		for _, f := range t.Files {
			if f != "" {
				files = append(files, validation.File{Path: f})
			}
		}
	}
	var results []validation.Result
	if len(files) > 0 {
		results = append(results, pipeline.ValidateFiles(files)...)
		results = append(results, pipeline.CheckSecurity(files)...)
	}
	results = append(results, pipeline.RunGoChecks()...)
	return results
}

func (e *Engine) reflect(p provider.Provider, goal string, plan *planning.Plan) (*critique.Result, error) {
	var code strings.Builder
	for _, t := range plan.Tasks {
		if t.Result != "" {
			code.WriteString(agents.ExtractCodeBlock(t.Result))
			code.WriteString("\n\n")
		}
		for _, f := range t.Files {
			if data, err := os.ReadFile(filepath.Join(e.workDir, f)); err == nil {
				code.Write(data)
				code.WriteString("\n")
			}
		}
	}
	if code.Len() == 0 {
		return nil, fmt.Errorf("no code produced to critique")
	}
	eng := critique.New(providerToAny(p), e.model)
	res, err := eng.ReviewCode(goal, code.String(), nil, plan.String())
	if err != nil {
		return nil, err
	}
	// Auto-iterate if below threshold: ask Debugger to fix.
	if !res.Passed {
		debugger, _ := agents.Find("Debugger")
		if debugger != nil {
			fixed, ferr := debugger.Run(&agents.Context{Provider: providerToAny(p), Model: e.model, Tools: e.toolReg}, agents.Task{
				ID: "critique-fix", Goal: goal,
				Description: "Fix the issues from code review.",
				Code:        code.String(),
				Context:     plan.String(),
			})
			if ferr == nil && fixed != nil && fixed.Output != "" {
				e.state.log("critique triggered auto-fix")
			}
		}
	}
	e.state.log(fmt.Sprintf("critique score %0.0f", res.OverallScore))
	return res, nil
}

func estimateTokens(res *agents.Result, err error) int {
	if res == nil {
		return 0
	}
	return len(res.Output) / 4
}

func estimateCost(prov models.ProviderConfig, tokens int) float64 {
	perMTok := 0.0
	switch prov.Type {
	case models.ProviderAnthropic:
		perMTok = 3.0
	case models.ProviderGoogle:
		perMTok = 0.5
	case models.ProviderOllama:
		perMTok = 0
	default:
		perMTok = 1.0
	}
	return float64(tokens) / 1e6 * perMTok
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
