package mcp

import "strings"

// MCPToolInfo contains parsed information from an MCP tool name string.
type MCPToolInfo struct {
	ServerName string
	ToolName   string // empty if only server prefix was matched
}

// MCPInfoFromString extracts MCP server information from a tool name string.
// Expected format: "mcp__serverName__toolName"
// Returns nil if not a valid MCP tool name.
//
// Known limitation: If a server name contains "__", parsing will be incorrect.
// For example, "mcp__my__server__tool" would parse as server="my" and tool="server__tool"
// instead of server="my__server" and tool="tool". This is rare in practice since server
// names typically don't contain double underscores.
func MCPInfoFromString(toolString string) *MCPToolInfo {
	parts := strings.SplitN(toolString, "__", 3)
	if len(parts) < 2 || parts[0] != "mcp" || parts[1] == "" {
		return nil
	}
	info := &MCPToolInfo{
		ServerName: parts[1],
	}
	if len(parts) > 2 {
		info.ToolName = parts[2]
	}
	return info
}

// GetMCPPrefix generates the MCP tool/command name prefix for a given server.
func GetMCPPrefix(serverName string) string {
	return "mcp__" + NormalizeNameForMCP(serverName) + "__"
}

// BuildMCPToolName builds a fully qualified MCP tool name from server and tool names.
// Inverse of MCPInfoFromString().
func BuildMCPToolName(serverName, toolName string) string {
	return GetMCPPrefix(serverName) + NormalizeNameForMCP(toolName)
}

// GetMCPDisplayName extracts the display name from an MCP tool/command name
// by stripping the server prefix.
func GetMCPDisplayName(fullName, serverName string) string {
	prefix := "mcp__" + NormalizeNameForMCP(serverName) + "__"
	return strings.TrimPrefix(fullName, prefix)
}

// GetToolNameForPermissionCheck returns the name to use for permission rule matching.
// For MCP tools, uses the fully qualified mcp__server__tool name so that
// deny rules targeting builtins (e.g., "Write") don't match unprefixed MCP
// replacements that share the same display name.
func GetToolNameForPermissionCheck(name string, mcpServerName string, mcpToolName string) string {
	if mcpServerName != "" {
		return BuildMCPToolName(mcpServerName, mcpToolName)
	}
	return name
}

// ExtractMCPToolDisplayName extracts just the tool/command display name from
// a userFacingName like "github - Add comment to issue (MCP)".
func ExtractMCPToolDisplayName(userFacingName string) string {
	// Remove the (MCP) suffix if present
	withoutSuffix := strings.TrimSpace(strings.TrimSuffix(
		strings.TrimSpace(userFacingName), "(MCP)"))

	// Remove the server prefix (everything before " - ")
	dashIndex := strings.Index(withoutSuffix, " - ")
	if dashIndex != -1 {
		return strings.TrimSpace(withoutSuffix[dashIndex+3:])
	}

	return withoutSuffix
}
