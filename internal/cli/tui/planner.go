package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/planning"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) planGoal(goal string) *planning.Plan {
	p := planning.NewPlan(goal)
	intent := m.parseIntent(goal)

	switch intent.Type {
	case IntentBuild:
		m.addBuildTasks(p, intent)
	case IntentFix:
		m.addFixTasks(p, intent)
	case IntentRefactor:
		m.addRefactorTasks(p, intent)
	case IntentTest:
		m.addTestTasks(p, intent)
	case IntentExplain:
		p.AddTask(planning.Task{
			ID: "1", Title: "Analyze codebase", Description: "Read relevant files and explain",
			Agent: "research", Files: intent.Files,
		})
	default:
		p.AddTask(planning.Task{ID: "1", Title: "Execute goal", Description: goal, Agent: "coding"})
	}

	return p
}

func (m *model) addBuildTasks(p *planning.Plan, intent Intent) {
	tasks := []planning.Task{
		{ID: "1", Title: "Database schema", Description: "Design and create database schema", DependsOn: []string{}, Agent: "plan", Files: intent.Files},
		{ID: "2", Title: "Backend API", Description: "Implement backend endpoints", DependsOn: []string{"1"}, Agent: "coding", Files: intent.Files},
		{ID: "3", Title: "Frontend UI", Description: "Build user interface", DependsOn: []string{"2"}, Agent: "coding", Files: intent.Files},
		{ID: "4", Title: "Tests", Description: "Write unit and integration tests", DependsOn: []string{"2", "3"}, Agent: "test", Files: intent.Files},
		{ID: "5", Title: "Documentation", Description: "Document the feature", DependsOn: []string{"2", "3", "4"}, Agent: "docs", Files: intent.Files},
	}
	for _, t := range tasks {
		p.AddTask(t)
	}
}

func (m *model) addFixTasks(p *planning.Plan, intent Intent) {
	tasks := []planning.Task{
		{ID: "1", Title: "Reproduce issue", Description: "Identify and reproduce the bug", DependsOn: []string{}, Agent: "research", Files: intent.Files},
		{ID: "2", Title: "Root cause analysis", Description: "Analyze the root cause", DependsOn: []string{"1"}, Agent: "plan", Files: intent.Files},
		{ID: "3", Title: "Implement fix", Description: "Apply the fix", DependsOn: []string{"2"}, Agent: "coding", Files: intent.Files},
		{ID: "4", Title: "Validate fix", Description: "Run tests and validate", DependsOn: []string{"3"}, Agent: "test", Files: intent.Files},
	}
	for _, t := range tasks {
		p.AddTask(t)
	}
}

func (m *model) addRefactorTasks(p *planning.Plan, intent Intent) {
	tasks := []planning.Task{
		{ID: "1", Title: "Analyze code", Description: "Analyze current code structure", DependsOn: []string{}, Agent: "research", Files: intent.Files},
		{ID: "2", Title: "Refactor", Description: "Apply refactoring changes", DependsOn: []string{"1"}, Agent: "coding", Files: intent.Files},
		{ID: "3", Title: "Validate", Description: "Run tests to ensure no regressions", DependsOn: []string{"2"}, Agent: "test", Files: intent.Files},
	}
	for _, t := range tasks {
		p.AddTask(t)
	}
}

func (m *model) addTestTasks(p *planning.Plan, intent Intent) {
	tasks := []planning.Task{
		{ID: "1", Title: "Scan code", Description: "Scan codebase for test gaps", DependsOn: []string{}, Agent: "research", Files: intent.Files},
		{ID: "2", Title: "Write tests", Description: "Write missing tests", DependsOn: []string{"1"}, Agent: "test", Files: intent.Files},
		{ID: "3", Title: "Run coverage", Description: "Run tests and report coverage", DependsOn: []string{"2"}, Agent: "test", Files: intent.Files},
	}
	for _, t := range tasks {
		p.AddTask(t)
	}
}

func (m *model) renderPlanCard(p *planning.Plan) string {
	intent := m.parseIntent(p.Goal)
	done, total := p.Progress()

	var b strings.Builder
	b.WriteString(m.t.badge.Render(fmt.Sprintf(" PLAN  %s  [%s] ", p.ID, p.Status)))
	b.WriteString("\n")
	b.WriteString(m.t.uM.Render("  Goal: " + p.Goal))
	b.WriteString("\n")
	b.WriteString(m.t.dim.Render(fmt.Sprintf("  Intent: %s | Complexity: %s | Est. cost: $%.4f", intent.Type, intent.Complexity, intent.EstimatedCost)))
	b.WriteString("\n")
	b.WriteString(m.t.dim.Render(fmt.Sprintf("  Progress: %d/%d tasks (%d replans)", done, total, p.ReplanCount)))
	b.WriteString("\n")
	b.WriteString("\n")

	for _, t := range p.Tasks {
		icon := "□"
		switch t.Status {
		case planning.StatusDone:
			icon = "✓"
		case planning.StatusRunning:
			icon = "▶"
		case planning.StatusFailed:
			icon = "✗"
		case planning.StatusBlocked:
			icon = "⊘"
		case planning.StatusReady:
			icon = "◉"
		}

		statusColor := m.t.dim
		switch t.Status {
		case planning.StatusDone:
			statusColor = m.t.badge
		case planning.StatusFailed:
			statusColor = m.t.errM
		case planning.StatusRunning:
			statusColor = m.t.stat
		}

		b.WriteString(statusColor.Render(fmt.Sprintf("  %s %s: %s", icon, t.ID, t.Title)))
		if len(t.DependsOn) > 0 {
			b.WriteString(m.t.dim.Render(fmt.Sprintf(" (after %s)", strings.Join(t.DependsOn, ","))))
		}
		if t.Agent != "" {
			b.WriteString(m.t.dim.Render(fmt.Sprintf(" [%s]", t.Agent)))
		}
		b.WriteString("\n")
		if t.Result != "" {
			b.WriteString(m.t.dim.Render("    → " + truncateStr(t.Result, 80)))
			b.WriteString("\n")
		}
		if t.Error != "" {
			b.WriteString(m.t.errM.Render("    ✗ " + t.Error))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m *model) executePlan(p *planning.Plan) tea.Cmd {
	m.planPending = false
	m.addSys(fmt.Sprintf("Executing plan %s...", p.ID))
	m.statusText = "Planning..."
	m.streaming = true
	m.startTime = time.Now()
	m.vp.GotoBottom()

	go func() {
		for {
			task, ok := p.NextTask()
			if !ok {
				break
			}
			p.SetTaskStatus(task.ID, planning.StatusRunning, "", "")
			m.addSys(fmt.Sprintf("▶ Task %s: %s", task.ID, task.Title))
			m.addSys(m.renderPlanCard(p))
			m.vp.GotoBottom()

			prompt := task.Description
			if prompt == "" {
				prompt = task.Title
			}
			if len(task.Files) > 0 {
				prompt += "\n\nFiles: " + strings.Join(task.Files, ", ")
			}
			prompt += "\n\n" + m.renderPlanCard(p)

			_ = m.submit(prompt)
			_ = m.nextChunk()

			for m.streaming {
				time.Sleep(100 * time.Millisecond)
			}

			if m.lastError != nil {
				p.SetTaskStatus(task.ID, planning.StatusFailed, "", m.lastError.Error())
				m.addSys(fmt.Sprintf("✗ Task %s failed: %v", task.ID, m.lastError))
			} else {
				p.SetTaskStatus(task.ID, planning.StatusDone, "Completed", "")
				m.addSys(fmt.Sprintf("✓ Task %s done", task.ID))
			}
			m.addSys(m.renderPlanCard(p))
			m.vp.GotoBottom()
		}

		m.streaming = false
		m.statusText = "Ready"
		done, total := p.Progress()
		if done == total {
			m.addSys(fmt.Sprintf("Plan %s complete! (%d/%d tasks)", p.ID, done, total))
		} else {
			m.addSys(fmt.Sprintf("Plan %s finished with partial success (%d/%d tasks)", p.ID, done, total))
		}
		m.vp.GotoBottom()
	}()

	return m.tick()
}
