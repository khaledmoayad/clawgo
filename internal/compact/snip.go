package compact

import (
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/api"
)

const (
	// SnipRecencyWindow is the number of recent messages exempt from snipping.
	// Messages newer than this boundary are never touched. This is larger
	// than MicroCompact's window (10) because snip is more aggressive.
	SnipRecencyWindow = 20

	// SnipMinResultLength is the minimum character length for a tool result
	// to be snip-eligible. Short results are kept as-is. This is higher
	// than MicroCompact's threshold (500) because snip also removes the
	// corresponding tool_use input.
	SnipMinResultLength = 1000

	// SnipReplacementMessage is the placeholder that replaces snipped tool results.
	SnipReplacementMessage = "[Tool result snipped for context management]"

	// SnipInputReplacementMessage replaces the tool_use input when its result is snipped.
	SnipInputReplacementMessage = "[Input snipped]"
)

// snipCompactableTools lists tools whose results can be snipped.
// Same set as MicroCompact's CompactableTools.
var snipCompactableTools = map[string]bool{
	"Read":      true,
	"Bash":      true,
	"Grep":      true,
	"Glob":      true,
	"WebSearch": true,
	"WebFetch":  true,
	"Edit":      true,
	"Write":     true,
}

// IsSnipEnabled returns true if snip compaction is enabled.
// Always true for now; in the TS codebase this is gated behind the
// HISTORY_SNIP feature flag.
func IsSnipEnabled() bool {
	return true
}

// IsSnipRuntimeEnabled checks if snip is enabled at runtime.
// For now this delegates to IsSnipEnabled; in the future this may
// check environment variables or feature flags.
func IsSnipRuntimeEnabled() bool {
	return IsSnipEnabled()
}

// SnipConversation prunes old tool results from the conversation to save
// context. More aggressive than MicroCompact:
//   - Larger recency window (20 vs 10)
//   - Higher minimum length threshold (1000 vs 500)
//   - Also snips the tool_use input, not just the result
//   - More aggressive replacement message
//
// Called before auto-compact to reduce what needs summarizing, matching
// Claude Code's snipCompact.ts behavior.
func SnipConversation(messages []api.Message, model string) []api.Message {
	if len(messages) <= SnipRecencyWindow {
		return messages
	}

	// Phase 1: Build a map of tool_use IDs from compactable tools
	// so we can match tool_result blocks to their originating tool.
	compactableIDs := make(map[string]bool)
	for _, m := range messages {
		for _, cb := range m.Content {
			if cb.Type == api.ContentToolUse && snipCompactableTools[cb.Name] {
				compactableIDs[cb.ID] = true
			}
		}
	}

	// Phase 2: Determine cutoff -- messages before this index are eligible
	cutoff := len(messages) - SnipRecencyWindow

	// Phase 3: Identify which tool_use IDs have been snipped (their results
	// exceeded the threshold) so we can also snip the corresponding input.
	snippedIDs := make(map[string]bool)
	for i := 0; i < cutoff; i++ {
		for _, cb := range messages[i].Content {
			if cb.Type == api.ContentToolResult &&
				compactableIDs[cb.ToolUseID] &&
				len(cb.Content) > SnipMinResultLength {
				snippedIDs[cb.ToolUseID] = true
			}
		}
	}

	if len(snippedIDs) == 0 {
		return messages
	}

	// Phase 4: Build result, snipping both tool_result content AND
	// corresponding tool_use input for maximum context savings.
	result := make([]api.Message, len(messages))
	for i, m := range messages {
		if i >= cutoff {
			// Recent messages are kept as-is
			result[i] = m
			continue
		}

		// Check if any content blocks need snipping
		needsCopy := false
		for _, cb := range m.Content {
			if cb.Type == api.ContentToolResult && snippedIDs[cb.ToolUseID] {
				needsCopy = true
				break
			}
			if cb.Type == api.ContentToolUse && snippedIDs[cb.ID] {
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
			if cb.Type == api.ContentToolResult && snippedIDs[cb.ToolUseID] {
				newBlocks[j].Content = SnipReplacementMessage
			}
			if cb.Type == api.ContentToolUse && snippedIDs[cb.ID] {
				newBlocks[j].Input = []byte(fmt.Sprintf(`{"input":"%s"}`, SnipInputReplacementMessage))
			}
		}
		result[i] = api.Message{
			Role:    m.Role,
			Content: newBlocks,
		}
	}

	return result
}

// AppendMessageTag appends an [id:{messageID}] tag to the last text content
// block of a message. Used by snip to track which messages have been snipped.
// Matches Claude Code's appendMessageTagToUserMessage.
func AppendMessageTag(msg api.Message, messageID string) api.Message {
	tag := fmt.Sprintf(" [id:%s]", messageID)

	// Find the last text content block
	lastTextIdx := -1
	for i, cb := range msg.Content {
		if cb.Type == api.ContentText {
			lastTextIdx = i
		}
	}

	if lastTextIdx < 0 {
		// No text block found; nothing to tag
		return msg
	}

	// Create a copy with the tag appended
	newBlocks := make([]api.ContentBlock, len(msg.Content))
	copy(newBlocks, msg.Content)
	newBlocks[lastTextIdx].Text = strings.TrimRight(newBlocks[lastTextIdx].Text, " ") + tag

	return api.Message{
		Role:    msg.Role,
		Content: newBlocks,
	}
}
