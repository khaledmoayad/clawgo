package api

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// shortRetryConfig returns a retry config with very short delays for testing.
func shortRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		InitialDelay:  10 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false, // Disable jitter for deterministic tests
	}
}

func TestWithRetry_Success(t *testing.T) {
	var callCount atomic.Int32
	cfg := shortRetryConfig()

	result, err := WithRetry(context.Background(), cfg, func(ctx context.Context) (string, error) {
		callCount.Add(1)
		return "success", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success" {
		t.Errorf("expected %q, got %q", "success", result)
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call, got %d", callCount.Load())
	}
}

func TestWithRetry_RetryThenSuccess(t *testing.T) {
	var callCount atomic.Int32
	cfg := shortRetryConfig()

	result, err := WithRetry(context.Background(), cfg, func(ctx context.Context) (string, error) {
		n := callCount.Add(1)
		if n < 3 {
			return "", &anthropic.Error{StatusCode: 429} // retryable
		}
		return "success after retries", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "success after retries" {
		t.Errorf("expected %q, got %q", "success after retries", result)
	}
	if callCount.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", callCount.Load())
	}
}

func TestWithRetry_NonRetryable(t *testing.T) {
	var callCount atomic.Int32
	cfg := shortRetryConfig()

	_, err := WithRetry(context.Background(), cfg, func(ctx context.Context) (string, error) {
		callCount.Add(1)
		return "", &anthropic.Error{StatusCode: 401} // auth error, not retryable
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call (no retries), got %d", callCount.Load())
	}
}

func TestWithRetry_ExhaustedRetries(t *testing.T) {
	var callCount atomic.Int32
	cfg := shortRetryConfig()
	cfg.MaxRetries = 2

	_, err := WithRetry(context.Background(), cfg, func(ctx context.Context) (string, error) {
		callCount.Add(1)
		return "", &anthropic.Error{StatusCode: 500} // retryable
	})

	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// MaxRetries=2 means 3 total attempts (initial + 2 retries)
	if callCount.Load() != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount.Load())
	}
}

func TestWithRetry_ContextCancel(t *testing.T) {
	var callCount atomic.Int32
	cfg := shortRetryConfig()
	cfg.InitialDelay = 5 * time.Second // Long delay so we can cancel during it

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 50ms (during the backoff sleep)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := WithRetry(ctx, cfg, func(ctx context.Context) (string, error) {
		callCount.Add(1)
		return "", &anthropic.Error{StatusCode: 429}
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
	// Should have called fn at least once (the initial attempt)
	if callCount.Load() < 1 {
		t.Errorf("expected at least 1 call, got %d", callCount.Load())
	}
}

func TestWithRetry_BackoffDelays(t *testing.T) {
	cfg := RetryConfig{
		MaxRetries:    3,
		InitialDelay:  20 * time.Millisecond,
		MaxDelay:      1 * time.Second,
		BackoffFactor: 2.0,
		Jitter:        false,
	}

	var timestamps []time.Time
	var callCount atomic.Int32

	_, _ = WithRetry(context.Background(), cfg, func(ctx context.Context) (string, error) {
		timestamps = append(timestamps, time.Now())
		callCount.Add(1)
		return "", &anthropic.Error{StatusCode: 500}
	})

	if len(timestamps) != 4 { // initial + 3 retries
		t.Fatalf("expected 4 timestamps, got %d", len(timestamps))
	}

	// Verify increasing delays (with tolerance)
	// Expected: ~20ms, ~40ms, ~80ms between attempts
	for i := 1; i < len(timestamps); i++ {
		delay := timestamps[i].Sub(timestamps[i-1])
		expectedDelay := time.Duration(float64(cfg.InitialDelay) * pow(cfg.BackoffFactor, float64(i-1)))
		tolerance := time.Duration(float64(expectedDelay) * 0.5) // 50% tolerance for CI
		minDelay := expectedDelay - tolerance
		if delay < minDelay {
			t.Errorf("attempt %d: delay %v was shorter than expected minimum %v (expected ~%v)",
				i, delay, minDelay, expectedDelay)
		}
	}
}

func pow(base, exp float64) float64 {
	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}
	return result
}

func TestWithRetry_DifferentRetryableErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		shouldRetry bool
	}{
		{"rate_limit_429", 429, true},
		{"overloaded_529", 529, true},
		{"server_error_500", 500, true},
		{"server_error_503", 503, true},
		{"auth_401", 401, false},
		{"client_error_400", 400, false},
		{"client_error_422", 422, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var callCount atomic.Int32
			cfg := shortRetryConfig()
			cfg.MaxRetries = 1

			_, _ = WithRetry(context.Background(), cfg, func(ctx context.Context) (string, error) {
				callCount.Add(1)
				return "", &anthropic.Error{StatusCode: tt.statusCode}
			})

			if tt.shouldRetry {
				if callCount.Load() != 2 {
					t.Errorf("expected 2 attempts for retryable error %d, got %d", tt.statusCode, callCount.Load())
				}
			} else {
				if callCount.Load() != 1 {
					t.Errorf("expected 1 attempt for non-retryable error %d, got %d", tt.statusCode, callCount.Load())
				}
			}
		})
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if cfg.InitialDelay != 1*time.Second {
		t.Errorf("expected InitialDelay 1s, got %v", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("expected MaxDelay 30s, got %v", cfg.MaxDelay)
	}
	if cfg.BackoffFactor != 2.0 {
		t.Errorf("expected BackoffFactor 2.0, got %f", cfg.BackoffFactor)
	}
	if !cfg.Jitter {
		t.Error("expected Jitter to be true")
	}
}

// Verify generic type parameter works with different types
func TestWithRetry_IntResult(t *testing.T) {
	cfg := shortRetryConfig()
	result, err := WithRetry(context.Background(), cfg, func(ctx context.Context) (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestWithRetry_ZeroValueOnError(t *testing.T) {
	cfg := shortRetryConfig()
	cfg.MaxRetries = 0

	result, err := WithRetry(context.Background(), cfg, func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("some non-api error")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if result != "" {
		t.Errorf("expected zero value, got %q", result)
	}
}
