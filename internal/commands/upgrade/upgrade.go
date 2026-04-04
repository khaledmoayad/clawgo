// Package upgrade implements the /upgrade slash command.
package upgrade

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// UpgradeCommand checks for ClawGo upgrades.
type UpgradeCommand struct{}

// New creates a new UpgradeCommand.
func New() *UpgradeCommand { return &UpgradeCommand{} }

func (c *UpgradeCommand) Name() string                  { return "upgrade" }
func (c *UpgradeCommand) Description() string            { return "Check for ClawGo upgrades" }
func (c *UpgradeCommand) Aliases() []string              { return nil }
func (c *UpgradeCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *UpgradeCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	v := ctx.Version
	if v == "" {
		v = "unknown"
	}
	text := fmt.Sprintf("Current version: %s\nUpgrade check not yet implemented.", v)
	return &commands.CommandResult{Type: "text", Value: text}, nil
}
