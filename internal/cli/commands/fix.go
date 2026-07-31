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
	cmd := &cobra.Command{
		Use:     "fix [bug description]",
		Short:   "Autonomous bug fixing with Plan→Fix→Run→Debug loop",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"debug"},
		Run: func(cmd *cobra.Command, args []string) {
			workers, _ := cmd.Flags().GetInt("concurrency")
			runFix(cfg, strings.Join(args, " "), workers)
		},
	}
	cmd.Flags().IntP("concurrency", "j", 3, "worker pool size for parallel task execution")
	return cmd
}

func runFix(cfg *config.Manager, task string, workers int) {
	e := autonomous.NewEngine(cfg)
	e.SetConcurrency(workers)
	if err := e.Execute("fix: " + task); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}
