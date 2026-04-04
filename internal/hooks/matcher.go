package hooks

import (
	"path/filepath"
	"strings"
)

// MatchesToolName checks whether a matcher pattern matches the given tool name.
//
// Rules:
//   - Empty pattern matches everything.
//   - Exact case-insensitive match (e.g., "Bash" matches "bash").
//   - Glob via filepath.Match (e.g., "File*" matches "FileRead").
//   - Parenthesized argument patterns: "Bash(git *)" splits into tool name
//     "Bash" and argument pattern "git *". Tool name must match exactly
//     (case-insensitive) and argument pattern is matched via filepath.Match
//     against the provided toolName's argument portion.
func MatchesToolName(pattern, toolName string) bool {
	if pattern == "" {
		return true
	}

	// Check for parenthesized argument pattern: "ToolName(argPattern)"
	if parenIdx := strings.Index(pattern, "("); parenIdx >= 0 {
		// Extract tool name portion before the paren
		patternTool := pattern[:parenIdx]
		// Extract the actual tool name (before any paren in toolName)
		actualTool := toolName
		if tIdx := strings.Index(toolName, "("); tIdx >= 0 {
			actualTool = toolName[:tIdx]
		}

		// Tool names must match case-insensitively
		if !strings.EqualFold(patternTool, actualTool) {
			return false
		}

		// Extract argument patterns
		patternEnd := strings.LastIndex(pattern, ")")
		if patternEnd < 0 {
			patternEnd = len(pattern)
		}
		argPattern := pattern[parenIdx+1 : patternEnd]

		// Extract actual arguments
		toolParenIdx := strings.Index(toolName, "(")
		if toolParenIdx < 0 {
			// No arguments in tool name -- only match if arg pattern is empty or wildcard
			matched, _ := filepath.Match(argPattern, "")
			return matched
		}
		toolParenEnd := strings.LastIndex(toolName, ")")
		if toolParenEnd < 0 {
			toolParenEnd = len(toolName)
		}
		actualArgs := toolName[toolParenIdx+1 : toolParenEnd]

		matched, _ := filepath.Match(argPattern, actualArgs)
		return matched
	}

	// Case-insensitive exact match
	if strings.EqualFold(pattern, toolName) {
		return true
	}

	// Glob match (case-insensitive)
	matched, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(toolName))
	return matched
}

// FilterMatchers collects all HookCommands from matchers where the Matcher
// pattern matches the given tool name. The returned hooks are in order of
// the matchers slice, then hooks within each matcher.
func FilterMatchers(matchers []HookMatcher, toolName string) []HookCommand {
	var result []HookCommand
	for _, m := range matchers {
		if MatchesToolName(m.Matcher, toolName) {
			result = append(result, m.Hooks...)
		}
	}
	return result
}
