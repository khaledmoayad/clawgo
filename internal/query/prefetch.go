package query

import (
	"context"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// MemoryPrefetchResult holds the outcome of an async memory prefetch.
// The Ready channel is closed when the prefetch completes. Callers can
// select on Ready to non-blockingly check for results.
type MemoryPrefetchResult struct {
	// Memories contains relevant memory snippets found during prefetch.
	// Empty if no relevant memories were found or prefetch was skipped.
	Memories []string

	// Ready is closed when the prefetch goroutine completes. Callers
	// should select on this channel to check for completion without
	// blocking.
	Ready chan struct{}
}

// StartMemoryPrefetch begins an async scan for relevant memories based
// on the current conversation context. It implements the fire-and-forget
// pattern from Claude Code's startRelevantMemoryPrefetch:
//   - Starts a goroutine that scans messages for relevant topics
//   - Looks up matching memories from .claude/memory/ files
//   - Returns a result that can be polled later via the Ready channel
//
// For now this is a stub that returns nil (memory infrastructure will
// be fully wired in Phase 13 Infrastructure). The interface is defined
// here so the query loop can start prefetch during streaming and
// consume results later when the memory system is implemented.
//
// Returns nil if prefetch is not applicable (no messages, disabled, etc.).
func StartMemoryPrefetch(ctx context.Context, messages []api.Message, projectRoot string) *MemoryPrefetchResult {
	if len(messages) == 0 || projectRoot == "" {
		return nil
	}

	// Stub: return a completed result with no memories.
	// Will be replaced with actual memory scanning in Phase 13.
	result := &MemoryPrefetchResult{
		Memories: nil,
		Ready:    make(chan struct{}),
	}

	go func() {
		defer close(result.Ready)
		// Future implementation will:
		// 1. Extract topics/keywords from the last user message
		// 2. Scan .claude/memory/ files for matching content
		// 3. Filter by relevance and byte budget
		// 4. Populate result.Memories
		_ = ctx // Will be used for cancellation
	}()

	return result
}

// ConsumeMemoryPrefetch non-blockingly checks if a memory prefetch has
// completed and returns any memories found. Returns nil if the prefetch
// is not ready or was nil.
func ConsumeMemoryPrefetch(result *MemoryPrefetchResult) []string {
	if result == nil {
		return nil
	}

	select {
	case <-result.Ready:
		return result.Memories
	default:
		return nil
	}
}
