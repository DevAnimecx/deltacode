package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Job is a background execution result.
type Job struct {
	ID       string
	Name     string
	Started  time.Time
	Finished time.Time
	ExitCode int
	Output   string
	Running  bool
	Done     bool
}

type jobStore struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

var jobs = &jobStore{jobs: map[string]*Job{}}

func (r *Registry) execute(t *Tool, opts CallOptions, args ...string) (string, error) {
	// Permission gate.
	if t.NeedsApproval(r.policy) {
		if opts.Approver != nil {
			if !opts.Approver(t, args) {
				return "", fmt.Errorf("tool %q not approved", t.Name())
			}
		} else {
			// No approver: default to ask once via stored decisions.
			if !r.approvalDecided(t) {
				return "", fmt.Errorf("tool %q requires approval; use delta policy tool-allow %s", t.Name(), t.Name())
			}
		}
	}

	timeoutSec := t.Manifest.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	if opts.TimeoutSec > 0 {
		timeoutSec = opts.TimeoutSec
	}
	retries := t.Manifest.Retry
	if opts.Retries > 0 {
		retries = opts.Retries
	}
	if retries < 0 {
		retries = 0
	}

	// Background execution.
	if opts.Background {
		jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
		job := &Job{ID: jobID, Name: t.Name(), Started: time.Now(), Running: true}
		jobs.mu.Lock()
		jobs.jobs[jobID] = job
		jobs.mu.Unlock()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
			defer cancel()
			out, err := runWithTimeout(t.Run, ctx, args...)
			job.Finished = time.Now()
			job.Running = false
			job.Done = true
			job.Output = out
			if err != nil {
				job.ExitCode = 1
			}
		}()
		return fmt.Sprintf("started tool %s in background (%s)", t.Name(), jobID), nil
	}

	start := time.Now()
	var lastErr error
	var lastOut string
	for attempt := 0; attempt <= retries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		out, err := runWithTimeout(t.Run, ctx, args...)
		cancel()
		duration := float64(time.Since(start).Milliseconds())
		lastOut, lastErr = out, err

		// Record stats + learning data.
		t.Stats.Record(duration, err == nil, args)
		r.recordLearning(t, args, duration, err)

		if err == nil {
			r.saveStats()
			return out, nil
		}
		// Don't retry on approval/policy errors.
		if isFatalToolError(err) {
			break
		}
		if attempt < retries {
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
		}
	}
	r.saveStats()
	return lastOut, lastErr
}

// approvalDecided consults persisted per-tool approval decisions
// (persisted at ~/.delta/tool-approvals.json).
func (r *Registry) approvalDecided(t *Tool) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.approvals == nil {
		return false
	}
	return r.approvals.Approved[t.Name()]
}

// ApprovalsStore persists per-tool allow/deny decisions.
type approvalsStore struct {
	mu       sync.Mutex
	file     string
	Approved map[string]bool `json:"approved"`
	Denied   map[string]bool `json:"denied"`
}

func newApprovalsStore() *approvalsStore {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	s := &approvalsStore{
		file:     filepath.Join(home, ".delta", "tool-approvals.json"),
		Approved: map[string]bool{},
		Denied:   map[string]bool{},
	}
	if data, err := os.ReadFile(s.file); err == nil {
		_ = json.Unmarshal(data, s)
	}
	return s
}

func (s *approvalsStore) allow(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Approved[name] = true
	delete(s.Denied, name)
	s.save()
}

func (s *approvalsStore) deny(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Denied[name] = true
	delete(s.Approved, name)
	s.save()
}

func (s *approvalsStore) save() {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.file, data, 0600)
}

// SetToolAllowed marks a tool as approved for future runs.
func (r *Registry) SetToolAllowed(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.approvals == nil {
		r.approvals = newApprovalsStore()
	}
	r.approvals.allow(name)
}

// SetToolDenied marks a tool as denied for future runs.
func (r *Registry) SetToolDenied(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.approvals == nil {
		r.approvals = newApprovalsStore()
	}
	r.approvals.deny(name)
}

func (r *Registry) recordLearning(t *Tool, args []string, duration float64, err error) {
	if err != nil {
		t.Stats.RecoverySteps = append(t.Stats.RecoverySteps, fmt.Sprintf("%s failed with %v", t.Name(), err))
		if len(t.Stats.RecoverySteps) > 5 {
			t.Stats.RecoverySteps = t.Stats.RecoverySteps[len(t.Stats.RecoverySteps)-5:]
		}
		return
	}
	if t.Stats.Patterns == nil && len(args) > 0 {
		t.Stats.Patterns = append(t.Stats.Patterns, strings.Join(args, " "))
	}
}

func runWithTimeout(fn ToolFunc, ctx context.Context, args ...string) (string, error) {
	done := make(chan struct{})
	var out string
	var err error
	go func() {
		out, err = fn(args...)
		close(done)
	}()
	select {
	case <-done:
		return out, err
	case <-ctx.Done():
		return "", fmt.Errorf("tool timed out (%s)", ctx.Err())
	}
}

func isFatalToolError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "not approved") ||
		strings.Contains(msg, "requires approval") ||
		strings.Contains(msg, "denied")
}

// runExternal executes an external command with a timeout, capturing exit code.
func runExternal(bin string, args []string, timeoutSec int) (string, error) {
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("dependency %q not found", bin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	err = cmd.Run()
	result := strings.TrimSpace(out.String())
	if errBuf.Len() > 0 {
		msg := strings.TrimSpace(errBuf.String())
		if result == "" {
			result = msg
		} else {
			result += "\n" + msg
		}
	}
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("command timed out after %ds", timeoutSec)
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			return result, fmt.Errorf("exit code %d: %s", exitErr.ExitCode(), truncateErr(result))
		}
		return result, err
	}
	return result, nil
}

func truncateErr(s string) string {
	if len(s) <= 300 {
		return s
	}
	return s[:300] + "..."
}

// ListJobs returns running/finished background jobs.
func ListJobs() []*Job {
	jobs.mu.Lock()
	defer jobs.mu.Unlock()
	var out []*Job
	for _, j := range jobs.jobs {
		out = append(out, j)
	}
	return out
}
