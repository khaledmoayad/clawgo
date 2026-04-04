// Package clear implements the /clear slash command.
package clear

import "github.com/khaledmoayad/clawgo/internal/commands"

// ClearCommand signals the REPL to clear conversation history.
type ClearCommand struct{}

// New creates a new ClearCommand.
func New() *ClearCommand { return &ClearCommand{} }

func (c *ClearCommand) Name() string        { return "clear" }
func (c *ClearCommand) Description() string  { return "Clear conversation history" }
func (c *ClearCommand) Aliases() []string    { return []string{"cl", "reset", "new"} }
func (c *ClearCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *ClearCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "clear"}, nil
}
