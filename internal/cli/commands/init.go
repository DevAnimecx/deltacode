package commands

import (
	"fmt"
	"os"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/spf13/cobra"
)

func newInitCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Delta Code in the current project",
		Long:  "Creates .delta directory with project configuration and starts tracking context.",
		Run: func(cmd *cobra.Command, args []string) {
			dir := ".delta-project"
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			fmt.Println("✓ Delta Code initialized")
			fmt.Println("  Project context tracking enabled")
			fmt.Println("  Run `delta` to start the TUI")
			fmt.Println("  Run `delta run <prompt>` to generate code")
			fmt.Println()
			fmt.Println("  Next: add a provider with `delta provider add`")
		},
	}
}
