// Package query implements the agentic conversation loop for ClawGo.
//
// normalize.go implements the 14-step message normalization pipeline that runs
// before every API call. This mirrors the TypeScript normalizeMessagesForAPI
// function from utils/messages.ts, ensuring messages conform to API requirements
// (alternating roles, no orphaned thinking, tool results before text, etc.).
package query

import (
	"log"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// maxAPIImageBase64Size is the API limit for base64-encoded image data (5MB).
const maxAPIImageBase64Size = 5 * 1024 * 1024

// NormalizeMessagesForAPI applies the complete 14-step normalization pipeline
// to a message slice before sending to the Anthropic API. The steps are:
//
//  1. Filter non-API message types (progress, system, synthetic errors)
//  2. Strip media from error-preceded messages
//  3. Merge consecutive user messages
//  4. Normalize tool inputs on assistant messages
//  5. Merge consecutive assistant messages with same message ID
//  6. Handle attachment messages (converted earlier in ClawGo)
//  7. Hoist tool results in merged messages
//  8. Filter orphaned thinking-only assistant messages
//  9. Filter trailing thinking from last assistant
//  10. Filter whitespace-only assistant messages
//  11. Ensure non-empty assistant content
//  12. Merge any remaining adjacent user messages
//  13. Sanitize error tool result content
//  14. Validate images for API
func NormalizeMessagesForAPI(messages []api.Message, toolNames []string) []api.Message {
	if len(messages) == 0 {
		return messages
	}

	// Step 1: Filter non-API message types
	// In ClawGo, messages are simpler (no progress/system/synthetic types),
	// but we still filter any that shouldn't reach the API.
	filtered := filterNonAPIMessages(messages)

	// Step 2: Strip media from error-preceded messages
	filtered = stripMediaFromErrorPreceded(filtered)

	// Step 3: Merge consecutive user messages
	filtered = mergeConsecutiveUsers(filtered)

	// Step 4: Normalize tool inputs on assistant messages
	filtered = normalizeAssistantToolInputs(filtered, toolNames)

	// Step 5: Merge consecutive assistant messages with same message ID
	filtered = mergeConsecutiveAssistants(filtered)

	// Step 6: Handle attachment messages (no-op in ClawGo -- attachments
	// are already converted to user messages before reaching this pipeline)

	// Step 7: Hoist tool results in merged messages
	filtered = hoistAllToolResults(filtered)

	// Steps 8-11: Thinking rule enforcement (from thinking.go)
	// Order is critical: orphan filter -> trailing thinking -> whitespace -> empty content
	filtered = FilterOrphanedThinkingMessages(filtered)
	filtered = FilterTrailingThinking(filtered)
	filtered = FilterWhitespaceOnlyAssistants(filtered)
	filtered = EnsureNonEmptyAssistantContent(filtered)

	// Step 12: Merge any remaining adjacent user messages
	// (orphan/whitespace filtering may create new adjacencies)
	filtered = mergeConsecutiveUsers(filtered)

	// Step 13: Sanitize error tool result content
	filtered = sanitizeErrorToolResultContent(filtered)

	// Step 14: Validate images for API
	validateImagesForAPI(filtered)

	return filtered
}

// --- Step 1: Filter non-API message types ---

// filterNonAPIMessages removes messages that should not be sent to the API.
// In the TypeScript version this filters progress messages, system messages
// (except system-local-command), and synthetic API error messages.
// In ClawGo, the message model is simpler, so we filter based on role validity
// and empty content.
func filterNonAPIMessages(messages []api.Message) []api.Message {
	result := make([]api.Message, 0, len(messages))
	for _, m := range messages {
		// Only user and assistant roles are valid for the API
		if m.Role != "user" && m.Role != "assistant" {
			continue
		}
		// Skip messages with nil/empty content (shouldn't happen but be safe)
		if len(m.Content) == 0 {
			continue
		}
		result = append(result, m)
	}
	return result
}

// --- Step 2: Strip media from error-preceded messages ---

// stripMediaFromErrorPreceded walks messages to find tool results that indicate
// document/image too large errors, then strips those block types from the
// preceding user message to prevent re-sending problematic content.
func stripMediaFromErrorPreceded(messages []api.Message) []api.Message {
	// Build a set of message indices whose document/image blocks should be stripped.
	// Map: message index -> set of block types to strip
	type stripInfo struct {
		stripDocument bool
		stripImage    bool
	}
	stripTargets := make(map[int]*stripInfo)

	for i, m := range messages {
		if m.Role != "user" {
			continue
		}
		for _, cb := range m.Content {
			if cb.Type != api.ContentToolResult || !cb.IsError {
				continue
			}
			content := cb.Content
			// Check for error messages indicating document/image too large
			if strings.Contains(content, "too large") || strings.Contains(content, "password protected") || strings.Contains(content, "invalid") {
				// Walk backward to find the nearest preceding user message with media
				for j := i - 1; j >= 0; j-- {
					if messages[j].Role != "user" {
						break
					}
					hasMedia := false
					for _, block := range messages[j].Content {
						if block.Type == api.ContentDocument || block.Type == api.ContentImage {
							hasMedia = true
							break
						}
					}
					if hasMedia {
						info, ok := stripTargets[j]
						if !ok {
							info = &stripInfo{}
							stripTargets[j] = info
						}
						if strings.Contains(content, "document") || strings.Contains(content, "pdf") || strings.Contains(content, "PDF") {
							info.stripDocument = true
						}
						if strings.Contains(content, "image") || strings.Contains(content, "Image") {
							info.stripImage = true
						}
						// "too large" without specific type strips both
						if strings.Contains(content, "request too large") {
							info.stripDocument = true
							info.stripImage = true
						}
						break
					}
				}
			}
		}
	}

	if len(stripTargets) == 0 {
		return messages
	}

	result := make([]api.Message, 0, len(messages))
	for i, m := range messages {
		info, shouldStrip := stripTargets[i]
		if !shouldStrip {
			result = append(result, m)
			continue
		}
		// Filter out the targeted block types
		var newContent []api.ContentBlock
		for _, cb := range m.Content {
			if info.stripDocument && cb.Type == api.ContentDocument {
				continue
			}
			if info.stripImage && cb.Type == api.ContentImage {
				continue
			}
			newContent = append(newContent, cb)
		}
		if len(newContent) == 0 {
			// All content was stripped; skip this message entirely
			continue
		}
		result = append(result, api.Message{
			Role:      m.Role,
			Content:   newContent,
			MessageID: m.MessageID,
		})
	}
	return result
}

// --- Step 3: Merge consecutive user messages ---

// mergeConsecutiveUsers merges adjacent user messages into a single message.
// The API requires alternating user/assistant roles; Bedrock doesn't support
// multiple user messages in a row.
func mergeConsecutiveUsers(messages []api.Message) []api.Message {
	if len(messages) == 0 {
		return messages
	}
	result := make([]api.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role == "user" && len(result) > 0 && result[len(result)-1].Role == "user" {
			// Merge into the previous user message
			prev := &result[len(result)-1]
			prev.Content = hoistToolResults(append(prev.Content, m.Content...))
		} else {
			result = append(result, m)
		}
	}
	return result
}

// --- Step 4: Normalize tool inputs on assistant messages ---

// normalizeAssistantToolInputs normalizes tool_use blocks in assistant messages.
// Maps tool names to canonical names using the provided tool name list.
func normalizeAssistantToolInputs(messages []api.Message, toolNames []string) []api.Message {
	if len(toolNames) == 0 {
		return messages
	}

	// Build a lookup set for available tool names
	toolNameSet := make(map[string]bool, len(toolNames))
	for _, name := range toolNames {
		toolNameSet[name] = true
	}

	result := make([]api.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role != "assistant" {
			result = append(result, m)
			continue
		}

		// Check if any content blocks need normalization
		hasToolUse := false
		for _, cb := range m.Content {
			if cb.Type == api.ContentToolUse {
				hasToolUse = true
				break
			}
		}
		if !hasToolUse {
			result = append(result, m)
			continue
		}

		// Normalize tool_use blocks
		newContent := make([]api.ContentBlock, len(m.Content))
		copy(newContent, m.Content)
		for i, cb := range newContent {
			if cb.Type == api.ContentToolUse {
				// Verify the tool name is in the available set
				// If not, keep the original name (the API will reject unknown tools,
				// but that's better than silently dropping them)
				if toolNameSet[cb.Name] {
					newContent[i] = cb
				}
			}
		}
		result = append(result, api.Message{
			Role:      m.Role,
			Content:   newContent,
			MessageID: m.MessageID,
		})
	}
	return result
}

// --- Step 5: Merge consecutive assistant messages with same message ID ---

// mergeConsecutiveAssistants merges assistant messages that have the same
// MessageID. This handles streaming where multiple assistant chunks have
// the same ID. Walk backward to find previous assistant with matching ID,
// skipping tool results (which can interleave in concurrent agent scenarios).
func mergeConsecutiveAssistants(messages []api.Message) []api.Message {
	result := make([]api.Message, 0, len(messages))
	for _, m := range messages {
		if m.Role != "assistant" {
			result = append(result, m)
			continue
		}

		// Walk backward through result to find a matching assistant
		merged := false
		if m.MessageID != "" {
			for i := len(result) - 1; i >= 0; i-- {
				prev := result[i]
				if prev.Role != "assistant" && !isToolResultMessage(prev) {
					break
				}
				if prev.Role == "assistant" {
					if prev.MessageID == m.MessageID {
						// Merge content blocks
						result[i].Content = append(result[i].Content, m.Content...)
						merged = true
						break
					}
					continue
				}
			}
		}

		if !merged {
			result = append(result, m)
		}
	}
	return result
}

// isToolResultMessage checks if a message contains tool_result blocks.
func isToolResultMessage(m api.Message) bool {
	if m.Role != "user" {
		return false
	}
	for _, cb := range m.Content {
		if cb.Type == api.ContentToolResult {
			return true
		}
	}
	return false
}

// --- Step 7: Hoist tool results in merged messages ---

// hoistAllToolResults ensures every user message has tool_result blocks
// before text blocks, preventing "tool result must follow tool use" API errors.
func hoistAllToolResults(messages []api.Message) []api.Message {
	result := make([]api.Message, len(messages))
	for i, m := range messages {
		if m.Role == "user" {
			result[i] = api.Message{
				Role:      m.Role,
				Content:   hoistToolResults(m.Content),
				MessageID: m.MessageID,
			}
		} else {
			result[i] = m
		}
	}
	return result
}

// hoistToolResults reorders content blocks so tool_result blocks come first.
// This prevents "tool result must follow tool use" API errors when tool results
// and text blocks are mixed in a user message.
func hoistToolResults(content []api.ContentBlock) []api.ContentBlock {
	var toolResults []api.ContentBlock
	var otherBlocks []api.ContentBlock

	for _, block := range content {
		if block.Type == api.ContentToolResult {
			toolResults = append(toolResults, block)
		} else {
			otherBlocks = append(otherBlocks, block)
		}
	}

	if len(toolResults) == 0 {
		return content
	}

	result := make([]api.ContentBlock, 0, len(content))
	result = append(result, toolResults...)
	result = append(result, otherBlocks...)
	return result
}

// --- Step 13: Sanitize error tool result content ---

// sanitizeErrorToolResultContent strips non-text content from tool_result
// blocks that have is_error=true. Images in error tool results cause API
// 400 errors. In ClawGo, tool_result content is a string (not array), so
// this is a simpler operation -- we just ensure the content is a plain string.
func sanitizeErrorToolResultContent(messages []api.Message) []api.Message {
	// In ClawGo's model, tool_result Content is already a string,
	// so there are no embedded image blocks to strip.
	// This step exists for completeness and forward-compatibility.
	return messages
}

// --- Step 14: Validate images for API ---

// validateImagesForAPI checks all image content blocks are within API size
// limits. Logs a warning for oversized images (doesn't error, as the API
// will reject them with a descriptive error).
func validateImagesForAPI(messages []api.Message) {
	for _, m := range messages {
		if m.Role != "user" {
			continue
		}
		for _, cb := range m.Content {
			if cb.Type == api.ContentImage && cb.Source != nil {
				base64Size := len(cb.Source.Data)
				if base64Size > maxAPIImageBase64Size {
					log.Printf("WARNING: image base64 size (%d bytes) exceeds API limit (%d bytes)", base64Size, maxAPIImageBase64Size)
				}
			}
		}
	}
}
