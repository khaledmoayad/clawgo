package query

import (
	"strings"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// Recovery constants matching Claude Code's query.ts and utils/context.ts.
const (
	// MaxOutputTokensRecoveryLimit is the number of retry attempts when
	// the model hits max_tokens. After this many retries, the truncated
	// response is returned. Matches TS MAX_OUTPUT_TOKENS_RECOVERY_LIMIT.
	MaxOutputTokensRecoveryLimit = 3

	// CappedDefaultMaxTokens is the default max output tokens when the
	// slot-aware cap is active. Matches TS CAPPED_DEFAULT_MAX_TOKENS.
	CappedDefaultMaxTokens = 8_000

	// EscalatedMaxTokens is the higher output token limit used when the
	// capped default hits max_tokens. The first recovery escalates to this
	// value before falling through to multi-turn continuation retries.
	// Matches TS ESCALATED_MAX_TOKENS from utils/context.ts.
	EscalatedMaxTokens = 64_000
)

// MaxTokensRecoveryState tracks the recovery loop state across retries.
type MaxTokensRecoveryState struct {
	// RecoveryCount tracks how many continuation retries have been attempted.
	RecoveryCount int

	// MaxOutputOverride is the escalated max_tokens value (0 means no override).
	MaxOutputOverride int
}

// RecoveryAction describes what the query loop should do next.
type RecoveryAction string

const (
	// RecoveryEscalate means increase max_tokens to EscalatedMaxTokens and retry
	// the same request (no continuation message needed).
	RecoveryEscalate RecoveryAction = "escalate"

	// RecoveryRetry means keep the same max_tokens, append a continuation
	// message, and retry. The caller must increment RecoveryCount.
	RecoveryRetry RecoveryAction = "retry"

	// RecoveryStop means all retries are exhausted; surface the truncated response.
	RecoveryStop RecoveryAction = "stop"
)

// IsMaxTokensStop returns true when the API response stop_reason indicates
// the output was truncated due to the max_tokens limit.
func IsMaxTokensStop(stopReason string) bool {
	return stopReason == "max_tokens"
}

// HandleMaxTokensRecovery determines the next recovery action based on
// current state. It implements the two-phase recovery from Claude Code:
//
//  1. Escalation: If the capped default was used (currentMaxTokens < EscalatedMaxTokens)
//     and no override is active yet, escalate to EscalatedMaxTokens.
//  2. Continuation retry: Append a continuation message and retry up to
//     MaxOutputTokensRecoveryLimit times.
//  3. Stop: All retries exhausted.
func HandleMaxTokensRecovery(state *MaxTokensRecoveryState, currentMaxTokens int) (RecoveryAction, int) {
	// Phase 1: Escalate from capped default to full limit.
	// Only fires when no override is already active (MaxOutputOverride == 0)
	// and current tokens are below the escalated threshold.
	if state.MaxOutputOverride == 0 && currentMaxTokens < EscalatedMaxTokens {
		return RecoveryEscalate, EscalatedMaxTokens
	}

	// Phase 2: Multi-turn continuation retry.
	if state.RecoveryCount < MaxOutputTokensRecoveryLimit {
		return RecoveryRetry, currentMaxTokens
	}

	// Phase 3: Exhausted.
	return RecoveryStop, currentMaxTokens
}

// IsMediaSizeError checks whether an API error indicates that an image or
// document in the request is too large. These errors are recoverable by
// stripping the oversized media blocks and retrying.
//
// Patterns matched (case-insensitive):
//   - "image" + "too large"
//   - "document" + "too large"
//   - "request_too_large" (error type string)
func IsMediaSizeError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// Check for image/document too large patterns
	if strings.Contains(msg, "too large") {
		if strings.Contains(msg, "image") || strings.Contains(msg, "document") {
			return true
		}
	}

	// Check for request_too_large error type
	if strings.Contains(msg, "request_too_large") {
		return true
	}

	return false
}

// HandleMediaSizeError strips image and document content blocks from the
// most recent user message. This matches Claude Code's strip-and-retry
// behavior for oversized media: remove the problematic blocks and let the
// API call succeed without them.
//
// If the last user message has no media blocks, messages are returned unchanged.
func HandleMediaSizeError(messages []api.Message) []api.Message {
	if len(messages) == 0 {
		return messages
	}

	// Find the last user message
	lastUserIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}

	if lastUserIdx == -1 {
		return messages
	}

	lastUser := messages[lastUserIdx]
	var filtered []api.ContentBlock
	stripped := false

	for _, block := range lastUser.Content {
		switch block.Type {
		case api.ContentImage, api.ContentDocument:
			// Strip media blocks
			stripped = true
		default:
			filtered = append(filtered, block)
		}
	}

	if !stripped {
		return messages
	}

	// Build modified message list with the stripped user message
	result := make([]api.Message, len(messages))
	copy(result, messages)
	result[lastUserIdx] = api.Message{
		Role:    "user",
		Content: filtered,
	}

	return result
}

// GetContinuationMessage returns a user message instructing the model to
// continue from where it left off. This is appended to the conversation
// when max_tokens recovery uses the multi-turn retry path.
//
// Matches the TS recovery message from query.ts.
func GetContinuationMessage() api.Message {
	return api.UserMessage(
		"Output token limit hit. Resume directly — no apology, no recap of what you were doing. " +
			"Pick up mid-thought if that is where the cut happened. Break remaining work into smaller pieces.",
	)
}
