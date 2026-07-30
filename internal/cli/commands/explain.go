package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/internal/context"
	"github.com/delta-code/cli/internal/provider"
	"github.com/delta-code/cli/pkg/models"
	"github.com/spf13/cobra"
)

func newExplainCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:     "explain [file or prompt]",
		Short:   "Explain code or a change with full context",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"ex"},
		Run: func(cmd *cobra.Command, args []string) {
			runExplain(cfg, strings.Join(args, " "))
		},
	}
}

func runExplain(cfg *config.Manager, prompt string) {
	conf := cfg.GetConfig()
	provCfg, err := cfg.GetProvider(conf.DefaultProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	ctxEng, _ := context.NewEngine()
	ctx := ctxEng.Collect()

	fullPrompt := fmt.Sprintf(`Explain this with project context:

Project Context:
- Files: %s
- Recent changes: %s

Request: %s

Provide a clear, concise explanation.`,
		ctx.FileTree,
		ctx.GitDiff,
		prompt,
	)

	p, err := provider.NewProvider(*provCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	resp, err := p.Chat(models.ChatRequest{
		Model:       conf.DefaultModel,
		Messages:    []models.Message{{Role: models.RoleUser, Content: fullPrompt}},
		Temperature: 0.3,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Println(resp.Message.Content)
}
