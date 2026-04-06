package api

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RateLimitType describes the type of rate limit applied.
// Matches Claude Code's unified rate limit types.
type RateLimitType string

const (
	RateLimit5Hour    RateLimitType = "5_hour"
	RateLimit7Day     RateLimitType = "7_day"
	RateLimit7DayOpus RateLimitType = "7_day_opus"
	RateLimitUnknown  RateLimitType = "unknown"
)

// OverageStatus describes the account's overage status with the rate limit.
// Matches Claude Code's overage status values from the API.
type OverageStatus string

const (
	OverageAllowed        OverageStatus = "allowed"
	OverageAllowedWarning OverageStatus = "allowed_warning"
	OverageRejected       OverageStatus = "rejected"
	OverageUnknown        OverageStatus = ""
)

// QuotaStatus holds parsed rate limit information from API responses.
// This is the structured form consumed by the TUI for user-facing display.
type QuotaStatus struct {
	// RateLimitType is the type of rate limit applied (5h, 7d, 7d-opus).
	RateLimitType RateLimitType

	// Overage describes whether additional usage is allowed, warned, or rejected.
	Overage OverageStatus

	// ResetAt is the time when the rate limit window resets.
	ResetAt time.Time

	// ResetDuration is the computed duration until reset (for display).
	ResetDuration time.Duration

	// FallbackAvailable indicates whether a fallback model is available.
	FallbackAvailable bool

	// RetryAfter is the retry-after duration from the server (if any).
	RetryAfter time.Duration

	// OverageDisabledReason explains why overage is disabled (if applicable).
	OverageDisabledReason string

	// RepresentativeClaim is the representative rate limit claim identifier
	// from the API (e.g., "5_hour", "7_day").
	RepresentativeClaim string
}

// ExtractQuotaFromHeaders parses all unified rate limit headers from an API
// response. Returns nil if no quota headers are present.
//
// Recognized headers (matching Claude Code's unified rate limit system):
//   - anthropic-ratelimit-unified-limit-type: "5_hour", "7_day", "7_day_opus"
//   - anthropic-ratelimit-unified-overage-status: "allowed", "allowed_warning", "rejected"
//   - anthropic-ratelimit-unified-reset: Unix epoch seconds
//   - anthropic-ratelimit-unified-overage-disabled-reason: why overage is disabled
//   - anthropic-ratelimit-unified-representative-claim: claim identifier
//   - retry-after: seconds until safe to retry
func ExtractQuotaFromHeaders(headers http.Header) *QuotaStatus {
	limitType := headers.Get("anthropic-ratelimit-unified-limit-type")
	overageStr := headers.Get("anthropic-ratelimit-unified-overage-status")
	resetStr := headers.Get("anthropic-ratelimit-unified-reset")
	overageReason := headers.Get("anthropic-ratelimit-unified-overage-disabled-reason")
	repClaim := headers.Get("anthropic-ratelimit-unified-representative-claim")
	retryAfterStr := headers.Get("retry-after")

	// Return nil if no quota headers are present
	if limitType == "" && overageStr == "" && resetStr == "" && retryAfterStr == "" {
		return nil
	}

	q := &QuotaStatus{
		OverageDisabledReason: overageReason,
		RepresentativeClaim:   repClaim,
	}

	// Parse limit type
	switch strings.ToLower(limitType) {
	case "5_hour":
		q.RateLimitType = RateLimit5Hour
	case "7_day":
		q.RateLimitType = RateLimit7Day
	case "7_day_opus":
		q.RateLimitType = RateLimit7DayOpus
	default:
		q.RateLimitType = RateLimitUnknown
	}

	// Parse overage status
	switch strings.ToLower(overageStr) {
	case "allowed":
		q.Overage = OverageAllowed
	case "allowed_warning":
		q.Overage = OverageAllowedWarning
	case "rejected":
		q.Overage = OverageRejected
	default:
		q.Overage = OverageUnknown
	}

	// Parse reset time
	if resetStr != "" {
		resetUnix, err := strconv.ParseInt(resetStr, 10, 64)
		if err == nil {
			q.ResetAt = time.Unix(resetUnix, 0)
			q.ResetDuration = time.Until(q.ResetAt)
			if q.ResetDuration < 0 {
				q.ResetDuration = 0
			}
		}
	}

	// Parse retry-after
	if retryAfterStr != "" {
		seconds, err := strconv.Atoi(retryAfterStr)
		if err == nil && seconds > 0 {
			q.RetryAfter = time.Duration(seconds) * time.Second
		}
	}

	return q
}

// ExtractQuotaFromError parses quota information from a 429 error response.
// This delegates to ExtractQuotaFromHeaders using the error's response headers.
// If the error does not carry response headers, returns nil.
func ExtractQuotaFromError(err error) *QuotaStatus {
	if err == nil {
		return nil
	}

	// Try to extract headers from HTTPError
	type headerProvider interface {
		ResponseHeaders() http.Header
	}
	if hp, ok := err.(headerProvider); ok {
		return ExtractQuotaFromHeaders(hp.ResponseHeaders())
	}

	return nil
}

// GetRateLimitMessage generates a user-facing rate limit message based on
// the quota status. This matches Claude Code's getRateLimitErrorMessage().
func GetRateLimitMessage(q *QuotaStatus) string {
	if q == nil {
		return "Rate limit exceeded. Please wait before making another request."
	}

	resetStr := FormatDuration(q.ResetDuration)

	switch q.Overage {
	case OverageRejected:
		return getRejectedMessage(q, resetStr)
	case OverageAllowedWarning:
		return getWarningMessage(q, resetStr)
	case OverageAllowed:
		return getAllowedMessage(q, resetStr)
	default:
		// No overage info -- generic rate limit message
		if resetStr != "" {
			return fmt.Sprintf("Rate limit exceeded. Resets in %s.", resetStr)
		}
		return "Rate limit exceeded. Please wait before making another request."
	}
}

// getRejectedMessage builds a message for when overage is rejected (hard limit hit).
func getRejectedMessage(q *QuotaStatus, resetStr string) string {
	var sb strings.Builder

	switch q.RateLimitType {
	case RateLimit5Hour:
		sb.WriteString("You've hit the 5-hour usage limit.")
	case RateLimit7Day:
		sb.WriteString("You've hit the 7-day usage limit.")
	case RateLimit7DayOpus:
		sb.WriteString("You've hit the 7-day Opus usage limit.")
	default:
		sb.WriteString("You've hit the usage limit.")
	}

	if resetStr != "" {
		sb.WriteString(fmt.Sprintf(" Resets in %s.", resetStr))
	}

	if q.FallbackAvailable {
		sb.WriteString(" Use /model to switch to a different model.")
	}

	return sb.String()
}

// getWarningMessage builds a message for the overage warning state.
func getWarningMessage(q *QuotaStatus, resetStr string) string {
	var sb strings.Builder

	switch q.RateLimitType {
	case RateLimit5Hour:
		sb.WriteString("Approaching 5-hour usage limit.")
	case RateLimit7Day:
		sb.WriteString("Approaching 7-day usage limit.")
	case RateLimit7DayOpus:
		sb.WriteString("Approaching 7-day Opus usage limit.")
	default:
		sb.WriteString("Approaching usage limit.")
	}

	if resetStr != "" {
		sb.WriteString(fmt.Sprintf(" Resets in %s.", resetStr))
	}

	sb.WriteString(" You may experience slower responses.")

	return sb.String()
}

// getAllowedMessage builds a message when overage is allowed (no concern).
func getAllowedMessage(q *QuotaStatus, resetStr string) string {
	if resetStr != "" {
		return fmt.Sprintf("Rate limit applied. Resets in %s. Overage usage is allowed.", resetStr)
	}
	return "Rate limit applied. Overage usage is allowed."
}

// FormatDuration formats a duration for user-facing display.
// Examples: "2 hours 30 minutes", "45 minutes", "5 minutes",
// "less than a minute".
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}

	hours := int(math.Floor(d.Hours()))
	minutes := int(math.Floor(d.Minutes())) % 60

	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%d %s %d %s",
			hours, pluralize("hour", hours),
			minutes, pluralize("minute", minutes))
	}
	if hours > 0 {
		return fmt.Sprintf("%d %s", hours, pluralize("hour", hours))
	}
	if minutes > 0 {
		return fmt.Sprintf("%d %s", minutes, pluralize("minute", minutes))
	}
	return "less than a minute"
}

// pluralize returns the singular or plural form based on count.
func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}
