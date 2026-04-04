// Package compact implements the /compact slash command.
package compact

import "github.com/khaledmoayad/clawgo/internal/commands"

// CompactCommand signals the REPL to trigger context compaction.
type CompactCommand struct{}

// New creates a new CompactCommand.
func New() *CompactCommand { return &CompactCommand{} }

func (c *CompactCommand) Name() string        { return "compact" }
func (c *CompactCommand) Description() string  { return "Compact conversation context" }
func (c *CompactCommand) Aliases() []string    { return []string{"co"} }
func (c *CompactCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *CompactCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "compact"}, nil
}
