// Package mcp implements the /mcp slash command.
package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/commands"
	mcppkg "github.com/khaledmoayad/clawgo/internal/mcp"
)

// MCPCommand manages MCP servers.
type MCPCommand struct{}

// New creates a new MCPCommand.
func New() *MCPCommand { return &MCPCommand{} }

func (c *MCPCommand) Name() string              { return "mcp" }
func (c *MCPCommand) Description() string        { return "Show MCP server status and discovered prompts" }
func (c *MCPCommand) Aliases() []string          { return nil }
func (c *MCPCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *MCPCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	mgr, ok := ctx.MCPManager.(*mcppkg.Manager)
	if !ok || mgr == nil {
		return &commands.CommandResult{
			Type:  "text",
			Value: "No MCP manager available. MCP servers are not configured.",
		}, nil
	}

	servers := mgr.ServerStatus()
	if len(servers) == 0 {
		return &commands.CommandResult{
			Type:  "text",
			Value: "No MCP servers configured.",
		}, nil
	}

	// Sort for deterministic output
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	var sb strings.Builder
	sb.WriteString("MCP Servers:\n")
	for _, s := range servers {
		statusIcon := statusEmoji(s.Status)
		transport := s.Transport
		if transport == "" {
			transport = "stdio"
		}
		sb.WriteString(fmt.Sprintf("  %s %s (%s) - %s", statusIcon, s.Name, transport, string(s.Status)))
		if s.Status == mcppkg.StatusConnected && s.ToolCount > 0 {
			sb.WriteString(fmt.Sprintf(" [%d tools]", s.ToolCount))
		}
		sb.WriteString("\n")
	}

	// List discovered prompts if any
	prompts := ListDiscoveredPrompts(mgr)
	if len(prompts) > 0 {
		sb.WriteString("\nDiscovered Prompts:\n")
		for _, p := range prompts {
			sb.WriteString(fmt.Sprintf("  - %s", p.NormalizedName))
			if p.Description != "" {
				desc := p.Description
				if len(desc) > 80 {
					desc = desc[:77] + "..."
				}
				sb.WriteString(fmt.Sprintf(": %s", desc))
			}
			sb.WriteString("\n")
		}
	}

	return &commands.CommandResult{
		Type:  "text",
		Value: sb.String(),
	}, nil
}

// ListDiscoveredPrompts returns all prompts from the live MCP manager.
// Exported so other subsystems can access the runtime prompt list.
func ListDiscoveredPrompts(mgr *mcppkg.Manager) []mcppkg.DiscoveredPrompt {
	if mgr == nil {
		return nil
	}
	return mgr.ListDiscoveredPrompts()
}

// statusEmoji returns a text indicator for connection status.
func statusEmoji(status mcppkg.ConnectionStatus) string {
	switch status {
	case mcppkg.StatusConnected:
		return "[ok]"
	case mcppkg.StatusFailed:
		return "[!!]"
	case mcppkg.StatusNeedsAuth:
		return "[auth]"
	case mcppkg.StatusPending:
		return "[..]"
	case mcppkg.StatusDisabled:
		return "[--]"
	default:
		return "[??]"
	}
}
