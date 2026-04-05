package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newServerCmd creates the "server" subcommand for IDE integration.
func newServerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start Claude Code server for IDE integration",
		Long:  "Start a server that provides Claude Code integration for IDEs like VS Code and JetBrains.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("IDE server is not yet available.")
			fmt.Println("This feature will be implemented in a future release.")
			return nil
		},
	}

	return cmd
}
