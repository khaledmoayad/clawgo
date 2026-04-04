package api

import (
	"net/http"
	"os"
	"time"
)

// NewProxyTransport creates an http.Transport that respects HTTP_PROXY,
// HTTPS_PROXY, and NO_PROXY environment variables via Go's stdlib
// http.ProxyFromEnvironment. This provides transparent proxy support
// for corporate network environments.
func NewProxyTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSHandshakeTimeout:  10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}
}

// ProxyURL returns the effective proxy URL for informational display.
// Checks HTTPS_PROXY first, then HTTP_PROXY. Returns empty string if
// no proxy is configured.
func ProxyURL() string {
	if url := os.Getenv("HTTPS_PROXY"); url != "" {
		return url
	}
	if url := os.Getenv("https_proxy"); url != "" {
		return url
	}
	if url := os.Getenv("HTTP_PROXY"); url != "" {
		return url
	}
	if url := os.Getenv("http_proxy"); url != "" {
		return url
	}
	return ""
}

// IsProxyConfigured returns true if HTTP_PROXY or HTTPS_PROXY is set
// (checking both upper and lower case variants).
func IsProxyConfigured() bool {
	return ProxyURL() != ""
}
