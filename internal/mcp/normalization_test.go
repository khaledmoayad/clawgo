package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeNameForMCP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple alphanumeric",
			input:    "myserver",
			expected: "myserver",
		},
		{
			name:     "with dots",
			input:    "my.server.com",
			expected: "my_server_com",
		},
		{
			name:     "with spaces",
			input:    "my server",
			expected: "my_server",
		},
		{
			name:     "with hyphens (preserved)",
			input:    "my-server",
			expected: "my-server",
		},
		{
			name:     "with underscores (preserved)",
			input:    "my_server",
			expected: "my_server",
		},
		{
			name:     "mixed special characters",
			input:    "my.server@v2/api",
			expected: "my_server_v2_api",
		},
		{
			name:     "claude.ai server collapses underscores",
			input:    "claude.ai my..server",
			expected: "claude_ai_my_server",
		},
		{
			name:     "claude.ai server strips leading/trailing underscores",
			input:    "claude.ai .server.",
			expected: "claude_ai_server",
		},
		{
			name:     "non-claude.ai preserves consecutive underscores",
			input:    "my..server",
			expected: "my__server",
		},
		{
			name:     "unicode characters replaced",
			input:    "my\u00e9server",
			expected: "my_server",
		},
		{
			name:     "emoji replaced",
			input:    "cool\U0001F680server",
			expected: "cool_server",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeNameForMCP(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
