package continuation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestClassifyTransient(t *testing.T) {
	for _, msg := range []string{
		"dial tcp: connection refused",
		"connectex: no connection could be made",
		"no such host",
		"connection reset by peer",
		"unexpected HTTP status 503",
		"too many requests (429)",
	} {
		c := Classify(errors.New(msg))
		if c.Class != ClassTransientAPI || !c.Retryable {
			t.Fatalf("%q: got %s retryable=%v", msg, c.Class, c.Retryable)
		}
		if c.MaxAttempts < 2 || c.Backoff <= 0 {
			t.Fatalf("%q: bad policy %+v", msg, c)
		}
	}
}

func TestClassifyTimeout(t *testing.T) {
	c := Classify(errors.New("context deadline exceeded"))
	if c.Class != ClassModelTimeout || !c.Retryable {
		t.Fatalf("got %s", c.Class)
	}
}

func TestClassifyToolCrash(t *testing.T) {
	c := Classify(errors.New("dependency \"black\" not found"))
	if c.Class != ClassToolCrash || c.Retryable {
		t.Fatalf("got %s retryable=%v", c.Class, c.Retryable)
	}
}

func TestClassifyApproval(t *testing.T) {
	c := Classify(errors.New("tool \"terminal\" requires approval"))
	if c.Class != ClassApproval {
		t.Fatalf("got %s", c.Class)
	}
}

func TestClassifyEmptyOutput(t *testing.T) {
	c := Classify(errors.New("agent returned empty output"))
	if c.Class != ClassEmptyOutput || !c.Retryable {
		t.Fatalf("got %s", c.Class)
	}
}

func TestRetryPolicy(t *testing.T) {
	c := Classify(errors.New("dial tcp: connection refused"))
	p := PolicyFor(c)
	if !p.ShouldRetry(0) || p.ShouldRetry(2) {
		t.Fatal("retry limits wrong")
	}
	if p.WaitFor(0) != c.Backoff {
		t.Fatal("backoff base wrong")
	}
	if p.WaitFor(2) <= p.WaitFor(1) {
		t.Fatal("backoff not increasing")
	}
}

func TestPlanRecovery(t *testing.T) {
	r := PlanRecovery("t1", errors.New("dependency not found"), "cp-1", true)
	if !r.AddFixTask || !r.ContinueRemaining {
		t.Fatalf("tool crash recovery: %+v", r)
	}
	if r.CheckpointID != "cp-1" {
		t.Fatal("checkpoint not carried")
	}
	r2 := PlanRecovery("t2", errors.New("connection refused"), "", true)
	if !r2.RetryImmediately {
		t.Fatalf("transient recovery: %+v", r2)
	}
	r3 := PlanRecovery("t3", errors.New("requires approval"), "", false)
	if !r3.ContinueRemaining {
		t.Fatalf("approval recovery: %+v", r3)
	}
}

func TestGoalStatus(t *testing.T) {
	g := GoalStatus{Goal: "x", TotalTasks: 4, StartedAt: time.Now()}
	if g.Progress() != 0 {
		t.Fatal("expected 0 progress")
	}
	g.MarkTask(true)
	g.MarkTask(true)
	if g.Progress() != 0.5 {
		t.Fatalf("progress = %v", g.Progress())
	}
	g.MarkTask(false)
	if g.FailedTasks != 1 || g.CompletedTasks != 2 {
		t.Fatalf("counters: %+v", g)
	}
	if !strings.Contains(g.String(), "2/4") {
		t.Fatalf("string: %s", g.String())
	}
}
