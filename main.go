package main

import (
	"fmt"
	"os"

	"github.com/DevAnimecx/deltacode/internal/cli/commands"
	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/setup"
)

func main() {
	cfg, err := config.NewManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config error: %v\n", err)
		os.Exit(1)
	}

	// First-run detection: no providers configured and no arguments
	if setup.IsFirstRun(cfg) && len(os.Args) <= 1 {
		setup.RunWizard(cfg)
		return
	}

	// If first run but user passed a command (e.g., `delta provider add`), let it through
	if setup.IsFirstRun(cfg) && len(os.Args) > 1 {
		cmd := os.Args[1]
		// Allow certain commands without setup
		allowedCmds := map[string]bool{
			"provider": true,
			"help":     true,
			"--help":   true,
			"-h":       true,
			"doctor":   true,
			"init":     true,
		}
		if !allowedCmds[cmd] {
			fmt.Println("Δ Welcome to Delta Code!")
			fmt.Println("First time? Run `delta` with no arguments to launch the setup wizard,")
			fmt.Println("or add a provider first:  delta provider add")
			fmt.Println()
			os.Exit(0)
		}
	}

	commands.ExecuteWithConfig(cfg)
}
