package api

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// Max529Retries is the number of consecutive 529 errors before triggering
// model fallback. Matches Claude Code's MAX_529_RETRIES = 3.
const Max529Retries = 3

// ErrFallbackNeeded is returned when consecutive 529 (overloaded) errors
// exceed Max529Retries, indicating the caller should fall back to an
// alternative model.
var ErrFallbackNeeded = errors.New("consecutive 529 errors exceeded MAX_529_RETRIES")

// RetryConfig controls retry behavior for API calls.
// Mirrors the TypeScript withRetry.ts logic: retry on 429/529/5xx with
// exponential backoff and optional jitter.
type RetryConfig struct {
	MaxRetries    int           // Maximum number of retry attempts (default: 10)
	InitialDelay  time.Duration // Delay before first retry (default: 500ms)
	MaxDelay      time.Duration // Maximum delay between retries (default: 5min)
	BackoffFactor float64       // Multiplicative factor per retry (default: 2.0)
	Jitter        bool          // Add random jitter to delays (default: true)
}

// DefaultRetryConfig returns the standard retry configuration matching
// Claude Code's withRetry.ts defaults:
// - DEFAULT_MAX_RETRIES = 10
// - BASE_DELAY_MS = 500
// - PERSISTENT_MAX_BACKOFF_MS = 5 * 60 * 1000 (5 minutes)
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    10,                    // Claude Code DEFAULT_MAX_RETRIES = 10
		InitialDelay:  500 * time.Millisecond, // Claude Code BASE_DELAY_MS = 500
		MaxDelay:      5 * time.Minute,        // Claude Code PERSISTENT_MAX_BACKOFF_MS = 5*60*1000
		BackoffFactor: 2.0,
		Jitter:        true,
	}
}

// ErrorWithHeaders wraps an error with HTTP headers, allowing retry logic
// to inspect rate limit headers like retry-after.
type ErrorWithHeaders struct {
	Err     error
	Headers http.Header
}

func (e *ErrorWithHeaders) Error() string {
	return e.Err.Error()
}

func (e *ErrorWithHeaders) Unwrap() error {
	return e.Err
}

// Is529Error returns true if the error represents an HTTP 529 (overloaded)
// response from the Anthropic API. It checks both the status code and the
// error message for overloaded_error patterns (the SDK sometimes fails to
// pass the 529 status code during streaming).
func Is529Error(err error) bool {
	if err == nil {
		return false
	}

	// Check Anthropic SDK error
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == 529 {
			return true
		}
		// The SDK sometimes fails to properly pass the 529 status code during streaming.
		// Check the raw JSON body for overloaded_error pattern.
		rawJSON := apiErr.RawJSON()
		if rawJSON != "" && strings.Contains(rawJSON, `"type":"overloaded_error"`) {
			return true
		}
	}

	// Check HTTPError (from auxiliary API clients)
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == 529
	}

	return false
}

// WithRetry executes fn, retrying on retryable errors with exponential backoff.
// It respects context cancellation between retries. When an error includes
// a retry-after header (via ErrorWithHeaders), that duration is honored
// (capped at MaxDelay).
// Returns the result of fn on success, or the last error after all retries are
// exhausted (or immediately for non-retryable errors).
func WithRetry[T any](ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error

	totalAttempts := cfg.MaxRetries + 1 // initial attempt + retries
	for attempt := 0; attempt < totalAttempts; attempt++ {
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Don't retry non-retryable errors
		if !IsRetryable(err) {
			return zero, err
		}

		// Don't sleep after the last attempt
		if attempt >= cfg.MaxRetries {
			break
		}

		// Check for retry-after header in ErrorWithHeaders
		delay := computeDelayWithRetryAfter(cfg, attempt, err)

		// Wait for delay or context cancellation
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}

	return zero, lastErr
}

// WithRetry529Tracking is like WithRetry but additionally tracks consecutive
// 529 (overloaded) errors. When Max529Retries consecutive 529 errors occur,
// it returns ErrFallbackNeeded to signal the caller should switch to an
// alternative model. Non-529 errors reset the consecutive counter.
func WithRetry529Tracking[T any](ctx context.Context, cfg RetryConfig, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error
	consecutive529 := 0

	totalAttempts := cfg.MaxRetries + 1
	for attempt := 0; attempt < totalAttempts; attempt++ {
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// Track consecutive 529 errors
		if Is529Error(err) {
			consecutive529++
			if consecutive529 >= Max529Retries {
				return zero, ErrFallbackNeeded
			}
		} else {
			// Non-529 error resets the counter
			consecutive529 = 0
		}

		// Don't retry non-retryable errors
		if !IsRetryable(err) {
			return zero, err
		}

		// Don't sleep after the last attempt
		if attempt >= cfg.MaxRetries {
			break
		}

		// Check for retry-after header in ErrorWithHeaders
		delay := computeDelayWithRetryAfter(cfg, attempt, err)

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}

	return zero, lastErr
}

// computeDelayWithRetryAfter calculates the backoff delay, honoring any
// retry-after header from ErrorWithHeaders. Falls back to standard exponential
// backoff if no retry-after header is present or parseable.
func computeDelayWithRetryAfter(cfg RetryConfig, attempt int, err error) time.Duration {
	// Check for retry-after header in ErrorWithHeaders
	var ewh *ErrorWithHeaders
	if errors.As(err, &ewh) && ewh.Headers != nil {
		retryAfterStr := ewh.Headers.Get("Retry-After")
		if retryAfterStr != "" {
			seconds, parseErr := strconv.Atoi(retryAfterStr)
			if parseErr == nil && seconds > 0 {
				delay := time.Duration(seconds) * time.Second
				// Cap at MaxDelay
				if delay > cfg.MaxDelay {
					delay = cfg.MaxDelay
				}
				return delay
			}
		}
	}

	return computeDelay(cfg, attempt)
}

// computeDelay calculates the backoff delay for a given attempt.
func computeDelay(cfg RetryConfig, attempt int) time.Duration {
	// Exponential backoff: initialDelay * backoffFactor^attempt
	delay := float64(cfg.InitialDelay) * math.Pow(cfg.BackoffFactor, float64(attempt))

	// Cap at max delay
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	// Add jitter: random 0-25% of delay
	if cfg.Jitter {
		jitter := delay * 0.25 * rand.Float64()
		delay += jitter
	}

	return time.Duration(delay)
}
