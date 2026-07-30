package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/internal/provider"
	"github.com/delta-code/cli/pkg/models"
	"github.com/spf13/cobra"
)

func newProviderCmd(cfg *config.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "provider",
		Short:   "Manage AI providers",
		Aliases: []string{"providers", "prov"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "add",
		Short: "Add a new AI provider",
		Run: func(cmd *cobra.Command, args []string) {
			addProviderInteractive(cfg)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "remove [name]",
		Short: "Remove a provider",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := cfg.RemoveProvider(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			fmt.Printf("✓ Removed provider %q\n", args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all configured providers",
		Run: func(cmd *cobra.Command, args []string) {
			providers := cfg.ListProviders()
			if len(providers) == 0 {
				fmt.Println("No providers configured. Use `delta provider add` to add one.")
				return
			}
			current := cfg.GetConfig().DefaultProvider
			fmt.Println("Configured providers:")
			for _, p := range providers {
				mark := " "
				if p.Name == current {
					mark = "→"
				}
				keyDisplay := "••••••••"
				if p.APIKey == "" {
					keyDisplay = "no key"
				}
				fmt.Printf("  %s %-12s %-10s %s\n", mark, p.Name, p.Type, keyDisplay)
			}
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "verify [name]",
		Short: "Verify a provider connection",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			verifyProvider(cfg, args)
		},
	})

	cmd.AddCommand(newExportCmd(cfg))
	cmd.AddCommand(newImportCmd(cfg))

	return cmd
}

func addProviderInteractive(cfg *config.Manager) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Provider name (e.g., openai, anthropic, deepseek): ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		fmt.Println("Cancelled.")
		return
	}

	fmt.Println("Provider type:")
	types := []string{"openai", "anthropic", "google", "deepseek", "ollama", "custom"}
	for i, t := range types {
		fmt.Printf("  %d. %s\n", i+1, t)
	}
	fmt.Print("Select type (1-6): ")
	typeIdx := ""
	fmt.Scanln(&typeIdx)

	providerType := models.ProviderOpenAI
	typeIdx = strings.TrimSpace(typeIdx)
	switch typeIdx {
	case "2":
		providerType = models.ProviderAnthropic
	case "3":
		providerType = models.ProviderGoogle
	case "4":
		providerType = models.ProviderDeepSeek
	case "5":
		providerType = models.ProviderOllama
	case "6":
		providerType = models.ProviderCustom
	}

	var baseURL string
	if providerType == models.ProviderOllama {
		baseURL = "http://localhost:11434"
	} else {
		defaultURL := ""
		switch providerType {
		case models.ProviderOpenAI:
			defaultURL = "https://api.openai.com/v1"
		case models.ProviderAnthropic:
			defaultURL = "https://api.anthropic.com/v1"
		case models.ProviderGoogle:
			defaultURL = "https://generativelanguage.googleapis.com/v1beta"
		case models.ProviderDeepSeek:
			defaultURL = "https://api.deepseek.com/v1"
		}
		fmt.Printf("Base URL [%s]: ", defaultURL)
		url, _ := reader.ReadString('\n')
		url = strings.TrimSpace(url)
		if url == "" {
			baseURL = defaultURL
		} else {
			baseURL = url
		}
	}

	fmt.Print("API Key (leave empty if not needed): ")
	key, _ := reader.ReadString('\n')
	key = strings.TrimSpace(key)

	fmt.Print("Models (comma separated, e.g., gpt-4o,gpt-4o-mini): ")
	modelsInput, _ := reader.ReadString('\n')
	modelsInput = strings.TrimSpace(modelsInput)

	var modelsList []string
	if modelsInput != "" {
		for _, m := range strings.Split(modelsInput, ",") {
			modelsList = append(modelsList, strings.TrimSpace(m))
		}
	}

	p := models.ProviderConfig{
		Name:   name,
		Type:   providerType,
		BaseURL: baseURL,
		APIKey: key,
		Models: modelsList,
	}

	if err := cfg.AddProvider(p); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving provider: %v\n", err)
		return
	}

	conf := cfg.GetConfig()
	isFirst := len(cfg.ListProviders()) == 1 || conf.DefaultProvider == "" || conf.DefaultProvider == "openai"
	if isFirst {
		cfg.SetDefault(name)
		fmt.Printf("✓ Provider %q added and set as default\n", name)
	} else {
		fmt.Printf("✓ Provider %q added\n", name)
	}
}

func verifyProvider(cfg *config.Manager, args []string) {
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
		fmt.Fprintf(os.Stderr, "Error creating provider: %v\n", err)
		return
	}

	models, err := p.ListModels()
	if err != nil {
		fmt.Fprintf(os.Stderr, "✗ Connection failed: %v\n", err)
		return
	}

	fmt.Printf("✓ %q connected successfully\n", name)
	if len(models) > 0 {
		fmt.Printf("  Found %d models\n", len(models))
		for _, m := range models[:min(5, len(models))] {
			fmt.Printf("  - %s\n", m.ID)
		}
		if len(models) > 5 {
			fmt.Printf("  ... and %d more\n", len(models)-5)
		}
	}
}
