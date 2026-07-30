package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/internal/toolmaker"
	"github.com/spf13/cobra"
)

func newToolCmd(cfg *config.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tool",
		Short:   "Auto-create and manage tools",
		Aliases: []string{"tools"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "create [task]",
		Short: "Auto-create a tool to solve a task",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runToolCreate(cfg, strings.Join(args, " "))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List installed tools",
		Run: func(cmd *cobra.Command, args []string) {
			runToolList(cfg)
		},
	})

	return cmd
}

func runToolCreate(cfg *config.Manager, task string) {
	conf := cfg.GetConfig()
	provCfg, err := cfg.GetProvider(conf.DefaultProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	creator, err := toolmaker.NewCreator(*provCfg, conf.DefaultModel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	tool, err := creator.Create(task)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Tool creation failed: %v\n", err)
		return
	}

	fmt.Printf("✓ Tool %q ready at ~/.delta/tools/%s/\n", tool.Name, tool.Name)
}

func runToolList(cfg *config.Manager) {
	conf := cfg.GetConfig()
	provCfg, err := cfg.GetProvider(conf.DefaultProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	creator, err := toolmaker.NewCreator(*provCfg, conf.DefaultModel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	tools, err := creator.ListTools()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	if len(tools) == 0 {
		fmt.Println("No tools installed.")
		return
	}

	fmt.Println("Installed tools:")
	for _, t := range tools {
		desc := t.Description
		if len([]rune(desc)) > 60 {
			desc = string([]rune(desc)[:60]) + "..."
		}
		fmt.Printf("  • %s (%s) — %s\n", t.Name, t.Language, desc)
	}
}
