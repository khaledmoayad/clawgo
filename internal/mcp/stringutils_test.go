package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPInfoFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *MCPToolInfo
	}{
		{
			name:  "valid full name",
			input: "mcp__myserver__mytool",
			expected: &MCPToolInfo{
				ServerName: "myserver",
				ToolName:   "mytool",
			},
		},
		{
			name:  "server only (no tool)",
			input: "mcp__myserver",
			expected: &MCPToolInfo{
				ServerName: "myserver",
				ToolName:   "",
			},
		},
		{
			name:  "tool name with double underscores",
			input: "mcp__server__my__complex__tool",
			expected: &MCPToolInfo{
				ServerName: "server",
				ToolName:   "my__complex__tool",
			},
		},
		{
			name:     "not mcp prefix",
			input:    "notmcp__server__tool",
			expected: nil,
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "just mcp",
			input:    "mcp",
			expected: nil,
		},
		{
			name:     "mcp with empty server",
			input:    "mcp__",
			expected: nil,
		},
		{
			name:     "regular tool name",
			input:    "BashTool",
			expected: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := MCPInfoFromString(tc.input)
			if tc.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tc.expected.ServerName, result.ServerName)
				assert.Equal(t, tc.expected.ToolName, result.ToolName)
			}
		})
	}
}

func TestGetMCPPrefix(t *testing.T) {
	assert.Equal(t, "mcp__myserver__", GetMCPPrefix("myserver"))
	assert.Equal(t, "mcp__my_server__", GetMCPPrefix("my.server"))
}

func TestBuildMCPToolName(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
		toolName   string
		expected   string
	}{
		{
			name:       "simple names",
			serverName: "github",
			toolName:   "create_issue",
			expected:   "mcp__github__create_issue",
		},
		{
			name:       "server with dots",
			serverName: "my.server",
			toolName:   "do_thing",
			expected:   "mcp__my_server__do_thing",
		},
		{
			name:       "tool with special chars",
			serverName: "server",
			toolName:   "my tool/v2",
			expected:   "mcp__server__my_tool_v2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := BuildMCPToolName(tc.serverName, tc.toolName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetMCPDisplayName(t *testing.T) {
	assert.Equal(t, "create_issue", GetMCPDisplayName("mcp__github__create_issue", "github"))
	assert.Equal(t, "do_thing", GetMCPDisplayName("mcp__my_server__do_thing", "my.server"))
	// If prefix doesn't match, return full name
	assert.Equal(t, "mcp__other__tool", GetMCPDisplayName("mcp__other__tool", "github"))
}

func TestGetToolNameForPermissionCheck(t *testing.T) {
	// MCP tool: uses fully qualified name
	result := GetToolNameForPermissionCheck("mytool", "github", "create_issue")
	assert.Equal(t, "mcp__github__create_issue", result)

	// Non-MCP tool: uses original name
	result = GetToolNameForPermissionCheck("BashTool", "", "")
	assert.Equal(t, "BashTool", result)
}

func TestExtractMCPToolDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "full format with server and MCP suffix",
			input:    "github - Add comment to issue (MCP)",
			expected: "Add comment to issue",
		},
		{
			name:     "no server prefix",
			input:    "Add comment to issue (MCP)",
			expected: "Add comment to issue",
		},
		{
			name:     "no MCP suffix",
			input:    "github - Add comment to issue",
			expected: "Add comment to issue",
		},
		{
			name:     "plain name",
			input:    "Add comment to issue",
			expected: "Add comment to issue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ExtractMCPToolDisplayName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
