package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/internal/provider"
	"github.com/delta-code/cli/pkg/models"
	"github.com/spf13/cobra"
)

type ReviewResult struct {
	Issues  []Issue `json:"issues"`
	Summary string  `json:"summary"`
	Score   int     `json:"score"`
}

type Issue struct {
	Severity   string `json:"severity"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	Suggestion string `json:"suggestion"`
}

func newReviewCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:     "review [file or diff]",
		Short:   "Automated code review using AI",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"rev"},
		Run: func(cmd *cobra.Command, args []string) {
			runReview(cfg, args)
		},
	}
}

func runReview(cfg *config.Manager, args []string) {
	conf := cfg.GetConfig()
	provCfg, err := cfg.GetProvider(conf.DefaultProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	var code string
	if len(args) > 0 {
		code = readFileOrPrompt(args[0])
	} else {
		code = getGitDiffFull()
	}

	if code == "" {
		fmt.Println("Nothing to review. Provide a file path or make some changes.")
		return
	}

	p, err := provider.NewProvider(*provCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	resp, err := p.Chat(models.ChatRequest{
		Model: conf.DefaultModel,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: `You are a code reviewer. Review the provided code and return JSON:
{
  "issues": [
    {
      "severity": "critical|major|minor|info",
      "file": "filename",
      "line": 0,
      "title": "short title",
      "detail": "explanation",
      "suggestion": "how to fix"
    }
  ],
  "summary": "overall review summary",
  "score": 0-100
}
Return ONLY valid JSON.`},
			{Role: models.RoleUser, Content: code},
		},
		Temperature: 0.2,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	content := cleanJSON(resp.Message.Content)
	var review ReviewResult
	if err := json.Unmarshal([]byte(content), &review); err != nil {
		fmt.Println("Review:", resp.Message.Content)
		return
	}

	fmt.Printf("\n📋 Code Review — Score: %d/100\n", review.Score)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("  %s\n", review.Summary)
	fmt.Println()

	if len(review.Issues) == 0 {
		fmt.Println("  ✓ No issues found")
		return
	}

	for _, issue := range review.Issues {
		icon := map[string]string{
			"critical": "🔴",
			"major":    "🟡",
			"minor":    "🔵",
			"info":     "⚪",
		}[issue.Severity]
		if icon == "" {
			icon = "⚪"
		}

		fmt.Printf("  %s [%s]", icon, strings.ToUpper(issue.Severity))
		if issue.File != "" {
			fmt.Printf(" %s", issue.File)
			if issue.Line > 0 {
				fmt.Printf(":%d", issue.Line)
			}
		}
		fmt.Printf("\n    %s\n", issue.Title)
		if issue.Detail != "" {
			fmt.Printf("    %s\n", issue.Detail)
		}
		if issue.Suggestion != "" {
			fmt.Printf("    → %s\n", issue.Suggestion)
		}
		fmt.Println()
	}
}

func getGitDiffFull() string {
	out, err := exec.Command("git", "diff").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func readFileOrPrompt(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return path
	}
	return string(data)
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
