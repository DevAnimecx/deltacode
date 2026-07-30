package tasks

import (
	"fmt"
	"sync"
	"time"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

type Task struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Status    TaskStatus `json:"status"`
	Progress  float64    `json:"progress"`
	CreatedAt time.Time  `json:"created_at"`
	Result    string     `json:"result"`
	Error     string     `json:"error,omitempty"`
}

type Manager struct {
	mu    sync.Mutex
	tasks map[string]*Task
}

func NewManager() *Manager {
	return &Manager{
		tasks: make(map[string]*Task),
	}
}

func (m *Manager) Create(name string) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := fmt.Sprintf("task-%d", time.Now().UnixNano())
	task := &Task{
		ID:        id,
		Name:      name,
		Status:    StatusPending,
		Progress:  0,
		CreatedAt: time.Now(),
	}
	m.tasks[id] = task
	return task
}

func (m *Manager) Start(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		t.Status = StatusRunning
	}
}

func (m *Manager) UpdateProgress(id string, pct float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		t.Progress = pct
	}
}

func (m *Manager) Complete(id, result string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		t.Status = StatusCompleted
		t.Result = result
		t.Progress = 100
	}
}

func (m *Manager) Fail(id, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.tasks[id]; ok {
		t.Status = StatusFailed
		t.Error = errMsg
	}
}

func (m *Manager) Get(id string) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tasks[id]
}

func (m *Manager) List() []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	var list []*Task
	for _, t := range m.tasks {
		list = append(list, t)
	}
	return list
}
