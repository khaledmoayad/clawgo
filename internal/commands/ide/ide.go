// Package ide implements the /ide slash command.
package ide

import "github.com/khaledmoayad/clawgo/internal/commands"

// IDECommand provides IDE integration setup instructions.
type IDECommand struct{}

// New creates a new IDECommand.
func New() *IDECommand { return &IDECommand{} }

func (c *IDECommand) Name() string                  { return "ide" }
func (c *IDECommand) Description() string            { return "IDE integration setup" }
func (c *IDECommand) Aliases() []string              { return nil }
func (c *IDECommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *IDECommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	text := "IDE Integration\n\n" +
		"  VS Code: Install the Claude Code extension from the marketplace.\n" +
		"  JetBrains: Install the Claude Code plugin from the plugin repository.\n" +
		"  Neovim: See documentation for MCP-based integration.\n\n" +
		"  Run 'clawgo mcp serve' to start the MCP server for IDE integration."
	return &commands.CommandResult{Type: "text", Value: text}, nil
}
