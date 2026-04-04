// Package plugin implements the /plugin slash command.
package plugin

import "github.com/khaledmoayad/clawgo/internal/commands"

// PluginCommand manages plugins.
type PluginCommand struct{}

// New creates a new PluginCommand.
func New() *PluginCommand { return &PluginCommand{} }

func (c *PluginCommand) Name() string                  { return "plugin" }
func (c *PluginCommand) Description() string            { return "Manage plugins" }
func (c *PluginCommand) Aliases() []string              { return nil }
func (c *PluginCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *PluginCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "text", Value: "Plugin system available in Phase 6."}, nil
}
