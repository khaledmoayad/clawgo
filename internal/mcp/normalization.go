package mcp

import (
	"regexp"
	"strings"
)

// claudeAIServerPrefix identifies claude.ai proxy server names.
const claudeAIServerPrefix = "claude.ai "

// invalidMCPChars matches any character that isn't [a-zA-Z0-9_-].
var invalidMCPChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// consecutiveUnderscores matches two or more underscores in a row.
var consecutiveUnderscores = regexp.MustCompile(`_+`)

// leadingTrailingUnderscores matches underscores at start or end of string.
var leadingTrailingUnderscores = regexp.MustCompile(`^_|_$`)

// NormalizeNameForMCP normalizes server/tool names to be compatible with the
// API pattern ^[a-zA-Z0-9_-]{1,64}$.
// Replaces any invalid characters (including dots and spaces) with underscores.
//
// For claude.ai servers (names starting with "claude.ai "), also collapses
// consecutive underscores and strips leading/trailing underscores to prevent
// interference with the __ delimiter used in MCP tool names.
func NormalizeNameForMCP(name string) string {
	normalized := invalidMCPChars.ReplaceAllString(name, "_")
	if strings.HasPrefix(name, claudeAIServerPrefix) {
		normalized = consecutiveUnderscores.ReplaceAllString(normalized, "_")
		normalized = leadingTrailingUnderscores.ReplaceAllString(normalized, "")
	}
	return normalized
}

// NormalizeServerName normalizes an MCP server name to lowercase with
// non-alphanumeric spans replaced by underscore, trimming leading/trailing _.
// This is the canonical form stored in discovery caches.
func NormalizeServerName(name string) string {
	normalized := strings.ToLower(invalidMCPChars.ReplaceAllString(name, "_"))
	normalized = consecutiveUnderscores.ReplaceAllString(normalized, "_")
	normalized = strings.Trim(normalized, "_")
	return normalized
}

// NormalizeToolName returns the fully qualified Claude Code-compatible tool
// surface name: "mcp__<normalized-server>__<normalized-tool>".
func NormalizeToolName(serverName, toolName string) string {
	return "mcp__" + NormalizeServerName(serverName) + "__" + NormalizeServerName(toolName)
}

// NormalizePromptName returns the fully qualified Claude Code-compatible prompt
// surface name: "mcp__<normalized-server>__<normalized-prompt>".
func NormalizePromptName(serverName, promptName string) string {
	return "mcp__" + NormalizeServerName(serverName) + "__" + NormalizeServerName(promptName)
}
