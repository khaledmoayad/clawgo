package compact

import (
	"github.com/khaledmoayad/clawgo/internal/api"
)

const (
	// microCompactRecencyWindow is the number of recent messages that are
	// exempt from micro-compaction. Only tool results older than this
	// window are eligible for clearing.
	microCompactRecencyWindow = 10

	// microCompactMinLength is the minimum character length for a tool result
	// to be eligible for micro-compaction. Short results are kept as-is.
	microCompactMinLength = 500
)

// CompactableTools lists tools whose results can be micro-compacted
// (replaced with a short placeholder when they are old enough).
var CompactableTools = map[string]bool{
	"Read":      true,
	"Bash":      true,
	"Grep":      true,
	"Glob":      true,
	"WebSearch": true,
	"WebFetch":  true,
	"Edit":      true,
	"Write":     true,
}

// IsCompactableTool returns true if the named tool's results are eligible
// for micro-compaction.
func IsCompactableTool(name string) bool {
	return CompactableTools[name]
}

// MicroCompact scans messages for old tool_result blocks from compactable
// tools and replaces their content with a short placeholder to save context.
//
// Only tool results older than the most recent microCompactRecencyWindow
// messages are affected, and only if the result content exceeds
// microCompactMinLength characters. The original slice is not mutated;
// a new slice is returned with modified copies.
func MicroCompact(messages []api.Message, model string) []api.Message {
	if len(messages) <= microCompactRecencyWindow {
		return messages
	}

	// Build a set of tool_use IDs from compactable tools across all messages
	// so we can match tool_result blocks to their originating tool
	compactableIDs := make(map[string]bool)
	for _, m := range messages {
		for _, cb := range m.Content {
			if cb.Type == api.ContentToolUse && IsCompactableTool(cb.Name) {
				compactableIDs[cb.ID] = true
			}
		}
	}

	// Determine cutoff: messages before this index are eligible for compaction
	cutoff := len(messages) - microCompactRecencyWindow

	result := make([]api.Message, len(messages))
	for i, m := range messages {
		if i >= cutoff {
			// Recent messages are kept as-is
			result[i] = m
			continue
		}

		// Check if any content blocks need compaction
		needsCopy := false
		for _, cb := range m.Content {
			if cb.Type == api.ContentToolResult &&
				compactableIDs[cb.ToolUseID] &&
				len(cb.Content) > microCompactMinLength {
				needsCopy = true
				break
			}
		}

		if !needsCopy {
			result[i] = m
			continue
		}

		// Create a modified copy of the message
		newBlocks := make([]api.ContentBlock, len(m.Content))
		copy(newBlocks, m.Content)
		for j, cb := range newBlocks {
			if cb.Type == api.ContentToolResult &&
				compactableIDs[cb.ToolUseID] &&
				len(cb.Content) > microCompactMinLength {
				newBlocks[j].Content = "[Content cleared to save context]"
			}
		}
		result[i] = api.Message{
			Role:    m.Role,
			Content: newBlocks,
		}
	}

	return result
}
