package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IngressEntry represents a single transcript entry for the Session Ingress API.
// Each entry is a conversation event (message, tool use, etc.) that gets
// streamed to claude.ai for web-based session viewing.
type IngressEntry struct {
	UUID      string          `json:"uuid"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp time.Time       `json:"timestamp"`
}

// SessionIngressClient streams conversation transcripts to the Anthropic
// Session Ingress API, enabling web-based session viewing on claude.ai.
// Uses Last-Uuid header for optimistic concurrency control.
// Mirrors the TS services/api/sessionIngress.ts implementation.
type SessionIngressClient struct {
	baseURL     string
	oauthToken  string
	httpClient  *http.Client
	retryConfig RetryConfig

	// lastUUID tracks the UUID of the last successfully appended entry,
	// sent as the Last-Uuid header for optimistic concurrency.
	lastUUID string
}

// NewSessionIngressClient creates a SessionIngressClient with proxy-aware transport.
func NewSessionIngressClient(baseURL, oauthToken string) *SessionIngressClient {
	return &SessionIngressClient{
		baseURL:    baseURL,
		oauthToken: oauthToken,
		httpClient: &http.Client{
			Transport: NewProxyTransport(),
		},
		retryConfig: DefaultRetryConfig(),
	}
}

// Append sends a transcript entry to the Session Ingress API.
// Uses the Last-Uuid header for optimistic concurrency: the server rejects
// the append if the provided Last-Uuid doesn't match, preventing lost updates.
// On success, updates the internal lastUUID to the entry's UUID.
func (c *SessionIngressClient) Append(ctx context.Context, sessionID string, entry IngressEntry) error {
	_, err := WithRetry(ctx, c.retryConfig, func(ctx context.Context) (struct{}, error) {
		body, err := json.Marshal(entry)
		if err != nil {
			return struct{}{}, fmt.Errorf("session ingress: marshal entry: %w", err)
		}

		url := fmt.Sprintf("%s/api/session_ingress/%s", c.baseURL, sessionID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
		if err != nil {
			return struct{}{}, fmt.Errorf("session ingress: create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.oauthToken)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Content-Type", "application/json")

		// Optimistic concurrency: send the last known UUID
		if c.lastUUID != "" {
			req.Header.Set("Last-Uuid", c.lastUUID)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return struct{}{}, fmt.Errorf("session ingress append: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return struct{}{}, &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(respBody),
			}
		}

		return struct{}{}, nil
	})

	if err != nil {
		return err
	}

	// Update lastUUID on successful append
	c.lastUUID = entry.UUID
	return nil
}

// Fetch retrieves existing transcript entries for a session.
// Used for session resume: fetches the current transcript so the client
// can continue from where it left off.
func (c *SessionIngressClient) Fetch(ctx context.Context, sessionID string) ([]IngressEntry, error) {
	result, err := WithRetry(ctx, c.retryConfig, func(ctx context.Context) ([]IngressEntry, error) {
		url := fmt.Sprintf("%s/api/session_ingress/%s", c.baseURL, sessionID)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("session ingress: create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.oauthToken)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("session ingress fetch: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			return nil, &HTTPError{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}
		}

		var entries []IngressEntry
		if err := json.Unmarshal(body, &entries); err != nil {
			return nil, fmt.Errorf("session ingress: parse response: %w", err)
		}

		return entries, nil
	})

	return result, err
}
