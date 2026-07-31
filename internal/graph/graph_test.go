package graph

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSequentialDeps(t *testing.T) {
	g := New()
	var order []string
	var mu sync.Mutex
	g.Add(&Node{ID: "a", Title: "a", Run: func(ctx context.Context) (string, error) {
		mu.Lock()
		order = append(order, "a")
		mu.Unlock()
		return "a", nil
	}})
	g.Add(&Node{ID: "b", Title: "b", Dependencies: []string{"a"}, Run: func(ctx context.Context) (string, error) {
		mu.Lock()
		order = append(order, "b")
		mu.Unlock()
		return "b", nil
	}})
	sum := g.Run(context.Background(), 2)
	if sum.Done != 2 || sum.Failed != 0 {
		t.Fatalf("summary: %+v", sum)
	}
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Fatalf("order: %v", order)
	}
}

func TestParallelIndependent(t *testing.T) {
	g := New()
	var concurrent int32
	var maxSeen int32
	for _, id := range []string{"a", "b", "c", "d"} {
		id := id
		g.Add(&Node{ID: id, Run: func(ctx context.Context) (string, error) {
			n := atomic.AddInt32(&concurrent, 1)
			for {
				cur := atomic.LoadInt32(&maxSeen)
				if n <= cur || atomic.CompareAndSwapInt32(&maxSeen, cur, n) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&concurrent, -1)
			return id, nil
		}})
	}
	sum := g.Run(context.Background(), 4)
	if sum.Done != 4 {
		t.Fatalf("summary: %+v", sum)
	}
	if maxSeen < 2 {
		t.Fatalf("expected parallel execution, max concurrency %d", maxSeen)
	}
}

func TestFailedDependencyBlocks(t *testing.T) {
	g := New()
	g.Add(&Node{ID: "a", Run: func(ctx context.Context) (string, error) {
		return "", errors.New("boom")
	}})
	g.Add(&Node{ID: "b", Dependencies: []string{"a"}, Run: func(ctx context.Context) (string, error) {
		return "b", nil
	}})
	sum := g.Run(context.Background(), 2)
	g.MarkFailedDepsAsBlocked()
	if sum.Failed != 1 || sum.Skipped != 1 {
		t.Fatalf("summary: %+v", sum)
	}
	if n, _ := g.Get("b"); n.Status != "skipped" {
		t.Fatalf("b status: %s", n.Status)
	}
}

func TestChainedDeadlockSkips(t *testing.T) {
	g := New()
	g.Add(&Node{ID: "a", Run: func(ctx context.Context) (string, error) {
		return "", errors.New("boom")
	}})
	g.Add(&Node{ID: "b", Dependencies: []string{"a"}, Run: func(ctx context.Context) (string, error) {
		return "b", nil
	}})
	g.Add(&Node{ID: "c", Dependencies: []string{"b"}, Run: func(ctx context.Context) (string, error) {
		return "c", nil
	}})
	sum := g.Run(context.Background(), 2)
	if sum.Failed != 1 || sum.Skipped != 2 || sum.Done != 0 {
		t.Fatalf("summary: %+v", sum)
	}
}

func TestReprioritize(t *testing.T) {
	g := New()
	g.Add(&Node{ID: "low", Priority: 1, Run: func(ctx context.Context) (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "low", nil
	}})
	g.Add(&Node{ID: "high", Priority: 1, Run: func(ctx context.Context) (string, error) {
		return "high", nil
	}})
	g.Reprioritize("high")
	if n, _ := g.Get("high"); n.Priority != 1000 {
		t.Fatal("reprioritize failed")
	}
	sum := g.Run(context.Background(), 1)
	if sum.Done != 2 {
		t.Fatalf("summary: %+v", sum)
	}
}

func TestCancellation(t *testing.T) {
	g := New()
	g.Add(&Node{ID: "slow", Run: func(ctx context.Context) (string, error) {
		time.Sleep(5 * time.Second)
		return "done", nil
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	sum := g.Run(ctx, 1)
	if time.Since(start) > 2*time.Second {
		t.Fatal("cancellation too slow")
	}
	_ = sum
}

func TestStatusSnapshot(t *testing.T) {
	g := New()
	g.Add(&Node{ID: "a", Run: func(ctx context.Context) (string, error) { return "a", nil }})
	states := g.StatusSnapshot()
	if len(states) != 1 || states[0].ID != "a" || states[0].Status != "pending" {
		t.Fatalf("snapshot: %+v", states)
	}
}
