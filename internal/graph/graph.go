package graph

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Node is a unit of work with dependencies.
type Node struct {
	ID           string
	Title        string
	Dependencies []string
	Priority     int // higher = sooner when many tasks are ready
	Run          func(ctx context.Context) (string, error)
	Status       string // pending | running | done | failed | skipped
	Result       string
	Error        error
	StartedAt    time.Time
	FinishedAt   time.Time
}

// Graph is a dependency-ordered task graph with a worker pool.
type Graph struct {
	mu    sync.Mutex
	nodes map[string]*Node
	order []string
	exec  chan *Node
	done  chan struct{}
}

// New creates an empty task graph.
func New() *Graph {
	return &Graph{
		nodes: map[string]*Node{},
		done:  make(chan struct{}),
	}
}

// Add registers a node.
func (g *Graph) Add(n *Node) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n.Status == "" {
		n.Status = "pending"
	}
	g.nodes[n.ID] = n
	g.order = append(g.order, n.ID)
}

// Get returns a node by ID.
func (g *Graph) Get(id string) (*Node, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	n, ok := g.nodes[id]
	return n, ok
}

// SetPriority reprioritizes a pending node.
func (g *Graph) SetPriority(id string, priority int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n, ok := g.nodes[id]; ok && n.Status == "pending" {
		n.Priority = priority
	}
}

// Reprioritize boosts a task above all others.
func (g *Graph) Reprioritize(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, n := range g.nodes {
		if n.ID == id {
			n.Priority = 1000
			continue
		}
		if n.Priority > 0 && n.Priority < 1000 {
			n.Priority--
		}
	}
}

// readyNode returns the highest-priority pending node whose deps are all done.
func (g *Graph) readyNode() *Node {
	g.mu.Lock()
	defer g.mu.Unlock()
	var best *Node
	for _, id := range g.order {
		n := g.nodes[id]
		if n.Status != "pending" {
			continue
		}
		if !g.depsDoneLocked(n) {
			continue
		}
		if best == nil || n.Priority > best.Priority {
			best = n
		}
	}
	if best != nil {
		best.Status = "running"
	}
	return best
}

func (g *Graph) depsDoneLocked(n *Node) bool {
	for _, dep := range n.Dependencies {
		if d, ok := g.nodes[dep]; ok && d.Status != "done" {
			return false
		}
	}
	return true
}

// Completed reports whether every node reached a terminal state.
func (g *Graph) Completed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.completedLocked()
}

func (g *Graph) completedLocked() bool {
	for _, n := range g.nodes {
		if n.Status != "done" && n.Status != "failed" && n.Status != "skipped" {
			return false
		}
	}
	return true
}

// MarkFailedDepsAsBlocked flags pending nodes whose deps failed.
func (g *Graph) MarkFailedDepsAsBlocked() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.skipBlockedLocked()
}

// skipBlockedLocked marks pending nodes whose dependencies can never succeed
// (failed or already skipped) as skipped so the graph cannot deadlock.
func (g *Graph) skipBlockedLocked() {
	for _, n := range g.nodes {
		if n.Status != "pending" {
			continue
		}
		for _, dep := range n.Dependencies {
			if d, ok := g.nodes[dep]; ok && (d.Status == "failed" || d.Status == "skipped") {
				n.Status = "skipped"
				n.Error = fmt.Errorf("dependency %s failed", dep)
				break
			}
		}
	}
}

// RunSummary aggregates execution outcomes.
type RunSummary struct {
	Total    int
	Done     int
	Failed   int
	Skipped  int
	Elapsed  time.Duration
	Attempts int
}

// Run executes the graph with `workers` goroutines until completion or ctx cancel.
func (g *Graph) Run(ctx context.Context, workers int) RunSummary {
	if workers < 1 {
		workers = 1
	}
	start := time.Now()
	var sum RunSummary
	g.mu.Lock()
	g.exec = make(chan *Node, workers*2)
	g.done = make(chan struct{})
	done := g.done
	g.mu.Unlock()

	for i := 0; i < workers; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case n, ok := <-g.exec:
					if !ok {
						return
					}
					out, err := n.Run(ctx)
					g.mu.Lock()
					n.Result = out
					n.Error = err
					n.FinishedAt = time.Now()
					if err != nil {
						n.Status = "failed"
					} else {
						n.Status = "done"
					}
					g.mu.Unlock()
				}
			}
		}()
	}

	// Scheduler: dispatch ready nodes to the pool.
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(done)
				return
			default:
			}
			g.mu.Lock()
			complete := g.completedLocked()
			if !complete {
				g.skipBlockedLocked()
			}
			g.mu.Unlock()
			if complete {
				close(done)
				return
			}
			n := g.readyNode()
			if n == nil {
				time.Sleep(15 * time.Millisecond)
				continue
			}
			n.StartedAt = time.Now()
			select {
			case g.exec <- n:
			case <-ctx.Done():
				close(done)
				return
			}
		}
	}()

	<-done
	g.mu.Lock()
	for _, n := range g.nodes {
		sum.Total++
		switch n.Status {
		case "failed":
			sum.Failed++
		case "skipped":
			sum.Skipped++
		case "done":
			sum.Done++
		}
		if !n.StartedAt.IsZero() {
			sum.Attempts++
		}
	}
	sum.Elapsed = time.Since(start)
	g.mu.Unlock()
	return sum
}

// Summary aggregates current state.
func (g *Graph) Summary() RunSummary {
	g.mu.Lock()
	defer g.mu.Unlock()
	var s RunSummary
	for _, n := range g.nodes {
		s.Total++
		switch n.Status {
		case "done":
			s.Done++
		case "failed":
			s.Failed++
		case "skipped":
			s.Skipped++
		}
	}
	return s
}

// Remaining returns pending node IDs.
func (g *Graph) Remaining() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []string
	for _, id := range g.order {
		if g.nodes[id].Status == "pending" {
			out = append(out, id)
		}
	}
	return out
}

// NodeState is a thread-safe snapshot of a node.
type NodeState struct {
	ID       string
	Title    string
	Status   string
	Priority int
}

// StatusSnapshot returns a thread-safe list of node states.
func (g *Graph) StatusSnapshot() []NodeState {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]NodeState, 0, len(g.nodes))
	for _, id := range g.order {
		n := g.nodes[id]
		out = append(out, NodeState{ID: n.ID, Title: n.Title, Status: n.Status, Priority: n.Priority})
	}
	return out
}
