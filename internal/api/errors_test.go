package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestParseRateLimitHeaders_AllHeaders(t *testing.T) {
	resetTime := time.Now().Add(120 * time.Second)
	headers := http.Header{}
	headers.Set("retry-after", "5")
	headers.Set("anthropic-ratelimit-unified-reset", fmt.Sprintf("%d", resetTime.Unix()))
	headers.Set("anthropic-ratelimit-unified-overage-disabled-reason", "credit_limit")

	info := ParseRateLimitHeaders(headers)
	if info == nil {
		t.Fatal("expected non-nil RateLimitInfo")
	}
	if info.RetryAfter != 5*time.Second {
		t.Errorf("expected RetryAfter 5s, got %v", info.RetryAfter)
	}
	if info.UnifiedResetDuration < 115*time.Second || info.UnifiedResetDuration > 125*time.Second {
		t.Errorf("expected UnifiedResetDuration ~120s, got %v", info.UnifiedResetDuration)
	}
	if info.OverageDisabledReason != "credit_limit" {
		t.Errorf("expected OverageDisabledReason 'credit_limit', got %q", info.OverageDisabledReason)
	}
}

func TestParseRateLimitHeaders_PastReset(t *testing.T) {
	// Reset time in the past should result in zero duration
	pastTime := time.Now().Add(-60 * time.Second)
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-reset", fmt.Sprintf("%d", pastTime.Unix()))

	info := ParseRateLimitHeaders(headers)
	if info == nil {
		t.Fatal("expected non-nil RateLimitInfo for past reset")
	}
	// Duration should be clamped to 0 for past reset times
	if info.UnifiedResetDuration != 0 {
		t.Errorf("expected 0 duration for past reset, got %v", info.UnifiedResetDuration)
	}
}

func TestRateLimitHeaders_HTTPError_529(t *testing.T) {
	err := &HTTPError{StatusCode: 529, Body: "overloaded"}
	cat := CategorizeError(err)
	if cat != ErrOverloaded {
		t.Errorf("expected ErrOverloaded, got %v", cat)
	}
}

func TestRateLimitHeaders_HTTPError_429(t *testing.T) {
	err := &HTTPError{StatusCode: 429, Body: "rate limited"}
	cat := CategorizeError(err)
	if cat != ErrRateLimit {
		t.Errorf("expected ErrRateLimit, got %v", cat)
	}
}
