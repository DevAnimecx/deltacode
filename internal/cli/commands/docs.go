package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
	"github.com/spf13/cobra"
)

func newDocsCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:     "docs [description or file]",
		Short:   "Auto-generate documentation",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"doc"},
		Run: func(cmd *cobra.Command, args []string) {
			runDocs(cfg, strings.Join(args, " "))
		},
	}
}

func runDocs(cfg *config.Manager, task string) {
	conf := cfg.GetConfig()
	provCfg, err := cfg.GetProvider(conf.DefaultProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
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
			{Role: models.RoleSystem, Content: `Generate clear, comprehensive documentation. Include:
- Overview
- Installation
- Usage examples
- API reference
- Configuration
Return in markdown format.`},
			{Role: models.RoleUser, Content: task},
		},
		Temperature: 0.3,
		MaxTokens:   8192,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	fmt.Println(resp.Message.Content)

	filename := "DELTA_DOCS.md"
	os.WriteFile(filename, []byte(resp.Message.Content), 0644)
	fmt.Printf("Docs saved to: %s\n", filename)
}
