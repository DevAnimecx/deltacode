package commands

import (
	"fmt"
	"os/exec"

	"github.com/delta-code/cli/internal/config"
	"github.com/spf13/cobra"
)

func newDoctorCmd(cfg *config.Manager) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose Delta Code installation and project health",
		Run: func(cmd *cobra.Command, args []string) {
			runDoctor(cfg)
		},
	}
}

func runDoctor(cfg *config.Manager) {
	fmt.Println("Δ Delta Code Doctor")
	fmt.Println()

	issues := 0

	conf := cfg.GetConfig()
	fmt.Printf("Config path: ~/.delta/config.json\n")
	_ = conf

	if _, err := exec.LookPath("git"); err == nil {
		fmt.Println("Git:         ✓ installed")
	} else {
		fmt.Println("Git:         ✗ not found")
		issues++
	}

	providers := cfg.ListProviders()
	if len(providers) == 0 {
		fmt.Println("Providers:   ✗ none configured (run `delta provider add`)")
		issues++
	} else {
		fmt.Printf("Providers:   ✓ %d configured\n", len(providers))
		for _, p := range providers {
			keyStatus := "✓ key set"
			if p.APIKey == "" {
				keyStatus = "✗ no key"
				issues++
			}
			fmt.Printf("  - %s (%s) %s\n", p.Name, p.Type, keyStatus)
		}
	}

	if issues == 0 {
		fmt.Println("\n✓ All checks passed")
	} else {
		fmt.Printf("\n! %d issue(s) found\n", issues)
	}
}
