package commands

import (
	"fmt"
	"os"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/update"
	"github.com/spf13/cobra"
)

func newUpdateCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:     "update",
		Short:   "Update Delta Code to the latest version",
		Aliases: []string{"upgrade"},
		Run: func(cmd *cobra.Command, args []string) {
			m := update.NewManager()
			if err := m.Update(); err != nil {
				fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
				return
			}
		},
	}
}
