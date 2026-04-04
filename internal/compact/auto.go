package compact

import (
	"context"
	"os"
	"strconv"
)

const (
	// MaxOutputTokensForSummary is the max_tokens value used for compaction API calls.
	MaxOutputTokensForSummary = 20000

	// AutocompactBufferTokens is the token buffer subtracted from the effective
	// context window to determine the auto-compact threshold. This ensures
	// compaction triggers before hitting the hard limit.
	AutocompactBufferTokens = 13000

	// WarningThresholdBuffer is the additional buffer for warning the user
	// about approaching the context limit.
	WarningThresholdBuffer = 20000

	// MaxConsecutiveFailures is the circuit breaker limit. After this many
	// consecutive compaction failures, auto-compact is disabled for the session.
	MaxConsecutiveFailures = 3
)

// modelContextWindows maps model identifiers to their context window sizes.
var modelContextWindows = map[string]int{
	"claude-sonnet-4-20250514":  200000,
	"claude-opus-4-20250514":    200000,
	"claude-haiku-3-20250307":   200000,
	"claude-3-5-sonnet-20241022": 200000,
}

const defaultContextWindow = 200000

// GetEffectiveContextWindowSize returns the usable context window for a model.
// It subtracts the reserved output tokens from the total context window.
// The CLAUDE_CODE_AUTO_COMPACT_WINDOW env var can override the total window size.
func GetEffectiveContextWindowSize(model string) int {
	window := defaultContextWindow

	// Check env var override
	if envVal := os.Getenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW"); envVal != "" {
		if parsed, err := strconv.Atoi(envVal); err == nil && parsed > 0 {
			window = parsed
		}
	} else {
		// Look up model-specific window
		if w, ok := modelContextWindows[model]; ok {
			window = w
		}
	}

	// Subtract reserved output tokens (min of model's max output and our cap)
	reserved := MaxOutputTokensForSummary
	effective := window - reserved
	if effective < 0 {
		effective = 0
	}
	return effective
}

// GetAutoCompactThreshold returns the token count at which auto-compaction
// should trigger. The CLAUDE_AUTOCOMPACT_PCT_OVERRIDE env var can specify
// a percentage of the effective window to use as the threshold instead.
func GetAutoCompactThreshold(model string) int {
	effective := GetEffectiveContextWindowSize(model)

	// Check percentage override
	if pctStr := os.Getenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE"); pctStr != "" {
		if pct, err := strconv.ParseFloat(pctStr, 64); err == nil && pct > 0 && pct <= 100 {
			return int(float64(effective) * pct / 100)
		}
	}

	threshold := effective - AutocompactBufferTokens
	if threshold < 0 {
		threshold = 0
	}
	return threshold
}

// CheckAutoCompact checks if the current token count exceeds the auto-compact
// threshold and performs compaction if needed. It respects the circuit breaker:
// after MaxConsecutiveFailures, no further attempts are made.
//
// Returns:
//   - result: the compaction result (nil if no compaction needed or skipped)
//   - consecutiveFailures: updated failure count
//   - err: error from the compaction attempt (nil on success or skip)
func CheckAutoCompact(
	ctx context.Context,
	params CompactParams,
	tokenCount int,
	consecutiveFailures int,
) (*CompactionResult, int, error) {
	// Circuit breaker: stop trying after too many failures
	if consecutiveFailures >= MaxConsecutiveFailures {
		return nil, consecutiveFailures, nil
	}

	threshold := GetAutoCompactThreshold(params.Model)
	if tokenCount < threshold {
		return nil, consecutiveFailures, nil
	}

	result, err := CompactConversation(ctx, params)
	if err != nil {
		return nil, consecutiveFailures + 1, err
	}

	// Reset failure count on success
	return result, 0, nil
}
