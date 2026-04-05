package compact

import (
	"github.com/khaledmoayad/clawgo/internal/api"
)

// CachedMicroCompactState tracks cache-aware microcompact state. Unlike
// regular MicroCompact which uses a recency window, cached microcompact
// respects prompt-cache boundaries so that clearing old tool results
// does not invalidate the cached prefix.
type CachedMicroCompactState struct {
	// LastCacheBreakpoint is the message index of the last known cache
	// boundary. Only messages before this index are eligible for clearing.
	LastCacheBreakpoint int

	// ClearedIDs tracks tool result IDs that have already been cleared
	// so they are not processed again on subsequent calls.
	ClearedIDs map[string]bool
}

// NewCachedMicroCompactState creates a fresh state with no breakpoint
// and an empty cleared-IDs set.
func NewCachedMicroCompactState() *CachedMicroCompactState {
	return &CachedMicroCompactState{
		LastCacheBreakpoint: 0,
		ClearedIDs:          make(map[string]bool),
	}
}

// APIMicroCompact performs cache-aware microcompaction on a message
// slice. It only processes messages BEFORE the cacheBreakpoint (the
// cached prefix region), clearing compactable tool results that exceed
// microCompactMinLength.
//
// Key difference from regular MicroCompact: respects cache boundaries
// so incremental clears don't invalidate the cached prefix.
//
// Returns a new message slice with cleared content; the original is
// not mutated.
func APIMicroCompact(messages []api.Message, state *CachedMicroCompactState, cacheBreakpoint int) []api.Message {
	if len(messages) == 0 || cacheBreakpoint <= 0 {
		return messages
	}

	// Build a set of compactable tool_use IDs from the entire conversation
	compactableIDs := make(map[string]bool)
	for _, m := range messages {
		for _, cb := range m.Content {
			if cb.Type == api.ContentToolUse && IsCompactableTool(cb.Name) {
				compactableIDs[cb.ID] = true
			}
		}
	}

	// Effective breakpoint is capped to message count
	effectiveBreakpoint := cacheBreakpoint
	if effectiveBreakpoint > len(messages) {
		effectiveBreakpoint = len(messages)
	}

	result := make([]api.Message, len(messages))
	copy(result, messages)

	for i := 0; i < effectiveBreakpoint; i++ {
		m := messages[i]
		needsCopy := false

		for _, cb := range m.Content {
			if cb.Type == api.ContentToolResult &&
				compactableIDs[cb.ToolUseID] &&
				!state.ClearedIDs[cb.ToolUseID] &&
				len(cb.Content) > microCompactMinLength {
				needsCopy = true
				break
			}
		}

		if !needsCopy {
			continue
		}

		// Create a modified copy of the message
		newBlocks := make([]api.ContentBlock, len(m.Content))
		copy(newBlocks, m.Content)
		for j, cb := range newBlocks {
			if cb.Type == api.ContentToolResult &&
				compactableIDs[cb.ToolUseID] &&
				!state.ClearedIDs[cb.ToolUseID] &&
				len(cb.Content) > microCompactMinLength {
				newBlocks[j].Content = "[Content cleared to save context]"
				state.ClearedIDs[cb.ToolUseID] = true
			}
		}
		result[i] = api.Message{
			Role:    m.Role,
			Content: newBlocks,
		}
	}

	return result
}

// UpdateCacheBreakpoint updates the state's breakpoint for the next
// APIMicroCompact call.
func UpdateCacheBreakpoint(state *CachedMicroCompactState, newBreakpoint int) {
	state.LastCacheBreakpoint = newBreakpoint
}
