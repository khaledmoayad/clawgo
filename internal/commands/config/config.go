// Package config implements the /config slash command.
package config

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

// ConfigCommand shows the current configuration summary.
type ConfigCommand struct{}

// New creates a new ConfigCommand.
func New() *ConfigCommand { return &ConfigCommand{} }

func (c *ConfigCommand) Name() string        { return "config" }
func (c *ConfigCommand) Description() string  { return "Show current configuration" }
func (c *ConfigCommand) Aliases() []string    { return []string{"cfg"} }
func (c *ConfigCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *ConfigCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	text := fmt.Sprintf("Configuration Summary\n"+
		"  Model:       %s\n"+
		"  Working dir: %s\n"+
		"  Session ID:  %s\n"+
		"  Version:     %s",
		ctx.Model, ctx.WorkingDir, ctx.SessionID, ctx.Version)
	return &commands.CommandResult{Type: "text", Value: text}, nil
}
