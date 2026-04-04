// Package mcp implements the /mcp slash command.
package mcp

import "github.com/khaledmoayad/clawgo/internal/commands"

// MCPCommand manages MCP servers.
type MCPCommand struct{}

// New creates a new MCPCommand.
func New() *MCPCommand { return &MCPCommand{} }

func (c *MCPCommand) Name() string                  { return "mcp" }
func (c *MCPCommand) Description() string            { return "Manage MCP servers" }
func (c *MCPCommand) Aliases() []string              { return nil }
func (c *MCPCommand) Type() commands.CommandType     { return commands.CommandTypeLocal }

func (c *MCPCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	return &commands.CommandResult{Type: "text", Value: "MCP management available in Phase 5."}, nil
}
