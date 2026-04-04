package compact

import (
	"context"
	"errors"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/khaledmoayad/clawgo/internal/api"
)

const (
	// MaxReactiveRetries is the number of compaction attempts before giving up
	// on a prompt-too-long error.
	MaxReactiveRetries = 3
)

// IsPromptTooLongError checks if the error is an Anthropic API error
// indicating the prompt exceeds the model's context window. This occurs
// as a 400 status with message containing "prompt is too long" or as an
// invalid_request_error related to token limits.
func IsPromptTooLongError(err error) bool {
	if err == nil {
		return false
	}

	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		return false
	}

	if apiErr.StatusCode != 400 {
		return false
	}

	msg := strings.ToLower(apiErr.Error())
	return strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "too many tokens") ||
		(strings.Contains(msg, "invalid_request_error") && strings.Contains(msg, "token"))
}

// TruncateHeadForRetry removes the oldest message group (first user+assistant
// pair) from the conversation. If messages has fewer than 4 entries, returns
// messages unchanged to preserve at least the last 2 messages.
func TruncateHeadForRetry(messages []api.Message) []api.Message {
	if len(messages) < 4 {
		return messages
	}

	// Remove the first pair (user + assistant) to shrink the conversation
	// from the oldest end. This is a simple heuristic that works well
	// because the newest messages are most relevant.
	truncated := 2 // Remove first 2 messages (typically user + assistant)

	// If the first two messages are the same role, remove just one
	if messages[0].Role == messages[1].Role {
		truncated = 1
	}

	return messages[truncated:]
}

// ReactiveCompact attempts to recover from a prompt-too-long error by
// compacting the conversation. It tries up to MaxReactiveRetries times:
// each attempt truncates the head of messages, then calls CompactConversation.
// If compaction itself fails with prompt-too-long, it truncates more and retries.
// Returns the compaction result or the original error if all retries fail.
func ReactiveCompact(ctx context.Context, params CompactParams, originalErr error) (*CompactionResult, error) {
	msgs := make([]api.Message, len(params.Messages))
	copy(msgs, params.Messages)

	for attempt := 0; attempt < MaxReactiveRetries; attempt++ {
		// Truncate from the head to reduce size
		msgs = TruncateHeadForRetry(msgs)

		if len(msgs) < 2 {
			// Can't truncate any further
			return nil, originalErr
		}

		// Try compaction with the truncated messages
		compactParams := CompactParams{
			Client:             params.Client,
			Model:              params.Model,
			Messages:           msgs,
			SystemPrompt:       params.SystemPrompt,
			CustomInstructions: params.CustomInstructions,
		}

		result, err := CompactConversation(ctx, compactParams)
		if err != nil {
			if IsPromptTooLongError(err) {
				// Still too long, truncate more and retry
				continue
			}
			// Different error, return it
			return nil, err
		}

		return result, nil
	}

	// All retries exhausted
	return nil, originalErr
}
