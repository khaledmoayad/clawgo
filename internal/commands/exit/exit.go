// Package exit implements the /exit slash command.
package exit

import "github.com/khaledmoayad/clawgo/internal/commands"

// ExitCommand signals the REPL to quit.
type ExitCommand struct{}

// New creates a new ExitCommand.
func New() *ExitCommand { return &ExitCommand{} }

func (c *ExitCommand) Name() string        { return "exit" }
func (c *ExitCommand) Description() string  { return "Exit the REPL" }
func (c *ExitCommand) Aliases() []string    { return []string{"q", "quit"} }
func (c *ExitCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *ExitCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "exit", ExitRequested: true}, nil
}
