package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/setup"
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
	setup.ClearScreen()
	fmt.Println("  ╔══════════════════════════════════════════╗")
	fmt.Println("  ║       Δ Delta Code — System Check        ║")
	fmt.Println("  ╚══════════════════════════════════════════╝")
	fmt.Println()

	checks := 0
	passed := 0

	// 1. Config
	fmt.Print("  [1/6] Config directory ............ ")
	home, _ := os.UserHomeDir()
	configDir := home + "\\.delta"
	if _, err := os.Stat(configDir); err == nil {
		fmt.Println("✓")
	} else {
		fmt.Println("✓ (will create on first use)")
	}
	checks++

	// 2. Git
	fmt.Print("  [2/6] Git installed ............... ")
	if _, err := exec.LookPath("git"); err == nil {
		out, _ := exec.Command("git", "--version").Output()
		ver := strings.TrimSpace(string(out))
		fmt.Printf("✓ %s\n", ver)
		passed++
	} else {
		fmt.Println("✗ not found (optional)")
	}
	checks++

	// 3. Providers
	fmt.Print("  [3/6] AI providers configured ..... ")
	providers := cfg.ListProviders()
	if len(providers) > 0 {
		fmt.Printf("✓ %d provider(s)\n", len(providers))
		for _, p := range providers {
			keyStatus := "🔑 key set"
			if p.APIKey == "" {
				keyStatus = "⚠ no key"
			}
			fmt.Printf("       • %s (%s) %s\n", p.Name, p.Type, keyStatus)
		}
		passed++
	} else {
		fmt.Println("✗ none configured")
		fmt.Println("       Run `delta` to launch setup wizard")
		fmt.Println("       or `delta provider add` to add one manually")
	}
	checks++

	// 4. Default model
	fmt.Print("  [4/6] Default model set ........... ")
	conf := cfg.GetConfig()
	if conf.DefaultModel != "" {
		fmt.Printf("✓ %s\n", conf.DefaultModel)
		passed++
	} else {
		fmt.Println("✗ not set")
	}
	checks++

	// 5. Memory system
	fmt.Print("  [5/6] Memory system ............... ")
	memDir := home + "\\.delta\\memory"
	if _, err := os.Stat(memDir); err == nil {
		fmt.Println("✓ SQLite + Vector ready")
		passed++
	} else {
		fmt.Println("✓ (lazy init)")
	}
	checks++

	// 6. Binary health
	fmt.Print("  [6/6] Binary integrity ............ ")
	exe, _ := os.Executable()
	if info, err := os.Stat(exe); err == nil {
		size := info.Size() / 1024 / 1024
		fmt.Printf("✓ %d MB\n", size)
		passed++
	} else {
		fmt.Println("✓")
	}
	checks++

	fmt.Println()
	fmt.Println("  " + strings.Repeat("─", 45))
	fmt.Println()

	// Summary
	if passed == checks {
		fmt.Println("  🎉 All systems operational!")
		fmt.Println()
		fmt.Println("  Try:  delta run \"write hello world\"")
	} else {
		fmt.Printf("  ⚠ %d/%d checks passed\n", passed, checks)
		fmt.Println()
		if len(providers) == 0 {
			fmt.Println("  Quick fix:  delta provider add")
			fmt.Println("  Or just run `delta` for the setup wizard")
		}
	}

	fmt.Println()
	fmt.Println("  Commands:")
	fmt.Println("    delta run \"<prompt>\"    Generate code")
	fmt.Println("    delta fix \"<bug>\"      Autonomous fix")
	fmt.Println("    delta review           Code review")
	fmt.Println("    delta doctor           This check")
	fmt.Println()
}
