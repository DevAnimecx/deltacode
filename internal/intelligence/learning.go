package intelligence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type LearningEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Goal      string    `json:"goal"`
	Pattern   string    `json:"pattern"`
	Success   bool      `json:"success"`
	Duration  string    `json:"duration"`
	ToolCount int       `json:"tool_count"`
}

type WorkflowPattern struct {
	Pattern   string    `json:"pattern"`
	Frequency int       `json:"frequency"`
	Tools     []string  `json:"tools"`
	AvgSteps  float64   `json:"avg_steps"`
	LastUsed  time.Time `json:"last_used"`
}

type patternsData struct {
	Patterns []WorkflowPattern `json:"patterns"`
}

func (e *SkillEngine) patternsPath() string {
	return filepath.Join(e.skillsDir, "patterns.json")
}

func (e *SkillEngine) LearnFromWorkflow(goal string, tasks []string, success bool, duration time.Duration) {
	event := LearningEvent{
		Timestamp: time.Now(),
		Goal:      goal,
		Pattern:   extractPattern(goal, tasks),
		Success:   success,
		Duration:  duration.String(),
		ToolCount: len(tasks),
	}
	if success {
		e.recordSuccess(event)
	}
}

func (e *SkillEngine) recordSuccess(event LearningEvent) {
	patterns := e.loadPatterns()

	found := false
	for i, p := range patterns {
		if p.Pattern == event.Pattern {
			patterns[i].Frequency++
			patterns[i].LastUsed = time.Now()
			patterns[i].AvgSteps = (p.AvgSteps*float64(p.Frequency-1) + float64(event.ToolCount)) / float64(p.Frequency)
			found = true
			break
		}
	}

	if !found {
		patterns = append(patterns, WorkflowPattern{
			Pattern:   event.Pattern,
			Frequency: 1,
			Tools:     extractPatternTools(event.Goal),
			AvgSteps:  float64(event.ToolCount),
			LastUsed:  time.Now(),
		})
	}

	e.savePatterns(patterns)

	if len(patterns) > 0 {
		best := patterns[0]
		for _, p := range patterns {
			if p.Frequency > best.Frequency {
				best = p
			}
		}
		if best.Frequency >= 3 {
			e.Learn(best.Pattern, map[string]interface{}{
				"pattern":   best.Pattern,
				"tools":     best.Tools,
				"avg_steps": best.AvgSteps,
			})
		}
	}
}

func (e *SkillEngine) GetTopPatterns(n int) []WorkflowPattern {
	patterns := e.loadPatterns()
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Frequency > patterns[j].Frequency
	})
	if len(patterns) > n {
		patterns = patterns[:n]
	}
	return patterns
}

func (e *SkillEngine) loadPatterns() []WorkflowPattern {
	data, err := os.ReadFile(e.patternsPath())
	if err != nil {
		return nil
	}
	var pd patternsData
	if err := json.Unmarshal(data, &pd); err != nil {
		return nil
	}
	return pd.Patterns
}

func (e *SkillEngine) savePatterns(patterns []WorkflowPattern) {
	pd := patternsData{Patterns: patterns}
	data, err := json.MarshalIndent(pd, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(e.patternsPath(), data, 0600)
}

func extractPattern(goal string, tasks []string) string {
	words := strings.Fields(goal)
	if len(words) > 4 {
		return strings.Join(words[:4], " ")
	}
	return strings.Join(words, " ")
}

func extractPatternTools(goal string) []string {
	words := strings.Fields(strings.ToLower(goal))
	var tools []string
	for _, w := range words {
		switch w {
		case "create", "write", "build", "implement", "generate":
			tools = append(tools, "write")
		case "fix", "debug", "repair":
			tools = append(tools, "edit")
		case "search", "find", "locate":
			tools = append(tools, "search")
		case "run", "execute", "test":
			tools = append(tools, "exec")
		case "delete", "remove":
			tools = append(tools, "delete")
		}
	}
	if len(tools) == 0 {
		tools = append(tools, "write")
	}
	return tools
}
