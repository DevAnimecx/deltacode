package bench

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

type Benchmark struct {
	Provider   string        `json:"provider"`
	Model      string        `json:"model"`
	TaskType   string        `json:"task_type"`
	Latency    time.Duration `json:"latency"`
	TokenCount int           `json:"token_count"`
	Cost       float64       `json:"cost"`
	Success    bool          `json:"success"`
	Score      float64       `json:"score"`
}

var benchmarksResults []Benchmark
var mu sync.Mutex

func Run(providers []models.ProviderConfig) error {
	fmt.Println("Δ Benchmark Engine")
	fmt.Println(strings.Repeat("─", 50))

	tasks := []struct {
		Type   string
		Prompt string
	}{
		{"code", "Write a Python function to merge two sorted arrays"},
		{"debug", "Find and fix the bug: def add(a,b): return a-b"},
		{"explain", "Explain what a REST API is in one paragraph"},
		{"test", "Write a unit test for a function that validates email addresses"},
		{"refactor", "Refactor this: if x == True: return True else: return False"},
		{"docs", "Write docstring for a function that calculates fibonacci"},
	}

	for _, prov := range providers {
		p, err := provider.NewProvider(prov)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", prov.Name, err)
			continue
		}

		modelList, _ := p.ListModels()
		if len(modelList) == 0 {
			modelList = []models.Model{{ID: prov.Name + "-default"}}
		}

		maxModels := 2
		if len(modelList) < maxModels {
			maxModels = len(modelList)
		}

		for i := 0; i < maxModels; i++ {
			m := modelList[i]
			for _, task := range tasks {
				bench := Benchmark{
					Provider: prov.Name,
					Model:    m.ID,
					TaskType: task.Type,
				}

				start := time.Now()
				resp, err := p.Chat(models.ChatRequest{
					Model: m.ID,
					Messages: []models.Message{
						{Role: models.RoleUser, Content: task.Prompt},
					},
					MaxTokens: 1024,
				})
				bench.Latency = time.Since(start)
				bench.Success = err == nil
				if resp != nil {
					bench.TokenCount = resp.Usage.TotalTokens
				}
				bench.Cost = estimateCost(m.ID, bench.TokenCount)

				if err == nil && resp != nil {
					bench.Score = scoreResponse(resp.Message.Content)
				}

				mu.Lock()
				benchmarksResults = append(benchmarksResults, bench)
				mu.Unlock()

				status := "✓"
				if !bench.Success {
					status = "✗"
				}
				fmt.Printf("  %s %s/%s [%s] %v (%.1f)\n",
					status, prov.Name, m.ID, task.Type,
					bench.Latency.Round(time.Millisecond), bench.Score)
			}
		}
	}

	return nil
}

func Results() []Benchmark {
	sort.Slice(benchmarksResults, func(i, j int) bool {
		return benchmarksResults[i].Score > benchmarksResults[j].Score
	})
	return benchmarksResults
}

func BestForTask(taskType string) *Benchmark {
	var best *Benchmark
	for _, b := range benchmarksResults {
		if b.TaskType == taskType && b.Success {
			if best == nil || b.Score > best.Score {
				best = &b
			}
		}
	}
	return best
}

func scoreResponse(content string) float64 {
	score := 60.0
	words := strings.Fields(content)
	if len(words) > 10 {
		score += 10
	}
	if len(words) > 50 {
		score += 10
	}
	if strings.Contains(content, "```") {
		score += 10
	}
	if !strings.Contains(strings.ToLower(content), "sorry") &&
		!strings.Contains(strings.ToLower(content), "cannot") {
		score += 10
	}
	if score > 100 {
		score = 100
	}
	return score
}

func estimateCost(model string, tokens int) float64 {
	rates := map[string]float64{
		"gpt-4o":         0.005,
		"gpt-4o-mini":    0.001,
		"claude-sonnet":  0.003,
		"claude-haiku":   0.001,
		"gemini":         0.0005,
		"deepseek-chat":  0.0005,
		"deepseek-coder": 0.001,
	}
	for key, rate := range rates {
		if strings.Contains(strings.ToLower(model), strings.ToLower(key)) {
			return float64(tokens) / 1000 * rate
		}
	}
	return float64(tokens) / 1000 * 0.001
}
