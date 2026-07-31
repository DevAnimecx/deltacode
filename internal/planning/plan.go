package planning

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusReady     TaskStatus = "ready"
	StatusRunning   TaskStatus = "running"
	StatusDone      TaskStatus = "done"
	StatusFailed    TaskStatus = "failed"
	StatusBlocked   TaskStatus = "blocked"
	StatusCancelled TaskStatus = "cancelled"
)

type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DependsOn   []string   `json:"depends_on"`
	Status      TaskStatus `json:"status"`
	Agent       string     `json:"agent"`
	Files       []string   `json:"files"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	Attempts    int        `json:"attempts"`
	Validation  string     `json:"validation,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   time.Time  `json:"started_at,omitempty"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
}

type Plan struct {
	ID          string     `json:"id"`
	Goal        string     `json:"goal"`
	Tasks       []Task     `json:"tasks"`
	Status      TaskStatus `json:"status"`
	CurrentTask string     `json:"current_task,omitempty"`
	ReplanCount int        `json:"replan_count"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
	mu          sync.Mutex `json:"-"`
}

func NewPlan(goal string) *Plan {
	now := time.Now()
	return &Plan{
		ID:        fmt.Sprintf("plan-%d", now.UnixNano()),
		Goal:      goal,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (p *Plan) AddTask(t Task) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if t.ID == "" {
		t.ID = fmt.Sprintf("%d", len(p.Tasks)+1)
	}
	if t.Status == "" {
		t.Status = StatusPending
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	p.Tasks = append(p.Tasks, t)
	p.UpdatedAt = time.Now()
}

func (p *Plan) GetTask(id string) (Task, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, t := range p.Tasks {
		if t.ID == id {
			return t, true
		}
	}
	return Task{}, false
}

func (p *Plan) SetTaskStatus(id string, status TaskStatus, result, errMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Tasks {
		if p.Tasks[i].ID == id {
			p.Tasks[i].Status = status
			if status == StatusRunning && p.Tasks[i].StartedAt.IsZero() {
				p.Tasks[i].StartedAt = time.Now()
			}
			if status == StatusDone {
				p.Tasks[i].Result = result
				p.Tasks[i].CompletedAt = time.Now()
				p.Tasks[i].Attempts++
			}
			if status == StatusFailed {
				p.Tasks[i].Error = errMsg
				p.Tasks[i].Attempts++
			}
			p.Tasks[i].Validation = errMsg
			break
		}
	}
	p.UpdatedAt = time.Now()
	p.recomputeStatusLocked()
}

func (p *Plan) recomputeStatusLocked() {
	if p.Status == StatusCancelled {
		return
	}
	all := len(p.Tasks)
	if all == 0 {
		return
	}
	done := 0
	failed := 0
	for _, t := range p.Tasks {
		switch t.Status {
		case StatusDone:
			done++
		case StatusFailed:
			failed++
		}
	}
	if done == all {
		p.Status = StatusDone
		p.CompletedAt = time.Now()
		return
	}
	if failed > 0 {
		p.Status = StatusFailed
		return
	}
	p.Status = StatusRunning
}

// ReadyTasks returns tasks whose dependencies are done and are not started.
func (p *Plan) ReadyTasks() []Task {
	p.mu.Lock()
	defer p.mu.Unlock()
	var ready []Task
	for _, t := range p.Tasks {
		if t.Status != StatusPending {
			continue
		}
		depsOk := true
		for _, dep := range t.DependsOn {
			found := false
			for _, d := range p.Tasks {
				if d.ID == dep {
					found = true
					if d.Status != StatusDone {
						depsOk = false
					}
					break
				}
			}
			if !found {
				depsOk = false
			}
		}
		if depsOk {
			ready = append(ready, t)
		}
	}
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].ID < ready[j].ID
	})
	return ready
}

// NextTask picks the next task to run (smallest ID among ready).
func (p *Plan) NextTask() (Task, bool) {
	ready := p.ReadyTasks()
	if len(ready) == 0 {
		return Task{}, false
	}
	best := ready[0]
	for _, t := range ready[1:] {
		if t.ID < best.ID {
			best = t
		}
	}
	return best, true
}

// Progress returns (done, total).
func (p *Plan) Progress() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	done, total := 0, len(p.Tasks)
	for _, t := range p.Tasks {
		if t.Status == StatusDone {
			done++
		}
	}
	return done, total
}

func (p *Plan) AllDone() bool {
	done, total := p.Progress()
	return total > 0 && done == total
}

func (p *Plan) MarkFailedTasksBlocked() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.Tasks {
		if p.Tasks[i].Status == StatusPending {
			blocked := false
			for _, dep := range p.Tasks[i].DependsOn {
				for _, d := range p.Tasks {
					if d.ID == dep && d.Status == StatusFailed {
						blocked = true
					}
				}
			}
			if blocked {
				p.Tasks[i].Status = StatusBlocked
			}
		}
	}
}

// Replan adds new tasks after a failed task; returns the new task IDs.
func (p *Plan) Replan(newTasks []Task) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ReplanCount++
	var ids []string
	for _, nt := range newTasks {
		if nt.ID == "" {
			nt.ID = fmt.Sprintf("%d", len(p.Tasks)+1)
		}
		nt.Status = StatusPending
		nt.CreatedAt = time.Now()
		p.Tasks = append(p.Tasks, nt)
		ids = append(ids, nt.ID)
	}
	p.UpdatedAt = time.Now()
	return ids
}

func (p *Plan) String() string {
	var b strings.Builder
	done, total := p.Progress()
	fmt.Fprintf(&b, "Plan %s [%s] (%d/%d done, replans=%d)\n", p.ID, p.Status, done, total, p.ReplanCount)
	for _, t := range p.Tasks {
		icon := "□"
		switch t.Status {
		case StatusDone:
			icon = "✓"
		case StatusRunning:
			icon = "▶"
		case StatusFailed:
			icon = "✗"
		case StatusBlocked:
			icon = "⊘"
		case StatusReady:
			icon = "◉"
		}
		fmt.Fprintf(&b, "  %s %s: %s", icon, t.ID, t.Title)
		if len(t.DependsOn) > 0 {
			fmt.Fprintf(&b, " (after %s)", strings.Join(t.DependsOn, ","))
		}
		if t.Agent != "" {
			fmt.Fprintf(&b, " [%s]", t.Agent)
		}
		if t.Attempts > 1 {
			fmt.Fprintf(&b, " (%d attempts)", t.Attempts)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Engine drives plan execution with an executor callback.
type Engine struct {
	Plan *Plan
	mu   sync.Mutex
	exec func(Task) (string, error)
}

func NewEngine(plan *Plan, exec func(Task) (string, error)) *Engine {
	return &Engine{Plan: plan, exec: exec}
}

func (e *Engine) Run() error {
	for {
		t, ok := e.Plan.NextTask()
		if !ok {
			break
		}
		e.Plan.SetTaskStatus(t.ID, StatusRunning, "", "")
		result, err := e.exec(t)
		if err != nil {
			e.Plan.SetTaskStatus(t.ID, StatusFailed, result, err.Error())
			e.Plan.MarkFailedTasksBlocked()
			return err
		}
		e.Plan.SetTaskStatus(t.ID, StatusDone, result, "")
	}
	return nil
}
