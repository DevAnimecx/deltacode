package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/pkg/models"
	"github.com/spf13/cobra"
)

func newExportCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "export [filename]",
		Short: "Export all provider configurations",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filename := "delta-providers.json"
			if len(args) > 0 {
				filename = args[0]
			}
			providers := cfg.ListProviders()
			if len(providers) == 0 {
				fmt.Println("No providers to export.")
				return
			}
			type safeProvider struct {
				Name    string   `json:"name"`
				Type    string   `json:"type"`
				BaseURL string   `json:"base_url"`
				Models  []string `json:"models"`
			}
			var safe []safeProvider
			for _, p := range providers {
				safe = append(safe, safeProvider{
					Name:    p.Name,
					Type:    string(p.Type),
					BaseURL: p.BaseURL,
					Models:  p.Models,
				})
			}
			data, _ := json.MarshalIndent(safe, "", "  ")
			os.WriteFile(filename, data, 0644)
			fmt.Printf("✓ Exported %d providers to %s (API keys excluded)\n", len(safe), filename)
		},
	}
}

func newImportCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "import [filename]",
		Short: "Import provider configurations from a file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			data, err := os.ReadFile(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			var imports []struct {
				Name    string   `json:"name"`
				Type    string   `json:"type"`
				BaseURL string   `json:"base_url"`
				APIKey  string   `json:"api_key"`
				Models  []string `json:"models"`
			}
			if err := json.Unmarshal(data, &imports); err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing file: %v\n", err)
				return
			}
			count := 0
			for _, imp := range imports {
				if imp.APIKey == "" {
					fmt.Printf("  Skipping %s: no API key\n", imp.Name)
					continue
				}
				cfg.AddProvider(models.ProviderConfig{
					Name:    imp.Name,
					Type:    models.ProviderType(imp.Type),
					BaseURL: imp.BaseURL,
					APIKey:  imp.APIKey,
					Models:  imp.Models,
				})
				count++
			}
			fmt.Printf("✓ Imported %d providers\n", count)
		},
	}
}
