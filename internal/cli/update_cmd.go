package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newUpdateCmd creates the "update" subcommand for checking and installing updates.
func newUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"upgrade"},
		Short:   "Check for updates and install if available",
		Long:    "Check the current version against the latest release and install updates if available.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Current version: %s\n", Version)
			fmt.Println("Self-update not yet available. Download latest from GitHub releases.")
			fmt.Println("  https://github.com/khaledmoayad/clawgo/releases")
			return nil
		},
	}

	return cmd
}
