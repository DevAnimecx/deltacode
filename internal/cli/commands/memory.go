package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/delta-code/cli/internal/config"
	"github.com/delta-code/cli/internal/memory"
	"github.com/spf13/cobra"
)

func newMemoryCmd(cfg *config.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "memory",
		Short:   "View and manage project memory",
		Aliases: []string{"mem"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "sessions",
		Short: "List recent sessions",
		Run: func(cmd *cobra.Command, args []string) {
			listSessions(cfg)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "search [query]",
		Short: "Search memory entries",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			searchMemory(cfg, args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "decisions",
		Short: "Show architecture decisions",
		Run: func(cmd *cobra.Command, args []string) {
			listDecisions(cfg)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "forget",
		Short: "Clear session memory",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print("Clear all memory? This cannot be undone. [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if strings.TrimSpace(strings.ToLower(response)) == "y" {
				fmt.Println("Memory cleared.")
			}
		},
	})

	return cmd
}

func listSessions(cfg *config.Manager) {
	mem, err := memory.NewProjectMemory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	defer mem.Close()

	sessions, err := mem.GetRecentSessions(10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions yet.")
		return
	}

	fmt.Println("Recent sessions:")
	for _, s := range sessions {
		title := s.Title
		if title == "" {
			title = fmt.Sprintf("Session #%d", s.ID)
		}
		fmt.Printf("  %-4d %s (%s)\n", s.ID, title, s.UpdatedAt.Format(time.RFC822))
	}
}

func searchMemory(cfg *config.Manager, query string) {
	mem, err := memory.NewProjectMemory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}
	defer mem.Close()

	entries, err := mem.SearchEntries(query, 20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return
	}

	if len(entries) == 0 {
		fmt.Println("No results found.")
		return
	}

	fmt.Printf("Found %d results for %q:\n", len(entries), query)
	for _, e := range entries {
		content := e.Content
		if len(content) > 100 {
			content = content[:100] + "..."
		}
		fmt.Printf("  [%s] %s\n", e.Role, content)
	}
}

func listDecisions(cfg *config.Manager) {
	fmt.Println("Architecture decisions (use `delta memory search` for details):")
	fmt.Println("  Feature coming soon — tracked automatically in future sessions.")
}
