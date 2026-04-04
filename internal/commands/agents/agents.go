// Package agents implements the /agents slash command.
package agents

import "github.com/khaledmoayad/clawgo/internal/commands"

// AgentsCommand lists or manages agent definitions.
type AgentsCommand struct{}

// New creates a new AgentsCommand.
func New() *AgentsCommand { return &AgentsCommand{} }

func (c *AgentsCommand) Name() string                  { return "agents" }
func (c *AgentsCommand) Description() string            { return "List or manage agent definitions" }
func (c *AgentsCommand) Aliases() []string              { return nil }
func (c *AgentsCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *AgentsCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "text", Value: "No agents configured."}, nil
}
