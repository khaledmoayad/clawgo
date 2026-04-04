// Package permissions implements the /permissions slash command.
package permissions

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// PermissionsCommand shows current permission mode and tool allow/deny lists.
type PermissionsCommand struct{}

// New creates a new PermissionsCommand.
func New() *PermissionsCommand { return &PermissionsCommand{} }

func (c *PermissionsCommand) Name() string        { return "permissions" }
func (c *PermissionsCommand) Description() string  { return "Show current permission mode" }
func (c *PermissionsCommand) Aliases() []string    { return []string{"perm"} }
func (c *PermissionsCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *PermissionsCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	// Permission mode details will be populated when the permissions system
	// is wired into CommandContext. For now, report the basic mode.
	text := fmt.Sprintf("Permission mode: (check session settings)\n"+
		"  Tool allow/deny lists are configured per-session.\n"+
		"  Use --permission-mode flag to change modes.")
	return &commands.CommandResult{Type: "text", Value: text}, nil
}
