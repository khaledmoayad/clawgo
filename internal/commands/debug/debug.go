// Package debug implements the /debug slash command.
package debug

import "github.com/khaledmoayad/clawgo/internal/commands"

// DebugCommand toggles or shows debug mode.
type DebugCommand struct{}

// New creates a new DebugCommand.
func New() *DebugCommand { return &DebugCommand{} }

func (c *DebugCommand) Name() string        { return "debug" }
func (c *DebugCommand) Description() string  { return "Toggle debug mode" }
func (c *DebugCommand) Aliases() []string    { return nil }
func (c *DebugCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *DebugCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	// Debug mode toggling will be wired to the app state in the TUI phase.
	return &commands.CommandResult{
		Type:  "text",
		Value: "Debug mode toggled. Use --debug flag for verbose output.",
	}, nil
}
