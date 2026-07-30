package commands

import (
	"fmt"
	"strings"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/internal/tasks"
	"github.com/spf13/cobra"
)

func newTasksCmd(cfg *config.Manager) *cobra.Command {
	manager := tasks.NewManager()

	cmd := &cobra.Command{
		Use:     "tasks",
		Short:   "Manage long-running background tasks",
		Aliases: []string{"task", "jobs"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all tasks",
		Run: func(cmd *cobra.Command, args []string) {
			taskList := manager.List()
			if len(taskList) == 0 {
				fmt.Println("No tasks.")
				return
			}
			fmt.Println("Tasks:")
			for _, t := range taskList {
				fmt.Printf("  %-30s %-10s %5.0f%%\n", t.ID, t.Status, t.Progress)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "run [description]",
		Short: "Run a background task",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			task := manager.Create(strings.Join(args, " "))
			manager.Start(task.ID)
			fmt.Printf("Task %s created and started\n", task.ID)
		},
	})

	return cmd
}
