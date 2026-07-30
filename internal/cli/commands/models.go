package commands

import (
	"fmt"
	"os"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/spf13/cobra"
)

func newModelsCmd(cfg *config.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "models",
		Short:   "List and manage models",
		Aliases: []string{"model", "m"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "list [provider]",
		Short: "List available models from a provider",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			listModels(cfg, args)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "sync",
		Short: "Sync models from all configured providers",
		Run: func(cmd *cobra.Command, args []string) {
			syncModels(cfg)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "set [model]",
		Short: "Set default model",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			conf := cfg.GetConfig()
			conf.DefaultModel = args[0]
			fmt.Printf("✓ Default model set to %q\n", args[0])
		},
	})

	return cmd
}

func listModels(cfg *config.Manager, args []string) {
	name := cfg.GetConfig().DefaultProvider
	if len(args) > 0 {
		name = args[0]
	}

	provCfg, err := cfg.GetProvider(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	p, err := provider.NewProvider(*provCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	models, err := p.ListModels()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching models: %v\n", err)
		return
	}

	if len(models) == 0 {
		fmt.Printf("No models found for %q\n", name)
		return
	}

	fmt.Printf("Models from %q:\n", name)
	for _, m := range models {
		current := "  "
		if m.ID == cfg.GetConfig().DefaultModel {
			current = "→ "
		}
		fmt.Printf("  %s%s\n", current, m.ID)
	}
}

func syncModels(cfg *config.Manager) {
	providers := cfg.ListProviders()
	if len(providers) == 0 {
		fmt.Println("No providers configured.")
		return
	}

	for _, provCfg := range providers {
		p, err := provider.NewProvider(provCfg)
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", provCfg.Name, err)
			continue
		}
		models, err := p.ListModels()
		if err != nil {
			fmt.Printf("  ✗ %s: %v\n", provCfg.Name, err)
			continue
		}
		provCfg.Models = make([]string, len(models))
		for i, m := range models {
			provCfg.Models[i] = m.ID
		}
		cfg.AddProvider(provCfg)
		fmt.Printf("  ✓ %s: %d models\n", provCfg.Name, len(models))
	}
	fmt.Println("Done.")
}
