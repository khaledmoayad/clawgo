package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
)

func TestGetFallbackModel(t *testing.T) {
	t.Run("known model returns fallback", func(t *testing.T) {
		fb, ok := GetFallbackModel("claude-sonnet-4-20250514")
		assert.True(t, ok)
		assert.Equal(t, "claude-haiku-3-5-20241022", fb)
	})

	t.Run("opus falls back to sonnet", func(t *testing.T) {
		fb, ok := GetFallbackModel("claude-opus-4-20250514")
		assert.True(t, ok)
		assert.Equal(t, "claude-sonnet-4-20250514", fb)
	})

	t.Run("unknown model returns false", func(t *testing.T) {
		fb, ok := GetFallbackModel("claude-unknown-99")
		assert.False(t, ok)
		assert.Equal(t, "", fb)
	})
}

func TestIsFallbackTrigger(t *testing.T) {
	t.Run("overloaded error triggers fallback", func(t *testing.T) {
		err := &anthropic.Error{StatusCode: 529}
		assert.True(t, IsFallbackTrigger(err))
	})

	t.Run("server error triggers fallback", func(t *testing.T) {
		err := &anthropic.Error{StatusCode: 500}
		assert.True(t, IsFallbackTrigger(err))
	})

	t.Run("auth error does not trigger fallback", func(t *testing.T) {
		err := &anthropic.Error{StatusCode: 401}
		assert.False(t, IsFallbackTrigger(err))
	})

	t.Run("rate limit does not trigger fallback", func(t *testing.T) {
		err := &anthropic.Error{StatusCode: 429}
		assert.False(t, IsFallbackTrigger(err))
	})

	t.Run("client error does not trigger fallback", func(t *testing.T) {
		err := &anthropic.Error{StatusCode: 400}
		assert.False(t, IsFallbackTrigger(err))
	})

	t.Run("nil error does not trigger fallback", func(t *testing.T) {
		assert.False(t, IsFallbackTrigger(nil))
	})
}

func TestWrapWithFallback(t *testing.T) {
	t.Run("wraps when fallback exists and trigger matches", func(t *testing.T) {
		origErr := &anthropic.Error{StatusCode: 529}
		wrapped := WrapWithFallback(origErr, "claude-sonnet-4-20250514")

		var fte *FallbackTriggeredError
		assert.True(t, errors.As(wrapped, &fte))
		assert.Equal(t, "claude-haiku-3-5-20241022", fte.FallbackModel)
		assert.Equal(t, origErr, fte.OriginalError)
	})

	t.Run("returns original when no fallback exists", func(t *testing.T) {
		origErr := &anthropic.Error{StatusCode: 529}
		result := WrapWithFallback(origErr, "unknown-model")
		assert.Equal(t, origErr, result)
	})

	t.Run("returns original when not a trigger", func(t *testing.T) {
		origErr := &anthropic.Error{StatusCode: 401}
		result := WrapWithFallback(origErr, "claude-sonnet-4-20250514")
		assert.Equal(t, origErr, result)
	})
}

func TestFallbackTriggeredError(t *testing.T) {
	t.Run("implements error interface", func(t *testing.T) {
		origErr := fmt.Errorf("overloaded")
		fte := &FallbackTriggeredError{
			OriginalError: origErr,
			FallbackModel: "claude-haiku-3-5-20241022",
		}
		assert.Contains(t, fte.Error(), "fallback triggered to claude-haiku-3-5-20241022")
		assert.Contains(t, fte.Error(), "overloaded")
	})

	t.Run("Unwrap returns original error", func(t *testing.T) {
		origErr := fmt.Errorf("server error")
		fte := &FallbackTriggeredError{
			OriginalError: origErr,
			FallbackModel: "claude-haiku-3-5-20241022",
		}
		assert.Equal(t, origErr, fte.Unwrap())
		assert.True(t, errors.Is(fte, origErr))
	})
}

func TestIsFallbackTriggeredError(t *testing.T) {
	t.Run("detects FallbackTriggeredError", func(t *testing.T) {
		err := &FallbackTriggeredError{
			OriginalError: fmt.Errorf("test"),
			FallbackModel: "model",
		}
		assert.True(t, IsFallbackTriggeredError(err))
	})

	t.Run("detects wrapped FallbackTriggeredError", func(t *testing.T) {
		inner := &FallbackTriggeredError{
			OriginalError: fmt.Errorf("test"),
			FallbackModel: "model",
		}
		wrapped := fmt.Errorf("outer: %w", inner)
		assert.True(t, IsFallbackTriggeredError(wrapped))
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		assert.False(t, IsFallbackTriggeredError(fmt.Errorf("plain error")))
	})
}

func TestExtractFallbackModel(t *testing.T) {
	t.Run("extracts model from FallbackTriggeredError", func(t *testing.T) {
		err := &FallbackTriggeredError{
			OriginalError: fmt.Errorf("test"),
			FallbackModel: "claude-haiku-3-5-20241022",
		}
		assert.Equal(t, "claude-haiku-3-5-20241022", ExtractFallbackModel(err))
	})

	t.Run("returns empty for non-FallbackTriggeredError", func(t *testing.T) {
		assert.Equal(t, "", ExtractFallbackModel(fmt.Errorf("plain")))
	})
}
