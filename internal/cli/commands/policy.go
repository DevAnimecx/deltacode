package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/permissions"
	"github.com/spf13/cobra"
)

func newPolicyCmd(cfg *config.Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "policy",
		Short:   "Manage security policies and permissions",
		Aliases: []string{"perm", "security"},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current policy",
		Run: func(cmd *cobra.Command, args []string) {
			m, err := permissions.NewManager()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			p := m.GetPolicy()
			data, _ := json.MarshalIndent(p, "", "  ")
			fmt.Println(string(data))
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "allow [command]",
		Short: "Add a command to the allow list",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			m, err := permissions.NewManager()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			p := m.GetPolicy()
			p.AllowCommands = append(p.AllowCommands, args[0])
			m.SetPolicy(p)
			fmt.Printf("✓ Allowed: %s\n", args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "deny [command]",
		Short: "Add a command to the deny list",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			m, err := permissions.NewManager()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			p := m.GetPolicy()
			p.DenyCommands = append(p.DenyCommands, args[0])
			m.SetPolicy(p)
			fmt.Printf("✓ Denied: %s\n", args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "vault",
		Short: "Manage secret vault",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Secret vault: set with VAULT_KEY=value environment variables")
		},
	})

	return cmd
}
