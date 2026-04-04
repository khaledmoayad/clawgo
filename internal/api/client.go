package api

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
)

const (
	// DefaultModel is the default Anthropic model to use.
	DefaultModel = "claude-sonnet-4-20250514"

	// DefaultMaxTokens is the default maximum number of tokens in a response.
	DefaultMaxTokens = 16384
)

// Client wraps the Anthropic SDK client with ClawGo-specific configuration.
type Client struct {
	SDK           anthropic.Client
	Model         string
	MaxTokens     int64
	FallbackModel string
}

// NewClient creates an Anthropic API client.
// apiKey: explicit key (from config). If empty, the SDK falls back to the
// ANTHROPIC_API_KEY environment variable.
// baseURL: optional API base URL override (from ANTHROPIC_BASE_URL or
// CLAUDE_CODE_API_BASE_URL).
// Returns an error if no API key is available from either source.
//
// This is a backward-compatible wrapper around NewProviderClient.
func NewClient(apiKey, baseURL string) (*Client, error) {
	return NewProviderClient(context.Background(), ProviderClientConfig{
		APIKey:  apiKey,
		BaseURL: baseURL,
	})
}

// SwitchToFallback switches the client to use the fallback model.
// Returns true if the switch was made, false if no fallback is available
// or the model is already the fallback.
func (c *Client) SwitchToFallback() bool {
	if c.FallbackModel == "" || c.FallbackModel == c.Model {
		return false
	}
	c.Model = c.FallbackModel
	c.FallbackModel = "" // Prevent double-fallback
	return true
}
