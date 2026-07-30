package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/fusion"
	"github.com/spf13/cobra"
)

func newArchitectCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:     "architect [project description]",
		Short:   "Generate architecture plans with multi-model orchestration",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"arch", "plan"},
		Run: func(cmd *cobra.Command, args []string) {
			runArchitect(cfg, strings.Join(args, " "))
		},
	}
}

func runArchitect(cfg *config.Manager, prompt string) {
	conf := cfg.GetConfig()
	fe := fusion.NewFusionEngine(conf.Providers)

	graph := fe.Plan(prompt)
	if err := fe.Execute(graph); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	output := fe.Merge(graph)
	fmt.Println(output)
}
