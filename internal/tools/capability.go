package tools

import (
	"sort"
	"strings"
)

// CapabilityMatch is the result of analyzing a task against the registry.
type CapabilityMatch struct {
	Tool      *Tool
	Score     float64
	Reason    string
	NeedsUser bool // tool exists but requires user approval to run
}

// Analyze matches a natural-language capability/query to the best tools.
func (r *Registry) Analyze(capability string) []CapabilityMatch {
	ranker := NewRanker()
	candidates := r.ListHealthy()
	ranked := ranker.Rank(capability, candidates, 5)

	matches := make([]CapabilityMatch, 0, len(ranked))
	for _, rt := range ranked {
		reason := "capability match"
		if rt.Score < 0.15 {
			continue
		}
		needsUser := rt.Tool.NeedsApproval(r.policy)
		if needsUser {
			reason = "requires approval"
		}
		matches = append(matches, CapabilityMatch{
			Tool:      rt.Tool,
			Score:     rt.Score,
			Reason:    reason,
			NeedsUser: needsUser,
		})
	}
	return matches
}

// PlanFor splits a high-level task into tool invocations, ranked best-first.
type PlanStep struct {
	Tool   string   `json:"tool"`
	Args   []string `json:"args"`
	Score  float64  `json:"score"`
	Reason string   `json:"reason,omitempty"`
}

// Plan produces an ordered list of tool steps to accomplish a task.
func (r *Registry) Plan(capability string) ([]PlanStep, bool) {
	matches := r.Analyze(capability)
	if len(matches) == 0 {
		return nil, false
	}
	// Derive likely arguments from the capability text (paths, packages, etc.).
	args := inferArgs(capability)
	steps := make([]PlanStep, 0, len(matches))
	for _, m := range matches {
		step := PlanStep{
			Tool:   m.Tool.Name(),
			Args:   args,
			Score:  m.Score,
			Reason: m.Reason,
		}
		if m.NeedsUser {
			step.Reason = "requires approval: delta policy tool-allow " + m.Tool.Name()
		}
		steps = append(steps, step)
	}
	return steps, true
}

// inferArgs extracts likely positional arguments (paths, names) from a task.
func inferArgs(capability string) []string {
	words := strings.Fields(capability)
	var args []string
	for _, w := range words {
		w = strings.Trim(w, "\"'.,;:()")
		if w == "" {
			continue
		}
		if strings.Contains(w, "/") || strings.Contains(w, "\\") ||
			strings.HasSuffix(w, ".go") || strings.HasSuffix(w, ".py") ||
			strings.HasSuffix(w, ".js") || strings.HasSuffix(w, ".ts") ||
			strings.HasSuffix(w, ".json") || strings.HasSuffix(w, ".md") {
			args = append(args, w)
		}
	}
	return args
}

// SortMatches orders matches by score descending.
func SortMatches(matches []CapabilityMatch) {
	sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
}

// Best returns the single best match, if any.
func (r *Registry) BestMatch(capability string) (CapabilityMatch, bool) {
	matches := r.Analyze(capability)
	if len(matches) == 0 {
		return CapabilityMatch{}, false
	}
	return matches[0], true
}
