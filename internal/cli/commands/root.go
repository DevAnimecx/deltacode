package commands

import (
	"fmt"
	"os"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/internal/cli/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var cfg *config.Manager

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var err error
	cfg, err = config.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	root := &cobra.Command{
		Use:   "delta",
		Short: "Delta Code — The Self-Evolving BYOK Coding Agent",
		Long: `Delta Code is a self-improving AI software engineer that runs from the terminal.
Connect any provider, any model, any API key.`,
		Run: func(cmd *cobra.Command, args []string) {
			p := tea.NewProgram(tui.NewModel())
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// Phase 1 — Foundation
	root.AddCommand(newInitCmd(cfg))
	root.AddCommand(newProviderCmd(cfg))
	root.AddCommand(newModelsCmd(cfg))
	root.AddCommand(newRunCmd(cfg))
	root.AddCommand(newCommitCmd(cfg))
	root.AddCommand(newDoctorCmd(cfg))

	// Phase 2 — Intelligence
	root.AddCommand(newExplainCmd(cfg))
	root.AddCommand(newReviewCmd(cfg))
	root.AddCommand(newMemoryCmd(cfg))

	// Phase 3 — Autonomy
	root.AddCommand(newFixCmd(cfg))
	root.AddCommand(newArchitectCmd(cfg))
	root.AddCommand(newTestCmd(cfg))
	root.AddCommand(newDocsCmd(cfg))
	root.AddCommand(newBenchCmd(cfg))
	root.AddCommand(newToolCmd(cfg))

	// Phase 4 — Polish & Ecosystem
	root.AddCommand(newCheckpointCmd(cfg))
	root.AddCommand(newCostCmd(cfg))
	root.AddCommand(newPolicyCmd(cfg))
	root.AddCommand(newUpdateCmd(cfg))
	root.AddCommand(newPRCmd(cfg))
	root.AddCommand(newTasksCmd(cfg))

	return root
}
