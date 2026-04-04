package teleport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

const (
	sessionsEndpoint = "/v1/sessions"
	requestTimeout   = 10 * time.Second
	maxRetries       = 3

	initialRetryDelay = 1 * time.Second
)

// SessionInfo represents a remote session returned by the Teleport API.
type SessionInfo struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	Environment string    `json:"environment"`
}

// Client is an HTTP client for the Teleport session management API.
// It handles OAuth authentication and automatic retry with exponential backoff
// for rate-limited (429) and server error (5xx) responses.
type Client struct {
	apiBaseURL string
	getToken   func() string
	httpClient *http.Client
}

// NewClient creates a new Teleport API client.
func NewClient(apiBaseURL string, getToken func() string) *Client {
	return &Client{
		apiBaseURL: apiBaseURL,
		getToken:   getToken,
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

// CreateSession creates a new remote session in the specified environment.
func (c *Client) CreateSession(ctx context.Context, env string) (*SessionInfo, error) {
	body, err := json.Marshal(map[string]string{"environment": env})
	if err != nil {
		return nil, fmt.Errorf("marshalling create session body: %w", err)
	}

	resp, err := c.doWithRetry(ctx, http.MethodPost, sessionsEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create session returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var info SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding create session response: %w", err)
	}
	return &info, nil
}

// FetchSession retrieves information about an existing session by ID.
func (c *Client) FetchSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	path := fmt.Sprintf("%s/%s", sessionsEndpoint, sessionID)
	resp, err := c.doWithRetry(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch session returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var info SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding fetch session response: %w", err)
	}
	return &info, nil
}

// ResumeSession resumes a paused or suspended session.
func (c *Client) ResumeSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	path := fmt.Sprintf("%s/%s/resume", sessionsEndpoint, sessionID)
	resp, err := c.doWithRetry(ctx, http.MethodPost, path, nil)
	if err != nil {
		return nil, fmt.Errorf("resuming session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("resume session returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var info SessionInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding resume session response: %w", err)
	}
	return &info, nil
}

// doWithRetry performs an HTTP request with automatic retry on 429 and 5xx errors.
// Uses exponential backoff: 1s, 2s, 4s (up to maxRetries attempts).
func (c *Client) doWithRetry(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.apiBaseURL + path

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: initialRetryDelay * 2^(attempt-1)
			delay := time.Duration(float64(initialRetryDelay) * math.Pow(2, float64(attempt-1)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// If body is a ReadSeeker, reset it for retry
			if seeker, ok := body.(io.ReadSeeker); ok {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return nil, fmt.Errorf("resetting request body for retry: %w", err)
				}
			}
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.getToken())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Check if the response is retriable
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("server returned status %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
}
