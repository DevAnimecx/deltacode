package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/internal/timemachine"
	"github.com/spf13/cobra"
)

func newCheckpointCmd(cfg *config.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "checkpoint",
		Short:   "Time Machine — save and manage AI state snapshots",
		Aliases: []string{"cp", "snapshot"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "save [label]",
		Short: "Save a checkpoint of current state",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			label := "manual"
			if len(args) > 0 {
				label = strings.Join(args, " ")
			}
			tm, err := timemachine.New()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			cp, err := tm.Save(label, "", "", "", "")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			fmt.Printf("✓ Checkpoint saved: %s (%s)\n", cp.ID, label)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "undo [id]",
		Short: "Restore state from a checkpoint",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			tm, err := timemachine.New()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			if err := tm.Undo(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "replay [id]",
		Short: "Replay an AI session from a checkpoint",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			tm, err := timemachine.New()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			if err := tm.Replay(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "compare [id1] [id2]",
		Short: "Compare two checkpoints",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			tm, err := timemachine.New()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			if err := tm.Compare(args[0], args[1]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "branch [id] [name]",
		Short: "Branch from a checkpoint",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			tm, err := timemachine.New()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			if err := tm.Branch(args[0], args[1]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "log",
		Short: "Show checkpoint history",
		Run: func(cmd *cobra.Command, args []string) {
			tm, err := timemachine.New()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			tm.Log(20)
		},
	})

	return cmd
}
