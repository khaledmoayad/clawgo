// Package fast implements the /fast slash command.
package fast

import "github.com/khaledmoayad/clawgo/internal/commands"

// FastCommand toggles fast mode (smaller model for speed).
type FastCommand struct{}

// New creates a new FastCommand.
func New() *FastCommand { return &FastCommand{} }

func (c *FastCommand) Name() string                  { return "fast" }
func (c *FastCommand) Description() string            { return "Toggle fast mode (use smaller model for speed)" }
func (c *FastCommand) Aliases() []string              { return nil }
func (c *FastCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *FastCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "text", Value: "Fast mode toggled."}, nil
}
