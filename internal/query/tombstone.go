package query

import (
	"github.com/khaledmoayad/clawgo/internal/api"
)

// TombstoneMessage records that an orphaned assistant message was cleaned up
// during streaming fallback or error recovery.
type TombstoneMessage struct {
	// OriginalUUID identifies the assistant message that was tombstoned.
	OriginalUUID string

	// Reason describes why the message was tombstoned.
	// Values: "fallback", "streaming_error", "user_interrupted"
	Reason string
}

// YieldMissingToolResultBlocks creates synthetic error tool results for every
// tool_use block found in the given assistant messages. This is used when a
// streaming response is interrupted (fallback, error, abort) and tool calls
// were issued but never executed.
//
// Each tool_use block gets a corresponding tool_result with is_error=true and
// the provided errorMessage, matching the TS yieldMissingToolResultBlocks
// function in query.ts:123-149.
func YieldMissingToolResultBlocks(assistantMessages []api.Message, errorMessage string) []api.ToolResultEntry {
	var results []api.ToolResultEntry

	for _, msg := range assistantMessages {
		if msg.Role != "assistant" {
			continue
		}
		for _, block := range msg.Content {
			if block.Type != api.ContentToolUse {
				continue
			}
			results = append(results, api.ToolResultEntry{
				ToolUseID: block.ID,
				Content:   errorMessage,
				IsError:   true,
			})
		}
	}

	return results
}

// tombstoneErrorContent is the error message used for synthetic tool results
// when tombstoning orphaned tool calls during streaming fallback.
const tombstoneErrorContent = "<tool_use_error>Error: Streaming fallback - tool execution discarded</tool_use_error>"

// TombstoneOrphanedMessages processes the message history starting from
// fromIndex, creating synthetic error tool_result entries for any assistant
// tool_use blocks that lack matching tool_result responses.
//
// This is necessary during streaming fallback: the original model may have
// issued tool_use blocks in its response, but fallback means those tools were
// never executed. The API requires every tool_use to have a corresponding
// tool_result, so we inject error results to satisfy that constraint.
//
// Returns:
//   - tombstoned: the cleaned message slice with synthetic results injected
//   - tombstones: records of which messages were tombstoned (for logging)
func TombstoneOrphanedMessages(messages []api.Message, fromIndex int) (tombstoned []api.Message, tombstones []TombstoneMessage) {
	if fromIndex < 0 || fromIndex >= len(messages) {
		return messages, nil
	}

	// First, collect all tool_use IDs that already have matching tool_result
	matchedToolUseIDs := make(map[string]bool)
	for _, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		for _, block := range msg.Content {
			if block.Type == api.ContentToolResult {
				matchedToolUseIDs[block.ToolUseID] = true
			}
		}
	}

	// Build the result slice, injecting synthetic tool_result messages
	// after each orphaned assistant message
	result := make([]api.Message, 0, len(messages))

	for i, msg := range messages {
		result = append(result, msg)

		// Only process assistant messages from fromIndex onward
		if i < fromIndex || msg.Role != "assistant" {
			continue
		}

		// Collect orphaned tool_use blocks (those without matching tool_result)
		var orphanedResults []api.ContentBlock
		for _, block := range msg.Content {
			if block.Type != api.ContentToolUse {
				continue
			}
			if matchedToolUseIDs[block.ID] {
				continue
			}
			orphanedResults = append(orphanedResults, api.ContentBlock{
				Type:      api.ContentToolResult,
				ToolUseID: block.ID,
				Content:   tombstoneErrorContent,
				IsError:   true,
			})
		}

		if len(orphanedResults) > 0 {
			// Inject a synthetic user message with the error tool results
			result = append(result, api.Message{
				Role:    "user",
				Content: orphanedResults,
			})

			tombstones = append(tombstones, TombstoneMessage{
				OriginalUUID: msg.Content[0].ID, // Use first block ID as identifier
				Reason:       "fallback",
			})
		}
	}

	return result, tombstones
}

// StripSignatureBlocks removes thinking and redacted_thinking content blocks
// from all assistant messages. Their cryptographic signatures are bound to the
// API key/model that generated them; after a credential change or model
// fallback, they are invalid and the API rejects them with a 400.
//
// Matches Claude Code's stripSignatureBlocks from utils/messages.ts.
func StripSignatureBlocks(messages []api.Message) []api.Message {
	changed := false
	result := make([]api.Message, len(messages))

	for i, msg := range messages {
		if msg.Role != "assistant" {
			result[i] = msg
			continue
		}

		var filtered []api.ContentBlock
		for _, block := range msg.Content {
			switch block.Type {
			case api.ContentThinking, api.ContentRedactedThinking:
				// Strip signature-bearing blocks
				changed = true
			default:
				filtered = append(filtered, block)
			}
		}

		if len(filtered) == len(msg.Content) {
			// No blocks were stripped
			result[i] = msg
		} else {
			result[i] = api.Message{
				Role:    msg.Role,
				Content: filtered,
			}
		}
	}

	if !changed {
		return messages
	}
	return result
}
