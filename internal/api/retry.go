package api

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

// RetryConfig controls retry behavior for API calls.
// Mirrors the TypeScript withRetry.ts logic: retry on 429/529/5xx with
// exponential backoff and optional jitter.
type RetryConfig struct {
	MaxRetries    int           // Maximum number of retry attempts (default: 3)
	InitialDelay  time.Duration // Delay before first retry (default: 1s)
	MaxDelay      time.Duration // Maximum delay between retries (default: 30s)
	BackoffFactor float64       // Multiplicative factor per retry (default: 2.0)
	Jitter        bool          // Add random jitter to delays (default: true)
}

// DefaultRetryConfig returns the standard retry configuration matching
// the TypeScript withRetry.ts defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        true,
	}
}

// WithRetry executes fn, retrying on retryable errors with exponential backoff.
// It respects context cancellation between retries.
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

		// Compute backoff delay
		delay := computeDelay(cfg, attempt)

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
