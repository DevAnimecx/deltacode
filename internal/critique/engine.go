package critique

import (
	"fmt"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/pkg/models"
)

type Aspect string

const (
	AspectTechnical    Aspect = "technical"
	AspectArchitecture Aspect = "architecture"
	AspectPerformance  Aspect = "performance"
	AspectSecurity     Aspect = "security"
	AspectReadability  Aspect = "readability"
	AspectConsistency  Aspect = "consistency"
)

type Issue struct {
	Aspect   Aspect `json:"aspect"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
}

type Review struct {
	Aspect   Aspect        `json:"aspect"`
	Score    float64       `json:"score"`
	Passed   bool          `json:"passed"`
	Issues   []Issue       `json:"issues"`
	Summary  string        `json:"summary"`
	Duration time.Duration `json:"duration_ms"`
}

type Result struct {
	Reviews      []Review `json:"reviews"`
	OverallScore float64  `json:"overall_score"`
	Passed       bool     `json:"passed"`
}

type Engine struct {
	provider interface {
		Chat(req models.ChatRequest) (*models.ChatResponse, error)
	}
	model     string
	aspects   []Aspect
	threshold float64
}

func New(provider interface {
	Chat(req models.ChatRequest) (*models.ChatResponse, error)
}, model string) *Engine {
	return &Engine{
		provider:  provider,
		model:     model,
		aspects:   []Aspect{AspectTechnical, AspectArchitecture, AspectPerformance, AspectSecurity, AspectReadability, AspectConsistency},
		threshold: 70,
	}
}

func (e *Engine) SetThreshold(t float64) { e.threshold = t }

func (e *Engine) Aspects() []Aspect { return e.aspects }

func aspectPrompt(a Aspect) string {
	switch a {
	case AspectTechnical:
		return "Review for technical correctness: bugs, race conditions, logic errors, error handling. Score 0-100."
	case AspectArchitecture:
		return "Review the architecture: separation of concerns, coupling, cohesion, module boundaries, extensibility. Score 0-100."
	case AspectPerformance:
		return "Review for performance: algorithmic complexity, allocations, I/O, caching, concurrency bottlenecks. Score 0-100."
	case AspectSecurity:
		return "Review for security: injection, XSS, CSRF, auth, secrets, unsafe deserialization, path traversal. Score 0-100."
	case AspectReadability:
		return "Review for readability: naming, structure, comments, complexity, maintainability. Score 0-100."
	case AspectConsistency:
		return "Review for consistency: code style, naming conventions, patterns matching the existing codebase. Score 0-100."
	}
	return "Review the code quality. Score 0-100."
}

// ReviewCode runs all critique aspects against the given code/files.
func (e *Engine) ReviewCode(goal, code string, files []string, repoInfo string) (*Result, error) {
	result := &Result{}
	var total float64
	start := time.Now()

	fileCtx := ""
	if len(files) > 0 {
		fileCtx = "\nFiles changed: " + strings.Join(files, ", ")
	}

	for _, aspect := range e.aspects {
		review, err := e.reviewAspect(aspect, goal, code, repoInfo, fileCtx)
		if err != nil {
			return nil, err
		}
		result.Reviews = append(result.Reviews, *review)
		total += review.Score
	}

	result.OverallScore = total / float64(len(e.aspects))
	result.Passed = result.OverallScore >= e.threshold
	for _, r := range result.Reviews {
		if !r.Passed {
			result.Passed = false
		}
	}
	result.OverallScore = total / float64(len(e.aspects))
	_ = start
	return result, nil
}

func (e *Engine) reviewAspect(aspect Aspect, goal, code, repoInfo, fileCtx string) (*Review, error) {
	review := &Review{Aspect: aspect}
	rstart := time.Now()
	system := fmt.Sprintf(`You are a strict senior staff engineer conducting a %s review. %s

Return your review as plain text with:
- A "Score: N" line at the end (0-100, where >=85 is excellent, >=70 acceptable, <70 needs work).
- Numbered issues, each with severity: [critical] [warning] [info].
- Concrete fix suggestions.`, aspect, aspectPrompt(aspect))

	user := fmt.Sprintf("Goal: %s\n%s\n\nCode under review:\n%s\n\nRepo context:\n%s",
		goal, fileCtx, truncate(code, 12000), truncate(repoInfo, 3000))

	resp, err := e.provider.Chat(models.ChatRequest{
		Model: e.model,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: system},
			{Role: models.RoleUser, Content: user},
		},
		Temperature: 0.1,
		MaxTokens:   4096,
	})
	if err != nil {
		return nil, err
	}

	output := resp.Message.Content
	review.Summary = output
	review.Score = parseScore(output)
	review.Passed = review.Score >= e.threshold
	review.Duration = time.Since(rstart)
	review.Issues = parseIssues(output, aspect)
	return review, nil
}

// Fix implements the auto-iterate loop: below threshold, ask the debugger agent for a fix.
type Fixer func(code, reviewText string) (string, error)

// IterateUntilPass retries the fix loop until reviews pass or maxIterations is hit.
func (e *Engine) IterateUntilPass(goal, code string, files []string, repoInfo string, fix Fixer, maxIterations int) (*Result, string, error) {
	current := code
	var lastRes *Result
	var err error
	for i := 0; i < maxIterations; i++ {
		lastRes, err = e.ReviewCode(goal, current, files, repoInfo)
		if err != nil {
			return nil, current, err
		}
		if lastRes.Passed {
			return lastRes, current, nil
		}
		if fix == nil {
			break
		}
		var reviewText strings.Builder
		for _, r := range lastRes.Reviews {
			fmt.Fprintf(&reviewText, "[%s] score=%0.0f\n%s\n", r.Aspect, r.Score, truncate(r.Summary, 1500))
		}
		fixed, ferr := fix(current, reviewText.String())
		if ferr != nil {
			return lastRes, current, ferr
		}
		if fixed == current {
			break
		}
		current = fixed
	}
	return lastRes, current, nil
}

func parseScore(output string) float64 {
	// 1. Explicit "Score: N" lines (case-insensitive, allow "N/100", "N/100%").
	for _, line := range strings.Split(output, "\n") {
		l := strings.ToLower(strings.TrimSpace(line))
		if !strings.Contains(l, "score") && !strings.Contains(l, "rating") {
			continue
		}
		if v, ok := extractNumber(l); ok && v <= 100 {
			return v
		}
	}
	// 2. Last "N/100" or "N%" anywhere in the text.
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.ToLower(lines[i])
		if v, ok := extractNumber(l); ok && v <= 100 {
			return v
		}
	}
	return 0
}

func extractNumber(s string) (float64, bool) {
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			j := i
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == '.') {
				j++
			}
			var v float64
			if _, err := fmt.Sscanf(s[i:j], "%f", &v); err == nil && v <= 100 {
				return v, true
			}
			return 0, false
		}
	}
	return 0, false
}

func parseIssues(output string, aspect Aspect) []Issue {
	var issues []Issue
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		low := strings.ToLower(line)
		sev := ""
		switch {
		case strings.Contains(low, "[critical]"):
			sev = "critical"
		case strings.Contains(low, "[warning]"), strings.Contains(low, "[warn]"):
			sev = "warning"
		case strings.Contains(low, "[info]"):
			sev = "info"
		}
		if sev == "" {
			continue
		}
		msg := strings.TrimSpace(line)
		msg = strings.TrimPrefix(msg, "-")
		msg = strings.TrimPrefix(msg, "*")
		msg = strings.TrimSpace(msg)
		issues = append(issues, Issue{Aspect: aspect, Severity: sev, Message: msg})
	}
	return issues
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
