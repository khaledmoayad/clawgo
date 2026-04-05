// Package query implements thinking rule enforcement for the query engine.
//
// The Three Rules of Thinking (from Claude Code query.ts:151-163):
//  1. A message that contains a thinking or redacted_thinking block must be
//     part of a query whose max_thinking_length > 0
//  2. A thinking block may not be the last content block in the last
//     assistant message
//  3. Thinking blocks must be preserved for the duration of an assistant
//     trajectory (a single turn, or if that turn includes a tool_use block
//     then also its subsequent tool_result and the following assistant message)
//
// This file implements the normalization pipeline that enforces these rules
// before sending messages to the API, matching Claude Code's
// utils/messages.ts functions.
package query

import (
	"strings"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// isThinkingBlock returns true if the content block is a thinking or
// redacted_thinking block.
func isThinkingBlock(cb api.ContentBlock) bool {
	return cb.Type == api.ContentThinking || cb.Type == api.ContentRedactedThinking
}

// HasThinkingContent checks if a message contains any thinking or
// redacted_thinking content blocks.
func HasThinkingContent(msg api.Message) bool {
	for _, cb := range msg.Content {
		if isThinkingBlock(cb) {
			return true
		}
	}
	return false
}

// FilterTrailingThinking removes trailing thinking/redacted_thinking blocks
// from the last assistant message. The API does not allow assistant messages
// to end with thinking blocks (Rule 2).
//
// Only the LAST assistant message is affected -- earlier messages are
// preserved. If all blocks in the last assistant are thinking, a placeholder
// text block is inserted.
//
// Matches Claude Code's filterTrailingThinkingFromLastAssistant.
func FilterTrailingThinking(messages []api.Message) []api.Message {
	if len(messages) == 0 {
		return messages
	}

	lastIdx := len(messages) - 1
	last := messages[lastIdx]
	if last.Role != "assistant" || len(last.Content) == 0 {
		return messages
	}

	// Check if the last content block is thinking
	lastBlock := last.Content[len(last.Content)-1]
	if !isThinkingBlock(lastBlock) {
		return messages
	}

	// Walk backwards to find the last non-thinking block
	lastValidIdx := len(last.Content) - 1
	for lastValidIdx >= 0 && isThinkingBlock(last.Content[lastValidIdx]) {
		lastValidIdx--
	}

	// Build filtered content
	var filteredContent []api.ContentBlock
	if lastValidIdx < 0 {
		// All blocks were thinking -- insert placeholder
		filteredContent = []api.ContentBlock{
			{Type: api.ContentText, Text: "[No message content]"},
		}
	} else {
		filteredContent = make([]api.ContentBlock, lastValidIdx+1)
		copy(filteredContent, last.Content[:lastValidIdx+1])
	}

	// Create new slice with modified last message
	result := make([]api.Message, len(messages))
	copy(result, messages)
	result[lastIdx] = api.Message{
		Role:    last.Role,
		Content: filteredContent,
	}
	return result
}

// FilterOrphanedThinkingMessages removes assistant messages that contain
// ONLY thinking/redacted_thinking blocks and have no sibling message with
// non-thinking content.
//
// During streaming, each content block may be yielded as a separate message.
// After compaction or resume, thinking-only messages can become orphaned
// (separated from their non-thinking sibling). These orphans cause
// "thinking blocks cannot be modified" API errors.
//
// Matches Claude Code's filterOrphanedThinkingOnlyMessages.
func FilterOrphanedThinkingMessages(messages []api.Message) []api.Message {
	if len(messages) == 0 {
		return messages
	}

	// In the Go version, we don't have message IDs like the TS version,
	// so we simply remove any assistant message that contains only thinking
	// blocks. This is a conservative approach that works correctly because
	// in Go the messages are already merged by the time they reach here.
	filtered := make([]api.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != "assistant" {
			filtered = append(filtered, msg)
			continue
		}

		if len(msg.Content) == 0 {
			filtered = append(filtered, msg)
			continue
		}

		// Check if ALL content blocks are thinking
		allThinking := true
		for _, cb := range msg.Content {
			if !isThinkingBlock(cb) {
				allThinking = false
				break
			}
		}

		if allThinking {
			// Orphaned thinking-only message -- remove it
			continue
		}

		filtered = append(filtered, msg)
	}

	return filtered
}

// EnsureNonEmptyAssistantContent adds a placeholder text block to any
// non-final assistant message that has empty content.
//
// The API requires "all messages must have non-empty content except for
// the optional final assistant message". This can happen when the model
// returns an empty content array, or after thinking stripping leaves a
// message with no content.
//
// The final assistant message is left as-is since it is allowed to be
// empty (for prefill).
//
// Matches Claude Code's ensureNonEmptyAssistantContent.
func EnsureNonEmptyAssistantContent(messages []api.Message) []api.Message {
	if len(messages) == 0 {
		return messages
	}

	hasChanges := false
	result := make([]api.Message, len(messages))
	copy(result, messages)

	for i, msg := range result {
		// Skip non-assistant messages
		if msg.Role != "assistant" {
			continue
		}

		// Skip the final message (allowed to be empty for prefill)
		if i == len(result)-1 {
			continue
		}

		// Check if content is empty
		if len(msg.Content) == 0 {
			hasChanges = true
			result[i] = api.Message{
				Role: msg.Role,
				Content: []api.ContentBlock{
					{Type: api.ContentText, Text: "[No content]"},
				},
			}
		}
	}

	if !hasChanges {
		return messages
	}
	return result
}

// hasOnlyWhitespaceTextContent returns true if all content blocks are text
// blocks with only whitespace. Returns false for empty content or if any
// non-text block exists.
func hasOnlyWhitespaceTextContent(content []api.ContentBlock) bool {
	if len(content) == 0 {
		return false
	}

	for _, cb := range content {
		// Non-text blocks (tool_use, thinking, etc.) mean the message is valid
		if cb.Type != api.ContentText {
			return false
		}
		// Text block with non-whitespace content means the message is valid
		if strings.TrimSpace(cb.Text) != "" {
			return false
		}
	}

	// All blocks are text with only whitespace
	return true
}

// FilterWhitespaceOnlyAssistants removes assistant messages whose text
// content is whitespace-only. After removing such messages, adjacent user
// messages are merged to maintain the required alternating user/assistant
// role pattern.
//
// The API requires "text content blocks must contain non-whitespace text".
// This can happen when the model outputs whitespace before a thinking
// block but the user cancels mid-stream.
//
// Matches Claude Code's filterWhitespaceOnlyAssistantMessages.
func FilterWhitespaceOnlyAssistants(messages []api.Message) []api.Message {
	hasChanges := false

	filtered := make([]api.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != "assistant" {
			filtered = append(filtered, msg)
			continue
		}

		if len(msg.Content) == 0 {
			filtered = append(filtered, msg)
			continue
		}

		if hasOnlyWhitespaceTextContent(msg.Content) {
			hasChanges = true
			continue // Remove this message
		}

		filtered = append(filtered, msg)
	}

	if !hasChanges {
		return messages
	}

	// Merge adjacent user messages that resulted from removing assistant
	// messages between them. The API requires alternating user/assistant.
	merged := make([]api.Message, 0, len(filtered))
	for _, msg := range filtered {
		if len(merged) > 0 && msg.Role == "user" && merged[len(merged)-1].Role == "user" {
			// Merge into previous user message
			prev := &merged[len(merged)-1]
			prev.Content = append(prev.Content, msg.Content...)
		} else {
			merged = append(merged, msg)
		}
	}

	return merged
}

// EnforceThinkingRules applies the full thinking normalization pipeline
// in the correct order, preparing messages for the API.
//
// The order is critical (per Claude Code comments at messages.ts:2313-2320):
//  1. FilterOrphanedThinkingMessages -- remove thinking-only orphans
//  2. FilterTrailingThinking -- strip trailing thinking from last assistant
//  3. FilterWhitespaceOnlyAssistants -- remove whitespace-only assistants
//  4. EnsureNonEmptyAssistantContent -- add placeholders for empty assistants
//
// Order matters: strip trailing thinking FIRST, then filter whitespace.
// The reverse has a bug where [text("\n\n"), thinking("...")] survives the
// whitespace filter, then thinking stripping leaves [text("\n\n")] which
// the API rejects.
func EnforceThinkingRules(messages []api.Message) []api.Message {
	result := FilterOrphanedThinkingMessages(messages)
	result = FilterTrailingThinking(result)
	result = FilterWhitespaceOnlyAssistants(result)
	result = EnsureNonEmptyAssistantContent(result)
	return result
}
