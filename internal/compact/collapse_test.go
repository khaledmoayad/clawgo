package compact

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/khaledmoayad/clawgo/internal/api"
)

func TestCollapseIsEnabled_RespectsConstructorParam(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		c := NewContextCollapser(true)
		assert.True(t, c.IsEnabled())
	})

	t.Run("disabled", func(t *testing.T) {
		c := NewContextCollapser(false)
		assert.False(t, c.IsEnabled())
	})
}

func TestCollapseStageCollapse(t *testing.T) {
	t.Run("stages when enabled", func(t *testing.T) {
		c := NewContextCollapser(true)
		c.StageCollapse(3, "old tool result")
		assert.Equal(t, 1, c.StagedCount())
	})

	t.Run("no-op when disabled", func(t *testing.T) {
		c := NewContextCollapser(false)
		c.StageCollapse(3, "old tool result")
		assert.Equal(t, 0, c.StagedCount())
	})

	t.Run("stages multiple collapses", func(t *testing.T) {
		c := NewContextCollapser(true)
		c.StageCollapse(1, "reason 1")
		c.StageCollapse(3, "reason 2")
		c.StageCollapse(5, "reason 3")
		assert.Equal(t, 3, c.StagedCount())
	})
}

func TestCollapseApplyCollapsesIfNeeded_NoCollapses(t *testing.T) {
	c := NewContextCollapser(true)
	messages := []api.Message{
		api.UserMessage("Hello"),
		api.AssistantMessage("Hi"),
	}

	result, applied, err := c.ApplyCollapsesIfNeeded(
		context.Background(),
		messages,
		CompactParams{Model: "claude-sonnet-4-20250514"},
	)
	require.NoError(t, err)
	assert.False(t, applied, "should not apply when no collapses staged")
	assert.Equal(t, messages, result)
}

func TestCollapseApplyCollapsesIfNeeded_DisabledReturnsUnchanged(t *testing.T) {
	c := NewContextCollapser(false)
	messages := []api.Message{
		api.UserMessage("Hello"),
		api.AssistantMessage("Hi"),
	}

	result, applied, err := c.ApplyCollapsesIfNeeded(
		context.Background(),
		messages,
		CompactParams{Model: "claude-sonnet-4-20250514"},
	)
	require.NoError(t, err)
	assert.False(t, applied)
	assert.Equal(t, messages, result)
}

func TestCollapseRecoverFromOverflow_DrainsAllCollapses(t *testing.T) {
	c := NewContextCollapser(true)

	messages := []api.Message{
		api.UserMessage("old query"),          // 0 - will be removed
		api.AssistantMessage("old response"),   // 1 - will be removed
		api.UserMessage("recent query"),        // 2
		api.AssistantMessage("recent response"), // 3
	}

	// Stage collapses for the old messages
	c.StageCollapse(0, "old tool result")
	c.StageCollapse(1, "old assistant response")

	result, drained := c.RecoverFromOverflow(messages)
	assert.True(t, drained)
	require.Len(t, result, 2, "should have removed 2 messages")
	assert.Equal(t, "recent query", result[0].Content[0].Text)
	assert.Equal(t, "recent response", result[1].Content[0].Text)

	// Staged collapses should be cleared after draining
	assert.Equal(t, 0, c.StagedCount())
}

func TestCollapseRecoverFromOverflow_NoCollapsesReturnsFalse(t *testing.T) {
	c := NewContextCollapser(true)

	messages := []api.Message{
		api.UserMessage("query"),
		api.AssistantMessage("response"),
	}

	result, drained := c.RecoverFromOverflow(messages)
	assert.False(t, drained, "should return false when no collapses staged")
	assert.Equal(t, messages, result)
}

func TestCollapseRecoverFromOverflow_DisabledReturnsFalse(t *testing.T) {
	c := NewContextCollapser(false)

	messages := []api.Message{
		api.UserMessage("query"),
	}

	result, drained := c.RecoverFromOverflow(messages)
	assert.False(t, drained)
	assert.Equal(t, messages, result)
}

func TestCollapseRecoverFromOverflow_OutOfBoundsIndicesIgnored(t *testing.T) {
	c := NewContextCollapser(true)

	messages := []api.Message{
		api.UserMessage("query"),
		api.AssistantMessage("response"),
	}

	// Stage a collapse with an out-of-bounds index
	c.StageCollapse(99, "out of bounds")
	c.StageCollapse(-1, "negative")

	result, drained := c.RecoverFromOverflow(messages)
	assert.False(t, drained, "should return false when all indices are out of bounds")
	assert.Equal(t, messages, result)
}

func TestCollapseIsWithheldPromptTooLong(t *testing.T) {
	t.Run("true when withheld and collapses staged", func(t *testing.T) {
		c := NewContextCollapser(true)
		c.StageCollapse(0, "old content")
		c.SetWithheldMessages([]api.Message{api.UserMessage("withheld")})

		assert.True(t, c.IsWithheldPromptTooLong(nil))
	})

	t.Run("false when no withheld messages", func(t *testing.T) {
		c := NewContextCollapser(true)
		c.StageCollapse(0, "old content")

		assert.False(t, c.IsWithheldPromptTooLong(nil))
	})

	t.Run("false when no collapses staged", func(t *testing.T) {
		c := NewContextCollapser(true)
		c.SetWithheldMessages([]api.Message{api.UserMessage("withheld")})

		assert.False(t, c.IsWithheldPromptTooLong(nil))
	})

	t.Run("false when disabled", func(t *testing.T) {
		c := NewContextCollapser(false)
		assert.False(t, c.IsWithheldPromptTooLong(nil))
	})
}

func TestCollapseReset(t *testing.T) {
	c := NewContextCollapser(true)
	c.StageCollapse(0, "reason")
	c.SetWithheldMessages([]api.Message{api.UserMessage("msg")})

	c.Reset()

	assert.Equal(t, 0, c.StagedCount())
	assert.False(t, c.IsWithheldPromptTooLong(nil))
}

func TestCollapseClearWithheldMessages(t *testing.T) {
	c := NewContextCollapser(true)
	c.StageCollapse(0, "reason")
	c.SetWithheldMessages([]api.Message{api.UserMessage("msg")})

	assert.True(t, c.IsWithheldPromptTooLong(nil))

	c.ClearWithheldMessages()
	assert.False(t, c.IsWithheldPromptTooLong(nil))
}
