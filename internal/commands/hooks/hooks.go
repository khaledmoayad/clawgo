// Package hooks implements the /hooks slash command.
package hooks

import "github.com/khaledmoayad/clawgo/internal/commands"

// HooksCommand lists or manages hooks.
type HooksCommand struct{}

// New creates a new HooksCommand.
func New() *HooksCommand { return &HooksCommand{} }

func (c *HooksCommand) Name() string                  { return "hooks" }
func (c *HooksCommand) Description() string            { return "List or manage hooks" }
func (c *HooksCommand) Aliases() []string              { return nil }
func (c *HooksCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *HooksCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "text", Value: "Hooks system available in Phase 6."}, nil
}
