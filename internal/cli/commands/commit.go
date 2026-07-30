package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
	"github.com/spf13/cobra"
)

func newCommitCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "commit [message]",
		Short: "Generate AI commit message and commit changes",
		Long:  "Stages all changes, generates an AI commit message from the diff, and commits.",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			commitWithAI(cfg, args)
		},
	}
}

func commitWithAI(cfg *config.Manager, args []string) {
	diff := getGitDiff()
	if diff == "" {
		fmt.Println("No changes to commit.")
		return
	}

	message := ""
	if len(args) > 0 {
		message = args[0]
	} else {
		msg, err := generateCommitMessage(cfg, diff)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating message: %v\n", err)
			return
		}
		message = msg
	}

	exec.Command("git", "add", "-A").Run()
	out, err := exec.Command("git", "commit", "-m", message).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Commit error: %v\n%s\n", err, string(out))
		return
	}
	fmt.Printf("✓ Committed: %s\n", message)
	fmt.Println(string(out))
}

func getGitDiff() string {
	out, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil {
		out, err = exec.Command("git", "diff", "--stat").Output()
		if err != nil {
			return ""
		}
	}
	return strings.TrimSpace(string(out))
}

func generateCommitMessage(cfg *config.Manager, diff string) (string, error) {
	conf := cfg.GetConfig()
	provCfg, err := cfg.GetProvider(conf.DefaultProvider)
	if err != nil {
		return "", err
	}

	p, err := provider.NewProvider(*provCfg)
	if err != nil {
		return "", err
	}

	resp, err := p.Chat(models.ChatRequest{
		Model: conf.DefaultModel,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: "Generate a concise git commit message (max 72 chars) from this diff. Return ONLY the message."},
			{Role: models.RoleUser, Content: diff},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resp.Message.Content), nil
}
