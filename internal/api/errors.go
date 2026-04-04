package api

import (
	"errors"
	"net"

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
