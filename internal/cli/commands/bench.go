package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/bench"
	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/spf13/cobra"
)

func newBenchCmd(cfg *config.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "benchmark",
		Short:   "Run benchmarks on connected models",
		Aliases: []string{"bench"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Benchmark all connected models across task types",
		Run: func(cmd *cobra.Command, args []string) {
			providers := cfg.ListProviders()
			if len(providers) == 0 {
				fmt.Println("No providers configured.")
				return
			}
			if err := bench.Run(providers); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "best [task-type]",
		Short: "Show best model for a task type",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			best := bench.BestForTask(args[0])
			if best == nil {
				fmt.Printf("No benchmark data for %q. Run `delta benchmark run` first.\n", args[0])
				return
			}
			fmt.Printf("Best for %q: %s/%s (score: %.1f)\n", args[0], best.Provider, best.Model, best.Score)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "results",
		Short: "Show all benchmark results",
		Run: func(cmd *cobra.Command, args []string) {
			results := bench.Results()
			if len(results) == 0 {
				fmt.Println("No benchmark results. Run `delta benchmark run` first.")
				return
			}
			fmt.Println("Δ Benchmark Results")
			fmt.Println(strings.Repeat("─", 60))
			fmt.Printf("%-20s %-20s %-10s %-10s %-6s\n", "Provider", "Model", "Task", "Latency", "Score")
			fmt.Println(strings.Repeat("─", 60))
			for _, b := range results {
				lat := b.Latency.Round(time.Millisecond)
				fmt.Printf("%-20s %-20s %-10s %-10s %-6.1f\n", b.Provider, b.Model, b.TaskType, lat, b.Score)
			}
		},
	})

	return cmd
}
