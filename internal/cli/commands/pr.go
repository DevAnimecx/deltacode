package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/internal/prreview"
	"github.com/spf13/cobra"
)

func newPRCmd(cfg *config.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pr",
		Short:   "Review pull requests with AI",
		Aliases: []string{"pull-request"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "review [repo] [pr-number]",
		Short: "Review a GitHub pull request",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			runPRReview(cfg, args[0], args[1])
		},
	})

	return cmd
}

func runPRReview(cfg *config.Manager, repo, prNumber string) {
	conf := cfg.GetConfig()
	provCfg, err := cfg.GetProvider(conf.DefaultProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Println("GITHUB_TOKEN not set. Set it to authenticate with GitHub API.")
		return
	}

	prNum := 0
	for _, c := range prNumber {
		if c >= '0' && c <= '9' {
			prNum = prNum*10 + int(c-'0')
		}
	}
	if prNum == 0 {
		fmt.Fprintf(os.Stderr, "Invalid PR number: %s\n", prNumber)
		return
	}

	client := prreview.NewClient(*provCfg, conf.DefaultModel, token)
	pr, err := client.FetchPR(repo, prNum)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching PR: %v\n", err)
		return
	}

	fmt.Printf("Reviewing PR #%d: %s\n", pr.Number, pr.Title)
	fmt.Println(strings.Repeat("─", 50))

	review, err := client.Review(pr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Review error: %v\n", err)
		return
	}

	fmt.Printf("Summary: %s\n", review.Summary)
	if len(review.Issues) > 0 {
		fmt.Println("\nIssues found:")
		for _, issue := range review.Issues {
			icon := map[string]string{"critical": "🔴", "major": "🟡", "minor": "🔵"}[issue.Severity]
			fmt.Printf("  %s %s:%d — %s\n", icon, issue.File, issue.Line, issue.Description)
		}
	}
	if review.Approve {
		fmt.Println("\n✓ Approved")
	} else {
		fmt.Println("\n✗ Changes requested")
	}
}
