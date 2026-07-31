package agents

import (
	"encoding/json"
	"strings"
)

type PlannerResult struct {
	Tasks []PlannerTask `json:"tasks"`
}

type PlannerTask struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on"`
	Agent       string   `json:"agent"`
	File        string   `json:"file,omitempty"`
}

func NewPlanner() Agent {
	return &baseAgent{
		name:         "Planner",
		phase:        PhaseDecompose,
		description:  "Decomposes goals into ordered, dependency-aware tasks.",
		systemPrompt: `You are the Planner agent in a cognitive engineering engine. Decompose the goal into small, ordered, dependency-aware tasks. Return ONLY a JSON object: {"tasks":[{"id":"1","title":"...","description":"...","depends_on":[],"agent":"Coder","file":"path"}]}. Agent must be one of: Planner, Architect, Coder, Reviewer, Debugger, Optimizer, SecurityAuditor, DocWriter, TestEngineer, ReleaseManager.`,
		temperature:  0.2,
		maxTokens:    4096,
		runFn: func(ctx *Context, task Task, base *baseAgent) (*Result, error) {
			out, err := base.chat(ctx, buildUserPrompt(task))
			if err != nil {
				return nil, err
			}
			clean := cleanJSON(out)
			var pr PlannerResult
			if json.Unmarshal([]byte(clean), &pr) == nil && len(pr.Tasks) > 0 {
				return &Result{Output: out}, nil
			}
			return &Result{Output: out}, nil
		},
	}
}

func NewArchitect() Agent {
	return &baseAgent{
		name:         "Architect",
		phase:        PhaseWorldModel,
		description:  "Builds the world model: architecture, data flow, component design.",
		systemPrompt: `You are the Architect agent in a cognitive engineering engine. Analyze the project context and design the architecture: components, data flow, dependencies, and trade-offs. Be concrete and reference actual files. Output a structured architecture document.`,
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

func NewCoder() Agent {
	return &baseAgent{
		name:         "Coder",
		phase:        PhaseExecute,
		description:  "Writes production-ready code.",
		systemPrompt: `You are the Coder agent in a cognitive engineering engine. Write production-ready, idiomatic code. Return the COMPLETE file content inside a single fenced code block, nothing else. Follow existing project conventions.`,
		temperature:  0.3,
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

func NewTestEngineer() Agent {
	return &baseAgent{
		name:         "TestEngineer",
		phase:        PhaseExecute,
		description:  "Writes unit and integration tests.",
		systemPrompt: `You are the TestEngineer agent in a cognitive engineering engine. Write comprehensive unit/integration tests covering edge cases. Return the COMPLETE test file inside a single fenced code block. Use the project's existing test framework.`,
		temperature:  0.3,
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

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "```") {
		parts := strings.Split(s, "```")
		for i, part := range parts {
			if i%2 == 1 {
				if idx := strings.Index(part, "\n"); idx != -1 {
					part = part[idx+1:]
				}
				part = strings.TrimSpace(part)
				if part != "" {
					return part
				}
			}
		}
	}
	if i := strings.Index(s, "{"); i != -1 {
		if j := strings.LastIndex(s, "}"); j > i {
			return s[i : j+1]
		}
	}
	return s
}
