package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/engine"
	"github.com/spf13/cobra"
)

func newRunCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:     "run [prompt]",
		Short:   "Run a code generation prompt",
		Long:    "Send a prompt to the default model and stream the response.",
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"generate", "g"},
		Run: func(cmd *cobra.Command, args []string) {
			prompt := strings.Join(args, " ")
			e := engine.New(cfg)
			if err := e.RunPrompt(prompt, true); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		},
	}
}
