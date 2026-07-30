package commands

import (
	"fmt"
	"os"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/cli/tui"
	"github.com/DevAnimecx/deltacode/internal/setup"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var globalCfg *config.Manager

func Execute() {
	cfg, err := config.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	ExecuteWithConfig(cfg)
}

func ExecuteWithConfig(cfg *config.Manager) {
	globalCfg = cfg

	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "delta",
		Short: "Δ Delta Code — The Self-Evolving BYOK Coding Agent",
		Long: `Delta Code is a self-improving AI software engineer that runs from the terminal.
Connect any provider, any model, any API key.

No vendor lock-in. Full BYOK. Runs everywhere.

Quick start:
  delta              → Launch setup or TUI
  delta run "..."    → Generate code
  delta fix "..."    → Autonomous bug fixing
  delta doctor       → System health check`,

		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if setup.IsFirstRun(globalCfg) && cmd.Use != "provider" && cmd.Use != "init" && cmd.Use != "help" {
				// Allow setup commands to run
			}
		},

		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				if setup.IsFirstRun(globalCfg) {
					setup.RunWizard(globalCfg)
					return
				}
				p := tea.NewProgram(tui.NewChatModel(globalCfg))
				if _, err := p.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
					os.Exit(1)
				}
			}
		},
	}

	// Phase 1 — Foundation
	root.AddCommand(newInitCmd(globalCfg))
	root.AddCommand(newProviderCmd(globalCfg))
	root.AddCommand(newModelsCmd(globalCfg))
	root.AddCommand(newRunCmd(globalCfg))
	root.AddCommand(newCommitCmd(globalCfg))
	root.AddCommand(newDoctorCmd(globalCfg))

	// Phase 2 — Intelligence
	root.AddCommand(newExplainCmd(globalCfg))
	root.AddCommand(newReviewCmd(globalCfg))
	root.AddCommand(newMemoryCmd(globalCfg))

	// Phase 3 — Autonomy
	root.AddCommand(newFixCmd(globalCfg))
	root.AddCommand(newArchitectCmd(globalCfg))
	root.AddCommand(newTestCmd(globalCfg))
	root.AddCommand(newDocsCmd(globalCfg))
	root.AddCommand(newBenchCmd(globalCfg))
	root.AddCommand(newToolCmd(globalCfg))

	// Phase 4 — Polish & Ecosystem
	root.AddCommand(newCheckpointCmd(globalCfg))
	root.AddCommand(newCostCmd(globalCfg))
	root.AddCommand(newPolicyCmd(globalCfg))
	root.AddCommand(newUpdateCmd(globalCfg))
	root.AddCommand(newPRCmd(globalCfg))
	root.AddCommand(newTasksCmd(globalCfg))

	return root
}
