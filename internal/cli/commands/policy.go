package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/permissions"
	"github.com/DevAnimecx/deltacode/internal/tools"
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
		Use:   "tool-allow [tool]",
		Short: "Approve a tool for future runs",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			reg := tools.NewRegistry()
			if _, err := reg.Get(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			reg.SetToolAllowed(args[0])
			fmt.Printf("✓ Tool allowed: %s\n", args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "tool-deny [tool]",
		Short: "Deny a tool for future runs",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			reg := tools.NewRegistry()
			if _, err := reg.Get(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}
			reg.SetToolDenied(args[0])
			fmt.Printf("✓ Tool denied: %s\n", args[0])
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "tool-policy [ask|allow_all|deny_all|remember]",
		Short: "Set the global tool approval policy",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			reg := tools.NewRegistry()
			switch args[0] {
			case "ask":
				reg.SetPolicy(tools.PolicyAsk)
			case "allow_all":
				reg.SetPolicy(tools.PolicyAllowAll)
			case "deny_all":
				reg.SetPolicy(tools.PolicyDenyAll)
			case "remember":
				reg.SetPolicy(tools.PolicyRemember)
			default:
				fmt.Fprintf(os.Stderr, "unknown policy %q (ask|allow_all|deny_all|remember)\n", args[0])
				return
			}
			fmt.Printf("✓ Tool policy set to %s\n", args[0])
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
