package api

import (
	"errors"
	"fmt"
)

// FallbackModelMap maps primary models to their fallback alternatives.
// When a primary model is unavailable (overloaded, server error), the system
// switches to the fallback model for graceful degradation.
var FallbackModelMap = map[string]string{
	"claude-sonnet-4-20250514": "claude-haiku-3-5-20241022",
	"claude-opus-4-20250514":   "claude-sonnet-4-20250514",
}

// FallbackTriggeredError signals that the query loop should retry with a
// fallback model. It wraps the original error that triggered the fallback.
type FallbackTriggeredError struct {
	OriginalError error
	FallbackModel string
}

// Error implements the error interface.
func (e *FallbackTriggeredError) Error() string {
	return fmt.Sprintf("fallback triggered to %s: %v", e.FallbackModel, e.OriginalError)
}

// Unwrap returns the original error for errors.Is/As compatibility.
func (e *FallbackTriggeredError) Unwrap() error {
	return e.OriginalError
}

// GetFallbackModel looks up the fallback model for a given primary model.
// Returns the fallback model name and true if one exists, or empty string
// and false if no fallback is configured.
func GetFallbackModel(model string) (string, bool) {
	fb, ok := FallbackModelMap[model]
	return fb, ok
}

// IsFallbackTrigger returns true if the error should trigger a model fallback.
// Conditions: overloaded (529), server error (500+) categories.
func IsFallbackTrigger(err error) bool {
	cat := CategorizeError(err)
	return cat == ErrOverloaded || cat == ErrServerError
}

// WrapWithFallback wraps the error in a FallbackTriggeredError if:
// 1. The error is a fallback trigger (overloaded or server error)
// 2. A fallback model exists for the current model
// Otherwise returns the original error unchanged.
func WrapWithFallback(err error, currentModel string) error {
	if !IsFallbackTrigger(err) {
		return err
	}
	fb, ok := GetFallbackModel(currentModel)
	if !ok {
		return err
	}
	return &FallbackTriggeredError{
		OriginalError: err,
		FallbackModel: fb,
	}
}

// IsFallbackTriggeredError checks if the error is a FallbackTriggeredError
// using errors.As for wrapped error chain compatibility.
func IsFallbackTriggeredError(err error) bool {
	var fte *FallbackTriggeredError
	return errors.As(err, &fte)
}

// ExtractFallbackModel extracts the FallbackModel from a FallbackTriggeredError.
// Returns empty string if the error is not a FallbackTriggeredError.
func ExtractFallbackModel(err error) string {
	var fte *FallbackTriggeredError
	if errors.As(err, &fte) {
		return fte.FallbackModel
	}
	return ""
}
