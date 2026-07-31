package agents

import (
	"fmt"
	"strings"
)

type Review struct {
	Category    string   `json:"category"`
	Severity    string   `json:"severity"`
	Issues      []string `json:"issues"`
	Score       float64  `json:"score"`
	Suggestions []string `json:"suggestions"`
}

const (
	SeverityCritical = "critical"
	SeverityWarning  = "warning"
	SeverityInfo     = "info"
)

func NewReviewer() Agent {
	return &baseAgent{
		name:         "Reviewer",
		phase:        PhaseReflect,
		description:  "Reviews code for correctness, readability, and consistency.",
		systemPrompt: `You are the Reviewer agent in a cognitive engineering engine. Review the code critically. Report: (1) bugs and correctness issues, (2) readability problems, (3) style/consistency issues. Be specific with line references. End with an overall score 0-100.`,
		temperature:  0.2,
		maxTokens:    8192,
		runFn: func(ctx *Context, task Task, base *baseAgent) (*Result, error) {
			out, err := base.chat(ctx, buildUserPrompt(task))
			if err != nil {
				return nil, err
			}
			return &Result{Output: out}, nil
		},
	}
}

func NewSecurityAuditor() Agent {
	return &baseAgent{
		name:         "SecurityAuditor",
		phase:        PhaseReflect,
		description:  "Audits code for security vulnerabilities.",
		systemPrompt: `You are the SecurityAuditor agent in a cognitive engineering engine. Audit the code for security vulnerabilities: injection, XSS, CSRF, auth flaws, secrets, unsafe deserialization, path traversal, dependency issues. Report each finding with severity (critical/warning/info) and a concrete fix.`,
		temperature:  0.2,
		maxTokens:    8192,
		runFn: func(ctx *Context, task Task, base *baseAgent) (*Result, error) {
			out, err := base.chat(ctx, buildUserPrompt(task))
			if err != nil {
				return nil, err
			}
			return &Result{Output: out}, nil
		},
	}
}

func NewDebugger() Agent {
	return &baseAgent{
		name:         "Debugger",
		phase:        PhaseExecute,
		description:  "Diagnoses and fixes bugs and failures.",
		systemPrompt: `You are the Debugger agent in a cognitive engineering engine. Analyze the error, find the root cause, and return the COMPLETE corrected file inside a single fenced code block. Explain the root cause briefly first.`,
		temperature:  0.2,
		maxTokens:    16384,
		runFn: func(ctx *Context, task Task, base *baseAgent) (*Result, error) {
			out, err := base.chat(ctx, buildUserPrompt(task))
			if err != nil {
				return nil, err
			}
			files := []string{}
			if task.File != "" {
				files = append(files, task.File)
			}
			return &Result{Output: out, Files: files}, nil
		},
	}
}

func NewOptimizer() Agent {
	return &baseAgent{
		name:         "Optimizer",
		phase:        PhaseExecute,
		description:  "Improves performance and resource usage.",
		systemPrompt: `You are the Optimizer agent in a cognitive engineering engine. Optimize the code for performance: algorithmic complexity, allocations, I/O, caching, concurrency. Preserve behavior. Return the COMPLETE optimized file inside a single fenced code block, with a short summary of changes.`,
		temperature:  0.2,
		maxTokens:    16384,
		runFn: func(ctx *Context, task Task, base *baseAgent) (*Result, error) {
			out, err := base.chat(ctx, buildUserPrompt(task))
			if err != nil {
				return nil, err
			}
			return &Result{Output: out}, nil
		},
	}
}

func NewDocWriter() Agent {
	return &baseAgent{
		name:         "DocWriter",
		phase:        PhaseExecute,
		description:  "Writes documentation and code comments.",
		systemPrompt: `You are the DocWriter agent in a cognitive engineering engine. Write clear, accurate documentation. Include usage examples. Return the complete document inside a single fenced code block if it's a file, otherwise plain markdown.`,
		temperature:  0.3,
		maxTokens:    8192,
		runFn: func(ctx *Context, task Task, base *baseAgent) (*Result, error) {
			out, err := base.chat(ctx, buildUserPrompt(task))
			if err != nil {
				return nil, err
			}
			return &Result{Output: out}, nil
		},
	}
}

func NewReleaseManager() Agent {
	return &baseAgent{
		name:         "ReleaseManager",
		phase:        PhaseReflect,
		description:  "Prepares releases: versioning, changelog, verification.",
		systemPrompt: `You are the ReleaseManager agent in a cognitive engineering engine. Prepare the release: verify all tasks complete, suggest a semantic version, draft a changelog, and list verification steps. Output a release summary.`,
		temperature:  0.2,
		maxTokens:    4096,
		runFn: func(ctx *Context, task Task, base *baseAgent) (*Result, error) {
			out, err := base.chat(ctx, buildUserPrompt(task))
			if err != nil {
				return nil, err
			}
			return &Result{Output: out}, nil
		},
	}
}

// ParseScore extracts a 0-100 score from review output.
func ParseScore(out string) float64 {
	score := 0.0
	for _, line := range strings.Split(out, "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(line, "score") || strings.Contains(line, "rating") {
			for i := 0; i < len(line); i++ {
				if line[i] >= '0' && line[i] <= '9' {
					j := i
					for j < len(line) && ((line[j] >= '0' && line[j] <= '9') || line[j] == '.') {
						j++
					}
					var v float64
					if _, err := fmt.Sscanf(line[i:j], "%f", &v); err == nil {
						if v <= 100 {
							score = v
							i = j
						}
					}
				}
			}
		}
	}
	return score
}

var _ = fmt.Sprintf
