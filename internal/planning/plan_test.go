package planning

import (
	"errors"
	"fmt"
	"testing"
)

func TestPlanDependencies(t *testing.T) {
	p := NewPlan("build feature")
	p.AddTask(Task{ID: "1", Title: "setup", Agent: "Coder"})
	p.AddTask(Task{ID: "2", Title: "impl", DependsOn: []string{"1"}, Agent: "Coder"})
	p.AddTask(Task{ID: "3", Title: "test", DependsOn: []string{"2"}, Agent: "TestEngineer"})

	if got := len(p.ReadyTasks()); got != 1 {
		t.Fatalf("expected 1 ready task, got %d", got)
	}
	if nxt, ok := p.NextTask(); !ok || nxt.ID != "1" {
		t.Fatalf("expected task 1 next, got %v", t)
	}
	p.SetTaskStatus("1", StatusDone, "ok", "")
	if got := len(p.ReadyTasks()); got != 1 {
		t.Fatalf("expected 1 ready after first, got %d", got)
	}
	p.SetTaskStatus("2", StatusDone, "ok", "")
	if nxt, ok := p.NextTask(); !ok || nxt.ID != "3" {
		t.Fatalf("expected task 3 next, got %v", t)
	}
	p.SetTaskStatus("3", StatusDone, "ok", "")
	if !p.AllDone() {
		t.Fatal("expected all done")
	}
	if p.Status != StatusDone {
		t.Fatalf("expected plan done, got %s", p.Status)
	}
}

func TestPlanReplan(t *testing.T) {
	p := NewPlan("goal")
	p.AddTask(Task{ID: "1", Title: "a"})
	p.AddTask(Task{ID: "2", Title: "b", DependsOn: []string{"1"}})
	p.SetTaskStatus("1", StatusFailed, "", "boom")
	ids := p.Replan([]Task{{ID: "3", Title: "fix", DependsOn: []string{"1"}}})
	if len(ids) != 1 {
		t.Fatalf("expected 1 replan id, got %v", ids)
	}
	if _, ok := p.GetTask("3"); !ok {
		t.Fatal("replanned task missing")
	}
	if p.ReplanCount != 1 {
		t.Fatalf("expected ReplanCount 1, got %d", p.ReplanCount)
	}
}

func TestPlanEngineRunsAll(t *testing.T) {
	p := NewPlan("goal")
	p.AddTask(Task{ID: "1", Title: "a"})
	p.AddTask(Task{ID: "2", Title: "b", DependsOn: []string{"1"}})
	exec := func(t Task) (string, error) {
		return fmt.Sprintf("ran %s", t.ID), nil
	}
	e := NewEngine(p, exec)
	if err := e.Run(); err != nil {
		t.Fatalf("engine run: %v", err)
	}
	if !p.AllDone() {
		t.Fatal("expected all tasks done")
	}
}

func TestPlanEngineFails(t *testing.T) {
	p := NewPlan("goal")
	p.AddTask(Task{ID: "1", Title: "a"})
	p.AddTask(Task{ID: "2", Title: "b", DependsOn: []string{"1"}})
	exec := func(t Task) (string, error) {
		if t.ID == "1" {
			return "", errors.New("failed")
		}
		return "ok", nil
	}
	e := NewEngine(p, exec)
	if err := e.Run(); err == nil {
		t.Fatal("expected engine error")
	}
	if p.Status != StatusFailed {
		t.Fatalf("expected failed plan, got %s", p.Status)
	}
	if t2, _ := p.GetTask("2"); t2.Status != StatusBlocked {
		t.Fatalf("expected task 2 blocked, got %s", t2.Status)
	}
}
