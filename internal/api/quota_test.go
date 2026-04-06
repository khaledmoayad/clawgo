package api

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExtractQuotaFromHeaders_AllHeaders(t *testing.T) {
	resetTime := time.Now().Add(2 * time.Hour).Unix()
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-limit-type", "5_hour")
	headers.Set("anthropic-ratelimit-unified-overage-status", "rejected")
	headers.Set("anthropic-ratelimit-unified-reset", strconv.FormatInt(resetTime, 10))
	headers.Set("anthropic-ratelimit-unified-overage-disabled-reason", "no_subscription")
	headers.Set("anthropic-ratelimit-unified-representative-claim", "5_hour")
	headers.Set("retry-after", "60")

	q := ExtractQuotaFromHeaders(headers)
	if q == nil {
		t.Fatal("expected non-nil QuotaStatus")
	}
	if q.RateLimitType != RateLimit5Hour {
		t.Errorf("expected RateLimit5Hour, got %s", q.RateLimitType)
	}
	if q.Overage != OverageRejected {
		t.Errorf("expected OverageRejected, got %s", q.Overage)
	}
	if q.ResetDuration <= 0 {
		t.Error("expected positive reset duration")
	}
	if q.RetryAfter != 60*time.Second {
		t.Errorf("expected 60s retry-after, got %v", q.RetryAfter)
	}
	if q.OverageDisabledReason != "no_subscription" {
		t.Errorf("expected no_subscription, got %s", q.OverageDisabledReason)
	}
	if q.RepresentativeClaim != "5_hour" {
		t.Errorf("expected 5_hour claim, got %s", q.RepresentativeClaim)
	}
}

func TestExtractQuotaFromHeaders_NoHeaders(t *testing.T) {
	headers := http.Header{}
	q := ExtractQuotaFromHeaders(headers)
	if q != nil {
		t.Error("expected nil for empty headers")
	}
}

func TestExtractQuotaFromHeaders_7Day(t *testing.T) {
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-limit-type", "7_day")
	headers.Set("anthropic-ratelimit-unified-overage-status", "allowed_warning")

	q := ExtractQuotaFromHeaders(headers)
	if q == nil {
		t.Fatal("expected non-nil QuotaStatus")
	}
	if q.RateLimitType != RateLimit7Day {
		t.Errorf("expected RateLimit7Day, got %s", q.RateLimitType)
	}
	if q.Overage != OverageAllowedWarning {
		t.Errorf("expected OverageAllowedWarning, got %s", q.Overage)
	}
}

func TestExtractQuotaFromHeaders_7DayOpus(t *testing.T) {
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-limit-type", "7_day_opus")
	headers.Set("anthropic-ratelimit-unified-overage-status", "allowed")

	q := ExtractQuotaFromHeaders(headers)
	if q == nil {
		t.Fatal("expected non-nil QuotaStatus")
	}
	if q.RateLimitType != RateLimit7DayOpus {
		t.Errorf("expected RateLimit7DayOpus, got %s", q.RateLimitType)
	}
	if q.Overage != OverageAllowed {
		t.Errorf("expected OverageAllowed, got %s", q.Overage)
	}
}

func TestExtractQuotaFromHeaders_PastResetTime(t *testing.T) {
	resetTime := time.Now().Add(-1 * time.Hour).Unix()
	headers := http.Header{}
	headers.Set("anthropic-ratelimit-unified-limit-type", "5_hour")
	headers.Set("anthropic-ratelimit-unified-reset", strconv.FormatInt(resetTime, 10))

	q := ExtractQuotaFromHeaders(headers)
	if q == nil {
		t.Fatal("expected non-nil QuotaStatus")
	}
	if q.ResetDuration != 0 {
		t.Errorf("past reset time should clamp to 0, got %v", q.ResetDuration)
	}
}

func TestExtractQuotaFromHeaders_RetryAfterOnly(t *testing.T) {
	headers := http.Header{}
	headers.Set("retry-after", "30")

	q := ExtractQuotaFromHeaders(headers)
	if q == nil {
		t.Fatal("expected non-nil QuotaStatus")
	}
	if q.RetryAfter != 30*time.Second {
		t.Errorf("expected 30s retry-after, got %v", q.RetryAfter)
	}
}

func TestGetRateLimitMessage_5HourRejected(t *testing.T) {
	q := &QuotaStatus{
		RateLimitType: RateLimit5Hour,
		Overage:       OverageRejected,
		ResetDuration: 2*time.Hour + 30*time.Minute,
	}
	msg := GetRateLimitMessage(q)
	if !strings.Contains(msg, "5-hour") {
		t.Errorf("expected '5-hour' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "2 hours 30 minutes") {
		t.Errorf("expected '2 hours 30 minutes' in message, got: %s", msg)
	}
}

func TestGetRateLimitMessage_7DayRejected_WithFallback(t *testing.T) {
	q := &QuotaStatus{
		RateLimitType:     RateLimit7Day,
		Overage:           OverageRejected,
		ResetDuration:     24 * time.Hour,
		FallbackAvailable: true,
	}
	msg := GetRateLimitMessage(q)
	if !strings.Contains(msg, "7-day") {
		t.Errorf("expected '7-day' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "/model") {
		t.Errorf("expected '/model' fallback hint in message, got: %s", msg)
	}
}

func TestGetRateLimitMessage_7DayOpusRejected(t *testing.T) {
	q := &QuotaStatus{
		RateLimitType: RateLimit7DayOpus,
		Overage:       OverageRejected,
		ResetDuration: 3 * 24 * time.Hour,
	}
	msg := GetRateLimitMessage(q)
	if !strings.Contains(msg, "Opus") {
		t.Errorf("expected 'Opus' in message, got: %s", msg)
	}
}

func TestGetRateLimitMessage_AllowedWarning(t *testing.T) {
	q := &QuotaStatus{
		RateLimitType: RateLimit5Hour,
		Overage:       OverageAllowedWarning,
		ResetDuration: 45 * time.Minute,
	}
	msg := GetRateLimitMessage(q)
	if !strings.Contains(msg, "Approaching") {
		t.Errorf("expected 'Approaching' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "slower responses") {
		t.Errorf("expected 'slower responses' in message, got: %s", msg)
	}
}

func TestGetRateLimitMessage_Allowed(t *testing.T) {
	q := &QuotaStatus{
		RateLimitType: RateLimit5Hour,
		Overage:       OverageAllowed,
		ResetDuration: 1 * time.Hour,
	}
	msg := GetRateLimitMessage(q)
	if !strings.Contains(msg, "Overage usage is allowed") {
		t.Errorf("expected 'Overage usage is allowed' in message, got: %s", msg)
	}
}

func TestGetRateLimitMessage_Nil(t *testing.T) {
	msg := GetRateLimitMessage(nil)
	if msg == "" {
		t.Error("expected non-empty default message for nil quota")
	}
}

func TestGetRateLimitMessage_NoOverage(t *testing.T) {
	q := &QuotaStatus{
		RateLimitType: RateLimit5Hour,
		Overage:       OverageUnknown,
		ResetDuration: 30 * time.Minute,
	}
	msg := GetRateLimitMessage(q)
	if !strings.Contains(msg, "Rate limit exceeded") {
		t.Errorf("expected 'Rate limit exceeded' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "30 minutes") {
		t.Errorf("expected '30 minutes' in message, got: %s", msg)
	}
}

func TestFormatDuration_Hours(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, ""},
		{-1 * time.Minute, ""},
		{30 * time.Second, "less than a minute"},
		{5 * time.Minute, "5 minutes"},
		{1 * time.Minute, "1 minute"},
		{1 * time.Hour, "1 hour"},
		{2 * time.Hour, "2 hours"},
		{2*time.Hour + 15*time.Minute, "2 hours 15 minutes"},
		{1*time.Hour + 1*time.Minute, "1 hour 1 minute"},
	}

	for _, tt := range tests {
		t.Run(tt.d.String(), func(t *testing.T) {
			got := FormatDuration(tt.d)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestExtractQuotaFromError_Nil(t *testing.T) {
	q := ExtractQuotaFromError(nil)
	if q != nil {
		t.Error("expected nil for nil error")
	}
}
