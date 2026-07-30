package commands

import (
	"fmt"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/cost"
	"github.com/spf13/cobra"
)

func newCostCmd(cfg *config.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cost",
		Short:   "Cost estimation and model optimization",
		Aliases: []string{"pricing", "price"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "estimate [model] [input-tokens] [output-tokens]",
		Short: "Estimate cost for a model",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			model := args[0]
			input := parseInt(args[1])
			output := parseInt(args[2])
			e := cost.NewEngine()
			est := e.Estimate(model, input, output)
			fmt.Printf("Cost estimate for %s:\n", model)
			fmt.Printf("  Input tokens:  %d\n", est.InputTokens)
			fmt.Printf("  Output tokens: %d\n", est.OutputTokens)
			fmt.Printf("  Estimated cost: $%.5f\n", est.Cost)
			fmt.Printf("  Quality score:  %.1f/100\n", est.Quality)
			fmt.Printf("  Overall score:  %.1f/100\n", est.Score)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "best [input-tokens] [output-tokens]",
		Short: "Find the best model for a given token budget",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			input := parseInt(args[0])
			output := parseInt(args[1])
			e := cost.NewEngine()
			model, score := e.BestModel(input, output, true)
			fmt.Printf("Best model for %d input / %d output tokens:\n", input, output)
			fmt.Printf("  %s (score: %.1f)\n", model, score)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "models",
		Short: "Show pricing for all models",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Model pricing (per 1K tokens):")
			fmt.Println(strings.Repeat("─", 60))
			fmt.Printf("%-25s %-12s %-12s %-8s %-8s\n", "Model", "Input $", "Output $", "Speed", "Quality")
			fmt.Println(strings.Repeat("─", 60))
			e := cost.NewEngine()
			models := []string{
				"gpt-4o", "gpt-4o-mini", "claude-sonnet-4",
				"claude-haiku-3.5", "gemini-2.0-flash", "gemini-1.5-pro",
				"deepseek-chat", "deepseek-coder",
			}
			for _, m := range models {
				est := e.Estimate(m, 1000, 1000)
				fmt.Printf("%-25s $%-9.4f $%-9.4f %-8.0f %-8.0f\n",
					m, est.Cost/2, est.Cost/2, est.Quality, est.Quality)
			}
		},
	})

	return cmd
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
