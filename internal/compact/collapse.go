package compact

import (
	"context"
	"sync"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// StagedCollapse represents a deferred compression queued for future application.
// Context collapse stages these instead of immediately compacting, allowing
// them to drain on overflow before falling back to reactive compact.
type StagedCollapse struct {
	MessageIndex int       // Index in conversation where collapse applies
	Reason       string    // Why this collapse was staged
	StagedAt     time.Time // When the collapse was staged
}

// ContextCollapser manages staged context collapses that are applied on overflow
// or periodically drained. This matches Claude Code's contextCollapse module:
// collapses are staged when old tool results or irrelevant sections are detected,
// then drained before reactive compact kicks in on a prompt-too-long error.
type ContextCollapser struct {
	enabled          bool
	stagedCollapses  []StagedCollapse
	withheldMessages []api.Message // Messages withheld during overflow
	mu               sync.Mutex
}

// NewContextCollapser creates a new ContextCollapser.
// When enabled is false, all operations are no-ops.
func NewContextCollapser(enabled bool) *ContextCollapser {
	return &ContextCollapser{
		enabled: enabled,
	}
}

// IsEnabled returns whether context collapse is active.
func (c *ContextCollapser) IsEnabled() bool {
	return c.enabled
}

// StageCollapse adds a collapse to the staged queue. Called when the system
// detects old tool results or irrelevant sections that could be deferred-compressed.
func (c *ContextCollapser) StageCollapse(messageIndex int, reason string) {
	if !c.enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.stagedCollapses = append(c.stagedCollapses, StagedCollapse{
		MessageIndex: messageIndex,
		Reason:       reason,
		StagedAt:     time.Now(),
	})
}

// StagedCount returns the number of pending staged collapses.
func (c *ContextCollapser) StagedCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.stagedCollapses)
}

// ApplyCollapsesIfNeeded checks for staged collapses and applies them by
// replacing targeted message ranges with their compact summary. This is the
// normal (non-emergency) drain path -- called at the collapse_drain continue
// site before reactive compact would trigger.
//
// Returns:
//   - modified messages (or original if no collapses)
//   - true if any collapses were applied
//   - error from compaction (nil on success or skip)
func (c *ContextCollapser) ApplyCollapsesIfNeeded(
	ctx context.Context,
	messages []api.Message,
	params CompactParams,
) ([]api.Message, bool, error) {
	if !c.enabled {
		return messages, false, nil
	}

	c.mu.Lock()
	if len(c.stagedCollapses) == 0 {
		c.mu.Unlock()
		return messages, false, nil
	}

	// Take ownership of staged collapses and clear the queue
	collapses := make([]StagedCollapse, len(c.stagedCollapses))
	copy(collapses, c.stagedCollapses)
	c.stagedCollapses = c.stagedCollapses[:0]
	c.mu.Unlock()

	// Apply collapses oldest first. Each collapse targets a specific message
	// index; we compact that section and replace it with a summary message.
	modified := make([]api.Message, len(messages))
	copy(modified, messages)

	applied := false
	for _, collapse := range collapses {
		if collapse.MessageIndex < 0 || collapse.MessageIndex >= len(modified) {
			continue
		}

		// Build a compact params for the targeted section.
		// We compact from the start up through the collapsed message
		// to get a summary of that region.
		endIdx := collapse.MessageIndex + 1
		if endIdx > len(modified) {
			endIdx = len(modified)
		}

		sectionParams := CompactParams{
			Client:             params.Client,
			Model:              params.Model,
			Messages:           modified[:endIdx],
			SystemPrompt:       params.SystemPrompt,
			CustomInstructions: params.CustomInstructions,
		}

		result, err := CompactConversation(ctx, sectionParams)
		if err != nil {
			// On error, re-stage the remaining collapses and return
			c.mu.Lock()
			c.stagedCollapses = append(c.stagedCollapses, collapses...)
			c.mu.Unlock()
			return messages, false, err
		}

		if result != nil && result.WasCompacted {
			// Replace the compacted section with a summary message
			summary := api.AssistantMessage(result.Summary)
			newMessages := make([]api.Message, 0, len(modified)-endIdx+1)
			newMessages = append(newMessages, summary)
			newMessages = append(newMessages, modified[endIdx:]...)
			modified = newMessages
			applied = true
		}
	}

	return modified, applied, nil
}

// RecoverFromOverflow drains all staged collapses aggressively. Unlike the
// normal gradual application in ApplyCollapsesIfNeeded, this applies ALL
// pending collapses at once. Called when prompt-too-long is encountered
// AND collapses are staged -- this is the "collapse_drain" continue site
// tried BEFORE reactive compact.
//
// Returns:
//   - modified messages (or original if no collapses)
//   - true if collapses were drained
func (c *ContextCollapser) RecoverFromOverflow(messages []api.Message) ([]api.Message, bool) {
	if !c.enabled {
		return messages, false
	}

	c.mu.Lock()
	if len(c.stagedCollapses) == 0 {
		c.mu.Unlock()
		return messages, false
	}

	// Take all staged collapses
	collapses := make([]StagedCollapse, len(c.stagedCollapses))
	copy(collapses, c.stagedCollapses)
	c.stagedCollapses = c.stagedCollapses[:0]
	c.mu.Unlock()

	// Aggressively apply: remove the messages at all collapse indices.
	// Build a set of indices to remove.
	removeSet := make(map[int]bool, len(collapses))
	for _, collapse := range collapses {
		if collapse.MessageIndex >= 0 && collapse.MessageIndex < len(messages) {
			removeSet[collapse.MessageIndex] = true
		}
	}

	if len(removeSet) == 0 {
		return messages, false
	}

	// Build new message slice excluding removed indices
	result := make([]api.Message, 0, len(messages)-len(removeSet))
	for i, m := range messages {
		if !removeSet[i] {
			result = append(result, m)
		}
	}

	return result, true
}

// IsWithheldPromptTooLong returns true if the collapser is holding withheld
// messages that triggered a prompt-too-long. Used by the query loop to
// decide whether to try collapse drain before reactive compact.
func (c *ContextCollapser) IsWithheldPromptTooLong(messages []api.Message) bool {
	if !c.enabled {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.withheldMessages) > 0 && len(c.stagedCollapses) > 0
}

// SetWithheldMessages stores messages that were withheld during an overflow.
// These are tracked so IsWithheldPromptTooLong can determine if collapse
// drain should be attempted.
func (c *ContextCollapser) SetWithheldMessages(messages []api.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.withheldMessages = messages
}

// ClearWithheldMessages removes all withheld messages after recovery.
func (c *ContextCollapser) ClearWithheldMessages() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.withheldMessages = nil
}

// Reset clears all staged collapses and withheld messages.
// Called after a full compaction to start fresh.
func (c *ContextCollapser) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stagedCollapses = c.stagedCollapses[:0]
	c.withheldMessages = nil
}
