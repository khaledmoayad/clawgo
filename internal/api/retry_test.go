package api

import (
	"context"
	"fmt"
	"net/http"
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
		name        string
		statusCode  int
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

// TestDefaultRetryConfig_MatchesClaudeCode verifies the corrected retry constants
// matching Claude Code's withRetry.ts: DEFAULT_MAX_RETRIES=10, BASE_DELAY_MS=500,
// PERSISTENT_MAX_BACKOFF_MS=5*60*1000.
func TestDefaultRetryConfig_MatchesClaudeCode(t *testing.T) {
	cfg := DefaultRetryConfig()
	if cfg.MaxRetries != 10 {
		t.Errorf("expected MaxRetries 10 (Claude Code DEFAULT_MAX_RETRIES), got %d", cfg.MaxRetries)
	}
	if cfg.InitialDelay != 500*time.Millisecond {
		t.Errorf("expected InitialDelay 500ms (Claude Code BASE_DELAY_MS), got %v", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 5*time.Minute {
		t.Errorf("expected MaxDelay 5m (Claude Code PERSISTENT_MAX_BACKOFF_MS), got %v", cfg.MaxDelay)
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

// --- New tests for rate limit header parsing ---

func TestParseRateLimitHeaders_RetryAfterSeconds(t *testing.T) {
	headers := http.Header{}
	headers.Set("retry-after", "2")

	info := ParseRateLimitHeaders(headers)
	if info == nil {
		t.Fatal("expected non-nil RateLimitInfo")
	}
	if info.RetryAfter != 2*time.Second {
		t.Errorf("expected RetryAfter 2s, got %v", info.RetryAfter)
	}
}

func TestParseRateLimitHeaders_UnifiedReset(t *testing.T) {
	// Set a reset time 60 seconds in the future
	resetTime := time.Now().Add(60 * time.Second)
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-reset", fmt.Sprintf("%d", resetTime.Unix()))

	info := ParseRateLimitHeaders(headers)
	if info == nil {
		t.Fatal("expected non-nil RateLimitInfo")
	}
	// The unified reset duration should be approximately 60 seconds
	if info.UnifiedResetDuration < 55*time.Second || info.UnifiedResetDuration > 65*time.Second {
		t.Errorf("expected UnifiedResetDuration ~60s, got %v", info.UnifiedResetDuration)
	}
}

func TestParseRateLimitHeaders_OverageDisabled(t *testing.T) {
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-overage-disabled-reason", "org_not_enabled")

	info := ParseRateLimitHeaders(headers)
	if info == nil {
		t.Fatal("expected non-nil RateLimitInfo")
	}
	if info.OverageDisabledReason != "org_not_enabled" {
		t.Errorf("expected OverageDisabledReason 'org_not_enabled', got %q", info.OverageDisabledReason)
	}
}

func TestParseRateLimitHeaders_Empty(t *testing.T) {
	headers := http.Header{}

	info := ParseRateLimitHeaders(headers)
	if info != nil {
		t.Error("expected nil RateLimitInfo for empty headers")
	}
}

func TestParseRateLimitHeaders_InvalidRetryAfter(t *testing.T) {
	headers := http.Header{}
	headers.Set("retry-after", "not-a-number")

	info := ParseRateLimitHeaders(headers)
	// Should return nil when retry-after is unparseable and no other headers present
	if info != nil {
		t.Error("expected nil RateLimitInfo for invalid retry-after")
	}
}

// --- New tests for 529 error detection ---

func TestIs529Error_SDK529(t *testing.T) {
	err := &anthropic.Error{StatusCode: 529}
	if !Is529Error(err) {
		t.Error("expected Is529Error to return true for 529 status code")
	}
}

func TestIs529Error_HTTPError529(t *testing.T) {
	err := &HTTPError{StatusCode: 529, Body: "overloaded"}
	if !Is529Error(err) {
		t.Error("expected Is529Error to return true for HTTPError with 529")
	}
}

func TestIs529Error_Not529(t *testing.T) {
	err := &anthropic.Error{StatusCode: 429}
	if Is529Error(err) {
		t.Error("expected Is529Error to return false for 429 status code")
	}
}

// Note: Testing overloaded_error message detection requires a real HTTP response
// because the SDK's Error.Error() method reads from the response body/raw JSON.
// This is tested indirectly via the 529 status code detection above.

func TestIs529Error_NonAPIError(t *testing.T) {
	err := fmt.Errorf("some random error")
	if Is529Error(err) {
		t.Error("expected Is529Error to return false for non-API error")
	}
}

// --- New tests for 529 fallback tracking ---

func TestWithRetry529Tracking_FallbackAfterConsecutive(t *testing.T) {
	var callCount atomic.Int32
	cfg := shortRetryConfig()
	cfg.MaxRetries = 10

	_, err := WithRetry529Tracking(context.Background(), cfg, func(ctx context.Context) (string, error) {
		callCount.Add(1)
		return "", &anthropic.Error{StatusCode: 529}
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if err != ErrFallbackNeeded {
		t.Errorf("expected ErrFallbackNeeded, got: %v", err)
	}
	// Should stop after MAX_529_RETRIES (3) consecutive 529 errors
	if callCount.Load() != int32(Max529Retries) {
		t.Errorf("expected %d attempts, got %d", Max529Retries, callCount.Load())
	}
}

func TestWithRetry529Tracking_ResetOnNon529(t *testing.T) {
	var callCount atomic.Int32
	cfg := shortRetryConfig()
	cfg.MaxRetries = 10

	_, err := WithRetry529Tracking(context.Background(), cfg, func(ctx context.Context) (string, error) {
		n := callCount.Add(1)
		switch {
		case n <= 2:
			return "", &anthropic.Error{StatusCode: 529} // 2 consecutive 529s
		case n == 3:
			return "", &anthropic.Error{StatusCode: 500} // non-529 resets counter
		case n <= 5:
			return "", &anthropic.Error{StatusCode: 529} // 2 more 529s
		case n == 6:
			return "", &anthropic.Error{StatusCode: 500} // non-529 resets again
		case n <= 8:
			return "", &anthropic.Error{StatusCode: 529} // 2 more 529s
		default:
			return "success", nil
		}
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Counter reset twice, never reached MAX_529_RETRIES consecutively
	if callCount.Load() < 9 {
		t.Errorf("expected at least 9 calls, got %d", callCount.Load())
	}
}

func TestWithRetry529Tracking_NonRetryableStopsImmediately(t *testing.T) {
	var callCount atomic.Int32
	cfg := shortRetryConfig()

	_, err := WithRetry529Tracking(context.Background(), cfg, func(ctx context.Context) (string, error) {
		callCount.Add(1)
		return "", &anthropic.Error{StatusCode: 401} // non-retryable
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call (no retries for non-retryable), got %d", callCount.Load())
	}
}

// --- Test WithRetry honors retry-after header ---

func TestWithRetry_HonorsRetryAfterHeader(t *testing.T) {
	var callCount atomic.Int32
	cfg := shortRetryConfig()
	cfg.MaxRetries = 1

	start := time.Now()
	_, _ = WithRetry(context.Background(), cfg, func(ctx context.Context) (string, error) {
		n := callCount.Add(1)
		if n == 1 {
			return "", &ErrorWithHeaders{
				Err: &anthropic.Error{StatusCode: 429},
				Headers: http.Header{
					"Retry-After": {"1"}, // 1 second
				},
			}
		}
		return "", &anthropic.Error{StatusCode: 429}
	})
	elapsed := time.Since(start)

	// Should have waited approximately 1 second due to retry-after header
	if elapsed < 800*time.Millisecond {
		t.Errorf("expected at least ~1s delay from retry-after header, got %v", elapsed)
	}
}
