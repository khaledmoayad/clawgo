package api

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// ErrorCategory classifies API errors for retry and display decisions.
type ErrorCategory string

const (
	ErrRateLimit   ErrorCategory = "rate_limit"
	ErrOverloaded  ErrorCategory = "overloaded"
	ErrServerError ErrorCategory = "server_error"
	ErrAuth        ErrorCategory = "auth"
	ErrClientError ErrorCategory = "client_error"
	ErrNetwork     ErrorCategory = "network"
	ErrUnknown     ErrorCategory = "unknown"
)

// CategorizeError inspects an API error and returns its category.
// It checks for anthropic.APIError status codes: 429 -> rate_limit,
// 529 -> overloaded, 500-528 -> server_error, 401/403 -> auth,
// 400-499 -> client_error. Network errors -> network.
// Also handles HTTPError from auxiliary API clients (Files, Bootstrap, Session Ingress).
func CategorizeError(err error) ErrorCategory {
	if err == nil {
		return ErrUnknown
	}

	// Check for Anthropic SDK API errors with HTTP status codes
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return categorizeStatusCode(apiErr.StatusCode)
	}

	// Check for HTTPError from auxiliary API clients
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return categorizeStatusCode(httpErr.StatusCode)
	}

	// Check for network errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ErrNetwork
	}

	// Check for DNS errors specifically
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrNetwork
	}

	return ErrUnknown
}

// categorizeStatusCode maps an HTTP status code to an ErrorCategory.
func categorizeStatusCode(code int) ErrorCategory {
	switch {
	case code == 429:
		return ErrRateLimit
	case code == 529:
		return ErrOverloaded
	case code >= 500 && code <= 528:
		return ErrServerError
	case code == 401 || code == 403:
		return ErrAuth
	case code >= 400 && code < 500:
		return ErrClientError
	}
	return ErrUnknown
}

// IsRetryable returns true if the error should trigger a retry.
// Retryable categories: rate_limit, overloaded, server_error, network.
func IsRetryable(err error) bool {
	cat := CategorizeError(err)
	switch cat {
	case ErrRateLimit, ErrOverloaded, ErrServerError, ErrNetwork:
		return true
	default:
		return false
	}
}

// RateLimitInfo holds parsed rate limit information from Anthropic API response headers.
type RateLimitInfo struct {
	// RetryAfter is the duration from the "retry-after" header (in seconds).
	RetryAfter time.Duration

	// UnifiedResetDuration is the duration until the rate limit resets,
	// computed from the "anthropic-ratelimit-unified-reset" header (Unix epoch seconds).
	// Clamped to 0 if the reset time is in the past.
	UnifiedResetDuration time.Duration

	// OverageDisabledReason from the "anthropic-ratelimit-unified-overage-disabled-reason" header.
	// Indicates why overage (extra usage beyond quota) is disabled for this account.
	OverageDisabledReason string
}

// ParseRateLimitHeaders extracts rate limit information from HTTP response headers.
// Returns nil if no rate limit headers are present.
//
// Recognized headers:
//   - "retry-after": integer seconds until the client should retry
//   - "anthropic-ratelimit-unified-reset": Unix epoch seconds when the rate limit window resets
//   - "anthropic-ratelimit-unified-overage-disabled-reason": why overage is disabled
func ParseRateLimitHeaders(headers http.Header) *RateLimitInfo {
	retryAfterStr := headers.Get("retry-after")
	resetStr := headers.Get("anthropic-ratelimit-unified-reset")
	overageReason := headers.Get("anthropic-ratelimit-unified-overage-disabled-reason")

	// Return nil if no rate limit headers are present
	if retryAfterStr == "" && resetStr == "" && overageReason == "" {
		return nil
	}

	info := &RateLimitInfo{}
	hasData := false

	// Parse retry-after as integer seconds
	if retryAfterStr != "" {
		seconds, err := strconv.Atoi(retryAfterStr)
		if err == nil && seconds > 0 {
			info.RetryAfter = time.Duration(seconds) * time.Second
			hasData = true
		}
	}

	// Parse unified reset as Unix epoch seconds and compute duration until reset
	if resetStr != "" {
		resetUnix, err := strconv.ParseInt(resetStr, 10, 64)
		if err == nil {
			resetTime := time.Unix(resetUnix, 0)
			duration := time.Until(resetTime)
			if duration < 0 {
				duration = 0
			}
			info.UnifiedResetDuration = duration
			hasData = true
		}
	}

	// Overage disabled reason
	if overageReason != "" {
		info.OverageDisabledReason = overageReason
		hasData = true
	}

	if !hasData {
		return nil
	}

	return info
}
