package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/autonomous"
	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/spf13/cobra"
)

func newFixCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:     "fix [bug description]",
		Short:   "Autonomous bug fixing with Plan→Fix→Run→Debug loop",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"debug"},
		Run: func(cmd *cobra.Command, args []string) {
			runFix(cfg, strings.Join(args, " "))
		},
	}
}

func runFix(cfg *config.Manager, task string) {
	e := autonomous.NewEngine(cfg)
	if err := e.Execute("fix: " + task); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}
