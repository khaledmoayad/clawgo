package compact

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/khaledmoayad/clawgo/internal/api"
)

func TestFormatCompactSummary_StripsAnalysisAndExtractsSummary(t *testing.T) {
	raw := `<analysis>
This is internal analysis that should be removed.
</analysis>

<summary>
This is the actual summary content.
It has multiple lines.
</summary>`

	result := FormatCompactSummary(raw)
	assert.NotContains(t, result, "analysis")
	assert.NotContains(t, result, "<analysis>")
	assert.NotContains(t, result, "</analysis>")
	assert.Contains(t, result, "This is the actual summary content.")
	assert.Contains(t, result, "It has multiple lines.")
	assert.NotContains(t, result, "<summary>")
	assert.NotContains(t, result, "</summary>")
}

func TestFormatCompactSummary_NoSummaryTags(t *testing.T) {
	raw := "Just plain text without any tags."
	result := FormatCompactSummary(raw)
	assert.Equal(t, "Just plain text without any tags.", result)
}

func TestFormatCompactSummary_CleansExcessiveWhitespace(t *testing.T) {
	raw := `<summary>
Line one.



Line two.




Line three.
</summary>`

	result := FormatCompactSummary(raw)
	// Should have at most 2 newlines between paragraphs
	assert.NotContains(t, result, "\n\n\n")
	assert.Contains(t, result, "Line one.")
	assert.Contains(t, result, "Line two.")
	assert.Contains(t, result, "Line three.")
}

func TestFormatCompactSummary_AnalysisOnly(t *testing.T) {
	raw := `<analysis>Remove me</analysis>

Remaining text.`

	result := FormatCompactSummary(raw)
	assert.NotContains(t, result, "Remove me")
	assert.Contains(t, result, "Remaining text.")
}

func TestGetEffectiveContextWindowSize_DefaultModel(t *testing.T) {
	// Unset any override
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "")

	size := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	// 200000 - 20000 = 180000
	assert.Equal(t, 180000, size)
}

func TestGetEffectiveContextWindowSize_UnknownModel(t *testing.T) {
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "")

	size := GetEffectiveContextWindowSize("unknown-model")
	// Should use default: 200000 - 20000 = 180000
	assert.Equal(t, 180000, size)
}

func TestGetEffectiveContextWindowSize_EnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "100000")

	size := GetEffectiveContextWindowSize("claude-sonnet-4-20250514")
	// 100000 - 20000 = 80000
	assert.Equal(t, 80000, size)
}

func TestGetAutoCompactThreshold_Default(t *testing.T) {
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "")
	t.Setenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE", "")

	threshold := GetAutoCompactThreshold("claude-sonnet-4-20250514")
	// 200000 - 20000 - 13000 = 167000
	assert.Equal(t, 167000, threshold)
}

func TestGetAutoCompactThreshold_PercentOverride(t *testing.T) {
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "")
	t.Setenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE", "80")

	threshold := GetAutoCompactThreshold("claude-sonnet-4-20250514")
	// effective = 180000, 80% = 144000
	assert.Equal(t, 144000, threshold)
}

func TestCheckAutoCompact_BelowThreshold(t *testing.T) {
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "")
	t.Setenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE", "")

	params := CompactParams{
		Model: "claude-sonnet-4-20250514",
	}

	// Token count well below threshold (167000)
	result, failures, err := CheckAutoCompact(context.Background(), params, 100000, 0)
	require.NoError(t, err)
	assert.Nil(t, result, "should not compact when below threshold")
	assert.Equal(t, 0, failures)
}

func TestCheckAutoCompact_CircuitBreaker(t *testing.T) {
	t.Setenv("CLAUDE_CODE_AUTO_COMPACT_WINDOW", "")
	t.Setenv("CLAUDE_AUTOCOMPACT_PCT_OVERRIDE", "")

	params := CompactParams{
		Model:    "claude-sonnet-4-20250514",
		Messages: []api.Message{api.UserMessage("test")},
	}

	// Token count above threshold but circuit breaker tripped
	result, failures, err := CheckAutoCompact(
		context.Background(), params, 200000, MaxConsecutiveFailures,
	)
	require.NoError(t, err)
	assert.Nil(t, result, "should skip compaction when circuit breaker is tripped")
	assert.Equal(t, MaxConsecutiveFailures, failures)
}

func TestBuildCompactionMessages_EmptyInput(t *testing.T) {
	result := BuildCompactionMessages(nil)
	assert.Nil(t, result)
}

func TestBuildCompactionMessages_PreservesAlternation(t *testing.T) {
	messages := []api.Message{
		api.UserMessage("Hello"),
		api.AssistantMessage("Hi there"),
		api.UserMessage("How are you?"),
	}

	result := BuildCompactionMessages(messages)
	require.Len(t, result, 3)
	assert.Equal(t, "user", string(result[0].Role))
	assert.Equal(t, "assistant", string(result[1].Role))
	assert.Equal(t, "user", string(result[2].Role))
}

func TestBuildCompactionMessages_MergesAdjacentSameRole(t *testing.T) {
	messages := []api.Message{
		api.UserMessage("First"),
		api.UserMessage("Second"),
		api.AssistantMessage("Response"),
	}

	result := BuildCompactionMessages(messages)
	// Two user messages should be merged into one
	require.Len(t, result, 2)
	assert.Equal(t, "user", string(result[0].Role))
	assert.Equal(t, "assistant", string(result[1].Role))
	// Merged user message should have 2 content blocks
	assert.Len(t, result[0].Content, 2)
}

func TestBuildCompactionMessages_PrependUserIfStartsWithAssistant(t *testing.T) {
	messages := []api.Message{
		api.AssistantMessage("I'll help you"),
	}

	result := BuildCompactionMessages(messages)
	// Should prepend a user message
	require.Len(t, result, 2)
	assert.Equal(t, "user", string(result[0].Role))
	assert.Equal(t, "assistant", string(result[1].Role))
}

func TestEstimateMessageTokens(t *testing.T) {
	messages := []api.Message{
		api.UserMessage("Hello world"), // 11 chars
		api.AssistantMessage("Hi"),     // 2 chars
	}

	tokens := estimateMessageTokens(messages)
	// (11 + 2) / 4 = 3 (integer division)
	assert.Equal(t, 3, tokens)
}
