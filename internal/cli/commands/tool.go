package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/toolmaker"
	"github.com/DevAnimecx/deltacode/internal/tools"
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

	cmd.AddCommand(&cobra.Command{
		Use:   "search [query]",
		Short: "Search the tool registry",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			q := ""
			if len(args) > 0 {
				q = args[0]
			}
			runToolSearch(q)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "info [tool]",
		Short: "Show manifest and stats for a tool",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runToolInfo(args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "call [tool] [args...]",
		Short: "Call a tool directly",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runToolCall(args[0], args[1:])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "rank [capability]",
		Short: "Rank tools for a capability",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runToolRank(strings.Join(args, " "))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "build [task]",
		Short: "Build a tool autonomously (registry -> plugins -> OSS -> scaffold -> install)",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runToolBuild(cfg, strings.Join(args, " "))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "reload",
		Short: "Hot reload user-installed tools",
		Run: func(cmd *cobra.Command, args []string) {
			added := newToolRegistry().Reload()
			fmt.Printf("✓ Reloaded registry (%d new tool(s))\n", added)
		},
	})

	return cmd
}

// newToolRegistry builds a registry shared by the tool commands.
func newToolRegistry() *tools.Registry {
	return tools.NewRegistry()
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
	reg := newToolRegistry()
	tools := reg.ListHealthy()
	if len(tools) == 0 {
		fmt.Println("No tools available.")
		return
	}

	fmt.Printf("Tool registry (%d tools):\n", len(tools))
	for _, t := range tools {
		cat := t.Manifest.Category
		if cat == "" {
			cat = "general"
		}
		desc := t.Manifest.Description
		if len([]rune(desc)) > 60 {
			desc = string([]rune(desc)[:60]) + "..."
		}
		fmt.Printf("  • %-14s [%-18s] %s\n", t.Name(), cat, desc)
	}

	// Also surface user-created tools from the toolmaker.
	conf := cfg.GetConfig()
	provCfg, err := cfg.GetProvider(conf.DefaultProvider)
	if err == nil {
		creator, err := toolmaker.NewCreator(*provCfg, conf.DefaultModel)
		if err == nil {
			userTools, err := creator.ListTools()
			if err == nil && len(userTools) > 0 {
				fmt.Printf("\nUser-created tools (%d):\n", len(userTools))
				for _, t := range userTools {
					fmt.Printf("  • %s (%s) — %s\n", t.Name, t.Language, t.Description)
				}
			}
		}
	}
}

func runToolSearch(q string) {
	reg := newToolRegistry()
	results := reg.Search(q)
	if len(results) == 0 {
		fmt.Printf("No tools match %q.\n", q)
		return
	}
	fmt.Printf("%d tool(s) matching %q:\n", len(results), q)
	for _, t := range results {
		cat := t.Manifest.Category
		fmt.Printf("  • %-14s [%s] %s\n", t.Name(), cat, t.Manifest.Description)
	}
}

func runToolInfo(name string) {
	reg := newToolRegistry()
	t, err := reg.Get(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	data, _ := json.MarshalIndent(t.Manifest, "", "  ")
	fmt.Println(string(data))
	st, _ := reg.StatsFor(t.Name())
	statsData, _ := json.MarshalIndent(st, "", "  ")
	fmt.Println("\nStats:")
	fmt.Println(string(statsData))
}

func runToolCall(name string, args []string) {
	reg := newToolRegistry()
	out, err := reg.Call(name, args...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	fmt.Println(out)
}

func runToolRank(capability string) {
	reg := newToolRegistry()
	ranker := tools.NewRanker()
	candidates := reg.List()
	ranked := ranker.Rank(capability, candidates, 10)
	if len(ranked) == 0 {
		fmt.Println("No tools available for this capability.")
		return
	}
	fmt.Printf("Top tools for %q:\n", capability)
	for i, rt := range ranked {
		fmt.Printf("  %d. %-14s score %.2f (%s)\n", i+1, rt.Tool.Name(), rt.Score, rt.Tool.Manifest.Description)
	}
}

func runToolBuild(cfg *config.Manager, task string) {
	reg := newToolRegistry()

	// Ladder step 1: search the built-in registry.
	results := reg.Search(task)
	if len(results) > 0 {
		best := results[0]
		fmt.Printf("✓ Capability already covered by built-in tool %q\n", best.Name())
		fmt.Printf("  %s\n", best.Manifest.Description)
		return
	}

	// Ladder step 2: suggest a user tool install via toolmaker.
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
		fmt.Fprintf(os.Stderr, "Tool build failed: %v\n", err)
		return
	}
	fmt.Printf("✓ Built tool %q at ~/.delta/tools/%s/\n", tool.Name, tool.Name)
	fmt.Println("  Register with: delta tool reload")
	_ = reg
}
