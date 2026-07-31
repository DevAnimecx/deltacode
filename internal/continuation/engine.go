package continuation

import (
	"fmt"
	"strings"
	"time"
)

// ErrorClass categorizes failures so the engine can pick a recovery path.
type ErrorClass string

const (
	ClassTransientAPI     ErrorClass = "transient_api"       // network blips, 5xx, rate limits
	ClassModelTimeout     ErrorClass = "model_timeout"        // provider/model took too long
	ClassToolCrash        ErrorClass = "tool_crash"           // tool dependency missing/crashed
	ClassValidation       ErrorClass = "validation_failure"   // build/lint/test/security failed
	ClassApproval         ErrorClass = "approval_required"    // needs user decision
	ClassPolicy           ErrorClass = "policy_denied"        // denied by policy
	ClassEmptyOutput      ErrorClass = "empty_output"         // model returned nothing usable
	ClassFatal            ErrorClass = "fatal"                // unrecoverable, stop
)

// Classified describes an error and its recovery options.
type Classified struct {
	Class     ErrorClass
	Message   string
	Retryable bool
	// Suggested strategy text for logs / UI.
	Strategy string
	// Backoff applied before the next attempt.
	Backoff time.Duration
	// MaxAttempts allowed for this class.
	MaxAttempts int
}

// Classify maps an error to a recovery policy.
func Classify(err error) Classified {
	if err == nil {
		return Classified{Class: ClassFatal, Retryable: false, MaxAttempts: 1}
	}
	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "connectex") ||
		strings.Contains(msg, "temporary failure") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "5"):
		if strings.Contains(msg, "500") || strings.Contains(msg, "502") || strings.Contains(msg, "503") || strings.Contains(msg, "504") {
			return Classified{
				Class: ClassTransientAPI, Message: err.Error(), Retryable: true,
				Strategy: "transient provider failure — retry with backoff, then fall back to another provider",
				Backoff: 1 * time.Second, MaxAttempts: 3,
			}
		}
		return Classified{
			Class: ClassTransientAPI, Message: err.Error(), Retryable: true,
			Strategy: "connectivity failure — retry after backoff, then alternate provider",
			Backoff: 2 * time.Second, MaxAttempts: 3,
		}

	case strings.Contains(msg, "timed out") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded"):
		return Classified{
			Class: ClassModelTimeout, Message: err.Error(), Retryable: true,
			Strategy: "model timeout — retry once with a faster model, then split the task",
			Backoff: 500 * time.Millisecond, MaxAttempts: 2,
		}

	case strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "no such tool") ||
		strings.Contains(msg, "dependency") ||
		strings.Contains(msg, "exit code"):
		return Classified{
			Class: ClassToolCrash, Message: err.Error(), Retryable: false,
			Strategy: "tool crashed or missing dependency — install missing tool or use an alternative tool",
			Backoff: 0, MaxAttempts: 1,
		}

	case strings.Contains(msg, "requires approval") ||
		strings.Contains(msg, "not approved"):
		return Classified{
			Class: ClassApproval, Message: err.Error(), Retryable: false,
			Strategy: "approval required — ask the user via delta policy tool-allow",
			Backoff: 0, MaxAttempts: 1,
		}

	case strings.Contains(msg, "denied") ||
		strings.Contains(msg, "not in allowed"):
		return Classified{
			Class: ClassPolicy, Message: err.Error(), Retryable: false,
			Strategy: "policy denied — user must adjust policy",
			Backoff: 0, MaxAttempts: 1,
		}

	case strings.Contains(msg, "empty output") ||
		strings.Contains(msg, "no code produced"):
		return Classified{
			Class: ClassEmptyOutput, Message: err.Error(), Retryable: true,
			Strategy: "model returned empty output — retry with more explicit instructions",
			Backoff: 300 * time.Millisecond, MaxAttempts: 2,
		}

	case strings.Contains(msg, "validation") ||
		strings.Contains(msg, "failed checks"):
		return Classified{
			Class: ClassValidation, Message: err.Error(), Retryable: true,
			Strategy: "validation failed — feed errors back to the agent and re-run",
			Backoff: 500 * time.Millisecond, MaxAttempts: 2,
		}

	default:
		return Classified{
			Class: ClassFatal, Message: err.Error(), Retryable: false,
			Strategy: "unclassified error — record and continue with remaining tasks",
			Backoff: 0, MaxAttempts: 1,
		}
	}
}

// RetryPolicy decides whether and how to retry based on attempts so far.
type RetryPolicy struct {
	Enabled    bool
	MaxAttempts int
	Backoff    time.Duration
	Multiply   time.Duration // exponential factor
}

// PolicyFor returns the retry policy for a class.
func PolicyFor(c Classified) RetryPolicy {
	return RetryPolicy{
		Enabled: c.Retryable, MaxAttempts: c.MaxAttempts,
		Backoff: c.Backoff, Multiply: 2 * time.Second,
	}
}

// ShouldRetry reports whether attempt (0-based) may be retried.
func (p RetryPolicy) ShouldRetry(attempt int) bool {
	return p.Enabled && attempt+1 < p.MaxAttempts
}

// WaitFor computes the backoff before retry `attempt` (0-based).
func (p RetryPolicy) WaitFor(attempt int) time.Duration {
	if attempt <= 0 {
		return p.Backoff
	}
	return p.Backoff + time.Duration(attempt)*p.Multiply
}

// Recovery is a set of actions the engine can take after a failure.
type Recovery struct {
	TaskID            string
	Error             error
	Class             ErrorClass
	RetryImmediately  bool
	AlternativeTool   string   // tool to try instead
	SplitTask         bool     // recommend splitting the task
	AddFixTask        bool     // append a fix task to the plan
	ContinueRemaining bool     // skip this task, keep going
	CheckpointID      string   // resume point if available
	Advice            []string // human-readable suggestions
}

// PlanRecovery builds a recovery plan for a failed task.
func PlanRecovery(taskID string, err error, checkpointID string, ready bool) Recovery {
	cls := Classify(err)
	r := Recovery{
		TaskID:       taskID,
		Error:        err,
		Class:        cls.Class,
		CheckpointID: checkpointID,
		Advice:       []string{cls.Strategy},
	}
	switch cls.Class {
	case ClassTransientAPI:
		r.RetryImmediately = true
	case ClassModelTimeout:
		r.RetryImmediately = true
		r.SplitTask = true
		r.Advice = append(r.Advice, "reduce the task scope on retry")
	case ClassToolCrash:
		r.AddFixTask = true
		r.AlternativeTool = suggestAlternative(err)
		r.ContinueRemaining = true
	case ClassEmptyOutput:
		r.RetryImmediately = true
		r.Advice = append(r.Advice, "add explicit file/format instructions")
	case ClassValidation:
		r.AddFixTask = true
		r.Advice = append(r.Advice, "feed validation errors to the Debugger agent")
	case ClassApproval, ClassPolicy:
		r.ContinueRemaining = true
	case ClassFatal:
		r.ContinueRemaining = true
	}
	return r
}

// suggestAlternative maps a missing tool to a fallback.
func suggestAlternative(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "black"):
		return "format"
	case strings.Contains(msg, "gofmt"), strings.Contains(msg, "go vet"):
		return "terminal"
	case strings.Contains(msg, "sqlite"):
		return "db"
	case strings.Contains(msg, "docker"):
		return "terminal"
	case strings.Contains(msg, "playwright"):
		return "websearch"
	}
	return ""
}

// GoalStatus tracks goal-oriented (not step-oriented) execution.
type GoalStatus struct {
	Goal           string
	TotalTasks     int
	CompletedTasks int
	FailedTasks    int
	SkippedTasks   int
	Attempts       int
	Retries        int
	Recoveries     int
	Partial        bool // goal finished with some tasks failed/skipped
	StartedAt      time.Time
}

// Progress returns completion ratio 0..1.
func (g *GoalStatus) Progress() float64 {
	if g.TotalTasks == 0 {
		return 0
	}
	return float64(g.CompletedTasks) / float64(g.TotalTasks)
}

// ETA estimates remaining time based on elapsed wall time and progress.
func (g *GoalStatus) ETA() time.Duration {
	elapsed := time.Since(g.StartedAt)
	progress := g.Progress()
	if progress <= 0 {
		return 0
	}
	return time.Duration(float64(elapsed) / progress * (1 - progress))
}

func (g *GoalStatus) String() string {
	return fmt.Sprintf("%d/%d tasks, %d failed, %d skipped, %d retries, partial=%v",
		g.CompletedTasks, g.TotalTasks, g.FailedTasks, g.SkippedTasks, g.Retries, g.Partial)
}

// MarkTask records a task outcome.
func (g *GoalStatus) MarkTask(completed bool) {
	if completed {
		g.CompletedTasks++
	} else {
		g.FailedTasks++
	}
}
