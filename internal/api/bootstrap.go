package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// BootstrapData holds client configuration fetched from the Bootstrap API
// at startup. Contains model options, feature flags, and raw client data
// for forward compatibility.
// Mirrors the TS services/api/bootstrap.ts response shape.
type BootstrapData struct {
	// ClientData holds arbitrary client configuration as raw JSON for
	// forward compatibility -- new fields don't require struct changes.
	ClientData json.RawMessage `json:"client_data"`

	// AdditionalModelOptions lists model IDs available beyond the default.
	AdditionalModelOptions []string `json:"additional_model_options"`
}

// BootstrapClient fetches and caches client configuration from the Anthropic
// Bootstrap API. The result is cached after the first successful fetch.
// Mirrors the TS services/api/bootstrap.ts implementation.
type BootstrapClient struct {
	baseURL     string
	oauthToken  string
	httpClient  *http.Client
	retryConfig RetryConfig

	mu     sync.Mutex
	cached *BootstrapData
}

// NewBootstrapClient creates a BootstrapClient with proxy-aware transport.
func NewBootstrapClient(baseURL, oauthToken string) *BootstrapClient {
	return &BootstrapClient{
		baseURL:    baseURL,
		oauthToken: oauthToken,
		httpClient: &http.Client{
			Transport: NewProxyTransport(),
		},
		retryConfig: DefaultRetryConfig(),
	}
}

// FetchConfig retrieves client configuration from the Bootstrap API.
// Results are cached -- subsequent calls return the cached data without
// making additional HTTP requests.
func (c *BootstrapClient) FetchConfig(ctx context.Context) (*BootstrapData, error) {
	c.mu.Lock()
	if c.cached != nil {
		cached := c.cached
		c.mu.Unlock()
		return cached, nil
	}
	c.mu.Unlock()

	result, err := WithRetry(ctx, c.retryConfig, func(ctx context.Context) (*BootstrapData, error) {
		url := c.baseURL + "/api/claude_cli/bootstrap"
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("bootstrap api: create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.oauthToken)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("bootstrap api: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return nil, &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}
		}

		var data BootstrapData
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("bootstrap api: parse response: %w", err)
		}

		return &data, nil
	})

	if err != nil {
		return nil, err
	}

	// Cache the successful result
	c.mu.Lock()
	c.cached = result
	c.mu.Unlock()

	return result, nil
}
