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

func newTestCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:     "test [description or file]",
		Short:   "Auto-generate and run tests",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"t"},
		Run: func(cmd *cobra.Command, args []string) {
			runTest(cfg, strings.Join(args, " "))
		},
	}
}

func runTest(cfg *config.Manager, task string) {
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

	fmt.Println("Δ Test Generator")
	fmt.Println(strings.Repeat("─", 50))

	resp, err := p.Chat(models.ChatRequest{
		Model: conf.DefaultModel,
		Messages: []models.Message{
			{Role: models.RoleSystem, Content: `Generate comprehensive tests. Cover edge cases, normal cases, and error cases.
Use the appropriate test framework. Include assertions. Return ONLY the test code.`},
			{Role: models.RoleUser, Content: task},
		},
		Temperature: 0.3,
		MaxTokens:   8192,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	code := resp.Message.Content
	fmt.Println(code)

	saved := saveTestFile(code, task)
	if saved != "" {
		fmt.Printf("Tests saved to: %s\n", saved)
	}
}

func saveTestFile(code, task string) string {
	ext := "test.py"
	if strings.Contains(strings.ToLower(task), "js") || strings.Contains(strings.ToLower(task), "node") {
		ext = "test.js"
	}
	if strings.Contains(strings.ToLower(task), "go") {
		ext = "test.go"
	}

	filename := fmt.Sprintf("delta_test_%s", ext)
	os.WriteFile(filename, []byte(code), 0644)
	return filename
}
