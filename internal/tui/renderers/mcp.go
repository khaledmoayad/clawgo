package renderers

import (
	"fmt"
	"strings"
)

// RenderMCPStatus renders MCP server status messages showing server name,
// connection state (connected/disconnected/connecting), tool and resource
// counts, and transport type. Matches Claude Code's MCP UI components.
func RenderMCPStatus(msg DisplayMessage, width int) string {
	var sb strings.Builder

	serverName := msg.Metadata["server_name"]
	if serverName == "" {
		serverName = "MCP Server"
	}
	state := msg.Metadata["connection_state"]
	toolCount := msg.Metadata["tool_count"]
	resourceCount := msg.Metadata["resource_count"]
	transport := msg.Metadata["transport"]

	// Server name in bold
	sb.WriteString(toolStyle.Render(serverName))

	// Connection state indicator
	switch state {
	case "connected":
		sb.WriteString(" ")
		sb.WriteString(successStyle.Render("\u25CF connected"))
	case "disconnected":
		sb.WriteString(" ")
		sb.WriteString(errorStyle.Render("\u25CF disconnected"))
	case "connecting":
		sb.WriteString(" ")
		sb.WriteString(warningStyle.Render("\u25CB connecting..."))
	default:
		if state != "" {
			sb.WriteString(" ")
			sb.WriteString(dimStyle.Render(fmt.Sprintf("\u25CB %s", state)))
		}
	}

	// Tool and resource counts
	var counts []string
	if toolCount != "" && toolCount != "0" {
		counts = append(counts, fmt.Sprintf("%s tools", toolCount))
	}
	if resourceCount != "" && resourceCount != "0" {
		counts = append(counts, fmt.Sprintf("%s resources", resourceCount))
	}
	if len(counts) > 0 {
		sb.WriteString(dimStyle.Render(fmt.Sprintf(" (%s)", strings.Join(counts, ", "))))
	}

	// Transport type indicator
	if transport != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(fmt.Sprintf("Transport: %s", transport))))
	}

	return sb.String()
}

// RenderMCPToolResult renders a tool result from an MCP server, showing
// the server-prefixed tool name and result content. Handles timeout and
// error states for MCP tool execution.
func RenderMCPToolResult(msg DisplayMessage, _ int) string {
	var sb strings.Builder

	// Tool name with server prefix
	serverName := msg.Metadata["server_name"]
	toolName := msg.ToolName
	if toolName == "" {
		toolName = msg.Metadata["tool_name"]
	}
	if serverName != "" && toolName != "" {
		sb.WriteString(toolStyle.Render(fmt.Sprintf("%s::%s", serverName, toolName)))
	} else if toolName != "" {
		sb.WriteString(toolStyle.Render(toolName))
	} else {
		sb.WriteString(toolStyle.Render("MCP Tool"))
	}

	// Error/timeout states
	if msg.IsError {
		sb.WriteString(" ")
		sb.WriteString(errorStyle.Render("[error]"))
	}
	timeout := msg.Metadata["timeout"]
	if timeout == "true" {
		sb.WriteString(" ")
		sb.WriteString(warningStyle.Render("[timed out]"))
	}

	sb.WriteString("\n")

	// Result content
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}

	return sb.String()
}

// RenderMCPServerList renders a tabular list of connected MCP servers
// with their status indicators. Used for /mcp list command output.
func RenderMCPServerList(msg DisplayMessage, width int) string {
	var sb strings.Builder

	sb.WriteString(toolStyle.Render("MCP Servers"))
	sb.WriteString("\n")

	// The server list is passed as newline-delimited entries in content,
	// each formatted as "name|state|tools|transport"
	if msg.Content == "" {
		sb.WriteString(paddingStyle.Render(dimStyle.Render("No MCP servers connected")))
		return sb.String()
	}

	lines := strings.Split(msg.Content, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 2 {
			sb.WriteString(paddingStyle.Render(line))
			sb.WriteString("\n")
			continue
		}

		name := strings.TrimSpace(parts[0])
		state := strings.TrimSpace(parts[1])

		// Status dot
		var stateStr string
		switch state {
		case "connected":
			stateStr = successStyle.Render("\u25CF")
		case "disconnected":
			stateStr = errorStyle.Render("\u25CF")
		case "connecting":
			stateStr = warningStyle.Render("\u25CB")
		default:
			stateStr = dimStyle.Render("\u25CB")
		}

		entry := fmt.Sprintf("  %s %s", stateStr, name)

		// Append tools count and transport if available
		if len(parts) >= 3 {
			tools := strings.TrimSpace(parts[2])
			if tools != "" && tools != "0" {
				entry += dimStyle.Render(fmt.Sprintf(" (%s tools)", tools))
			}
		}
		if len(parts) >= 4 {
			transport := strings.TrimSpace(parts[3])
			if transport != "" {
				entry += dimStyle.Render(fmt.Sprintf(" [%s]", transport))
			}
		}

		sb.WriteString(entry)
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}
