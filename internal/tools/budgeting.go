package tools

import (
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// Tool result size constants matching Claude Code's constants/toolLimits.ts.
const (
	// DefaultMaxResultSizeChars is the per-tool default maximum size in characters
	// before truncation. Individual tools may declare a lower limit, but this
	// acts as a system-wide cap.
	DefaultMaxResultSizeChars = 50_000

	// MaxToolResultTokens is the maximum size for tool results in tokens.
	MaxToolResultTokens = 100_000

	// BytesPerToken is the conservative estimate for converting bytes to tokens.
	BytesPerToken = 4

	// MaxToolResultBytesPerMessage is the maximum size for tool results in bytes
	// (derived from token limit). Approximately 400KB.
	MaxToolResultBytesPerMessage = MaxToolResultTokens * BytesPerToken

	// MaxToolResultsPerMessageChars is the default maximum aggregate size in
	// characters for tool_result blocks within a single user message. When a
	// message's blocks together exceed this, the largest blocks are truncated.
	// Messages are evaluated independently.
	MaxToolResultsPerMessageChars = 200_000

	// PersistedOutputTag is the XML tag used to wrap persisted output messages.
	PersistedOutputTag = "<persisted-output>"

	// PersistedOutputClosingTag closes the persisted output wrapper.
	PersistedOutputClosingTag = "</persisted-output>"

	// ToolResultClearedMessage is the message used when tool result content
	// was cleared to stay within budget.
	ToolResultClearedMessage = "[Old tool result content cleared]"
)

// truncationMarker returns the truncation notice appended to oversized results.
func truncationMarker(shown, total int) string {
	return fmt.Sprintf("\n\n[Output truncated. Showing first %d of %d characters]", shown, total)
}

// messageBudgetMarker is the notice when a result is cleared for per-message budget.
const messageBudgetMarker = "[Tool result cleared to stay within message budget]"

// ToolResultSizer provides per-tool result size limits. If a tool is not found
// in the registry or has no custom limit, DefaultMaxResultSizeChars is used.
// A limit of 0 means unlimited (used by tools like the query engine itself).
type ToolResultSizer interface {
	// MaxResultSizeChars returns the maximum result size for the named tool.
	// Returns DefaultMaxResultSizeChars if the tool has no custom limit.
	MaxResultSizeChars(toolName string) int
}

// defaultSizer always returns DefaultMaxResultSizeChars.
type defaultSizer struct{}

func (defaultSizer) MaxResultSizeChars(_ string) int {
	return DefaultMaxResultSizeChars
}

// ApplyToolResultBudget enforces per-tool and per-message size limits on tool
// results in a message slice. It does NOT mutate the input -- a new slice is
// returned if any modifications were made.
//
// For each user message containing tool_result blocks:
//  1. Per-tool limit: if a single tool result exceeds its maxResultSizeChars
//     (default 50k), it is truncated with a marker.
//  2. Per-message limit: if the total tool_result content in a single message
//     exceeds MaxToolResultsPerMessageChars (200k), the largest results are
//     truncated until the total fits.
//
// This matches Claude Code's applyToolResultBudget / enforceToolResultBudget
// behavior (simplified -- Go version does synchronous truncation rather than
// persisting to disk, since Go doesn't need the async file-persist path).
func ApplyToolResultBudget(messages []api.Message, sizer ToolResultSizer) []api.Message {
	if sizer == nil {
		sizer = defaultSizer{}
	}

	modified := false
	result := make([]api.Message, len(messages))

	for i, msg := range messages {
		if msg.Role != "user" {
			result[i] = msg
			continue
		}

		// Check if this message has any tool_result blocks
		hasToolResults := false
		for _, cb := range msg.Content {
			if cb.Type == api.ContentToolResult {
				hasToolResults = true
				break
			}
		}

		if !hasToolResults {
			result[i] = msg
			continue
		}

		// Phase 1: Per-tool truncation
		newBlocks := make([]api.ContentBlock, len(msg.Content))
		msgModified := false
		for j, cb := range msg.Content {
			if cb.Type != api.ContentToolResult {
				newBlocks[j] = cb
				continue
			}

			content := cb.Content
			limit := sizer.MaxResultSizeChars(cb.Name)

			// 0 means unlimited
			if limit > 0 && len(content) > limit {
				truncated := content[:limit] + truncationMarker(limit, len(content))
				newBlocks[j] = api.ContentBlock{
					Type:      cb.Type,
					ToolUseID: cb.ToolUseID,
					Content:   truncated,
					IsError:   cb.IsError,
					Name:      cb.Name,
				}
				msgModified = true
			} else {
				newBlocks[j] = cb
			}
		}

		// Phase 2: Per-message budget enforcement
		totalSize := 0
		for _, cb := range newBlocks {
			if cb.Type == api.ContentToolResult {
				totalSize += len(cb.Content)
			}
		}

		if totalSize > MaxToolResultsPerMessageChars {
			newBlocks = enforceMessageBudget(newBlocks, MaxToolResultsPerMessageChars)
			msgModified = true
		}

		if msgModified {
			modified = true
			result[i] = api.Message{
				Role:    msg.Role,
				Content: newBlocks,
			}
		} else {
			result[i] = msg
		}
	}

	if !modified {
		return messages // no changes, return original slice
	}
	return result
}

// enforceMessageBudget truncates the largest tool_result blocks in a message
// until the total size is within budget. Blocks are cleared largest-first.
func enforceMessageBudget(blocks []api.ContentBlock, budget int) []api.ContentBlock {
	// Build index of tool_result blocks with their sizes
	type indexedBlock struct {
		index int
		size  int
	}

	var toolResults []indexedBlock
	totalSize := 0
	for i, cb := range blocks {
		if cb.Type == api.ContentToolResult {
			size := len(cb.Content)
			toolResults = append(toolResults, indexedBlock{index: i, size: size})
			totalSize += size
		}
	}

	if totalSize <= budget {
		return blocks
	}

	// Sort by size descending (largest first to clear)
	// Simple selection sort -- number of tool results per message is small
	for i := 0; i < len(toolResults); i++ {
		maxIdx := i
		for j := i + 1; j < len(toolResults); j++ {
			if toolResults[j].size > toolResults[maxIdx].size {
				maxIdx = j
			}
		}
		toolResults[i], toolResults[maxIdx] = toolResults[maxIdx], toolResults[i]
	}

	// Clear largest blocks until under budget
	result := make([]api.ContentBlock, len(blocks))
	copy(result, blocks)

	for _, tr := range toolResults {
		if totalSize <= budget {
			break
		}

		// Replace with budget message
		originalSize := tr.size
		replacement := messageBudgetMarker
		result[tr.index] = api.ContentBlock{
			Type:      api.ContentToolResult,
			ToolUseID: blocks[tr.index].ToolUseID,
			Content:   replacement,
			IsError:   blocks[tr.index].IsError,
			Name:      blocks[tr.index].Name,
		}
		totalSize -= originalSize
		totalSize += len(replacement)
	}

	return result
}
