package teleport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	sessionsEndpoint = "/v1/sessions"
	requestTimeout   = 10 * time.Second
	// eventTimeout is longer to allow for worker cold start.
	eventTimeout = 30 * time.Second
	maxRetries   = 4

	anthropicVersion = "2023-06-01"
)

// retryDelays matches the TS retry schedule: [2000, 4000, 8000, 16000] ms.
var retryDelays = []time.Duration{
	2 * time.Second,
	4 * time.Second,
	8 * time.Second,
	16 * time.Second,
}

// Client is an HTTP client for the Teleport session management API.
// It handles OAuth authentication and automatic retry with exponential backoff
// for rate-limited (429) and server error (5xx) responses.
type Client struct {
	apiBaseURL string
	getToken   func() string
	getOrgUUID func() string
	httpClient *http.Client
}

// NewClient creates a new Teleport API client.
func NewClient(apiBaseURL string, getToken func() string) *Client {
	return &Client{
		apiBaseURL: apiBaseURL,
		getToken:   getToken,
		getOrgUUID: func() string { return "" },
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

// NewClientWithOrg creates a new Teleport API client that includes organization UUID headers.
func NewClientWithOrg(apiBaseURL string, getToken func() string, getOrgUUID func() string) *Client {
	return &Client{
		apiBaseURL: apiBaseURL,
		getToken:   getToken,
		getOrgUUID: getOrgUUID,
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

// GetOAuthHeaders returns the standard OAuth headers for Teleport API requests.
func GetOAuthHeaders(accessToken string) map[string]string {
	return map[string]string{
		"Authorization":    "Bearer " + accessToken,
		"Content-Type":     "application/json",
		"anthropic-version": anthropicVersion,
	}
}

// CreateSession creates a new remote session in the specified environment.
func (c *Client) CreateSession(ctx context.Context, env string) (*SessionResource, error) {
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

	var info SessionResource
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding create session response: %w", err)
	}
	return &info, nil
}

// FetchSession retrieves information about an existing session by ID.
// Returns specific error messages for 404 (not found) and 401 (expired token).
func (c *Client) FetchSession(ctx context.Context, sessionID string) (*SessionResource, error) {
	path := fmt.Sprintf("%s/%s", sessionsEndpoint, sessionID)
	resp, err := c.doWithRetry(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching session: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// Success
	case http.StatusNotFound:
		return nil, fmt.Errorf("session %s not found", sessionID)
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("authentication expired; please run 'claude login' to re-authenticate")
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch session returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var info SessionResource
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding fetch session response: %w", err)
	}
	return &info, nil
}

// ResumeSession resumes a paused or suspended session.
func (c *Client) ResumeSession(ctx context.Context, sessionID string) (*SessionResource, error) {
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

	var info SessionResource
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding resume session response: %w", err)
	}
	return &info, nil
}

// SendEventToRemoteSession sends a user event to a remote session.
// Uses a 30-second timeout to allow for worker cold start.
// Returns true on success (200/201), false otherwise.
func (c *Client) SendEventToRemoteSession(ctx context.Context, sessionID string, content interface{}, uuid string) (bool, error) {
	event := SessionEvent{
		UUID:            uuid,
		SessionID:       sessionID,
		Type:            "user",
		ParentToolUseID: nil,
		Message: SessionEventMessage{
			Role:    "user",
			Content: content,
		},
	}
	reqBody := SendEventsRequest{
		Events: []SessionEvent{event},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("marshalling event body: %w", err)
	}

	path := fmt.Sprintf("%s/%s/events", sessionsEndpoint, sessionID)

	// Use a longer timeout for event sending (worker cold start)
	eventCtx, cancel := context.WithTimeout(ctx, eventTimeout)
	defer cancel()

	resp, err := c.doWithRetry(eventCtx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("sending event to session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return true, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("send event returned status %d: %s", resp.StatusCode, string(respBody))
}

// UpdateSessionTitle updates the title of a remote session.
// Returns true on success (200).
func (c *Client) UpdateSessionTitle(ctx context.Context, sessionID, title string) (bool, error) {
	reqBody := UpdateSessionRequest{Title: title}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("marshalling title update body: %w", err)
	}

	path := fmt.Sprintf("%s/%s", sessionsEndpoint, sessionID)
	resp, err := c.doWithRetry(ctx, http.MethodPatch, path, bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("updating session title: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("update session title returned status %d: %s", resp.StatusCode, string(respBody))
}

// FetchCodeSessionsFromSessionsAPI retrieves all sessions from the sessions API.
// Uses doWithRetry for network resilience.
func (c *Client) FetchCodeSessionsFromSessionsAPI(ctx context.Context) ([]SessionResource, error) {
	resp, err := c.doWithRetry(ctx, http.MethodGet, sessionsEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching sessions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch sessions returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var listResp ListSessionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, fmt.Errorf("decoding sessions list response: %w", err)
	}
	return listResp.Data, nil
}

// GetBranchFromSession extracts the first branch name from a session's
// git_repository outcomes.
func GetBranchFromSession(session SessionResource) string {
	for _, rawOutcome := range session.SessionContext.Outcomes {
		var outcome GitRepositoryOutcome
		if err := json.Unmarshal(rawOutcome, &outcome); err != nil {
			continue
		}
		if outcome.Type != "git_repository" {
			continue
		}
		if len(outcome.GitInfo.Branches) > 0 {
			return outcome.GitInfo.Branches[0]
		}
	}
	return ""
}

// IsTransientNetworkError checks whether an error is a transient network
// error or a 5xx server error that warrants a retry.
func IsTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// Check for network-level errors
	if _, ok := err.(net.Error); ok {
		return true
	}
	// Check for common transient error strings
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "server returned status 5")
}

// doWithRetry performs an HTTP request with automatic retry on 429 and 5xx errors.
// Uses retry delays matching TS: [2s, 4s, 8s, 16s] (4 retries).
func (c *Client) doWithRetry(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.apiBaseURL + path

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Use predefined retry delays matching TS schedule
			delayIdx := attempt - 1
			if delayIdx >= len(retryDelays) {
				delayIdx = len(retryDelays) - 1
			}
			delay := retryDelays[delayIdx]
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

		// Set standard OAuth headers
		req.Header.Set("Authorization", "Bearer "+c.getToken())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-version", anthropicVersion)
		req.Header.Set("anthropic-beta", CCR_BYOC_BETA)

		// Set organization UUID header if available
		if orgUUID := c.getOrgUUID(); orgUUID != "" {
			req.Header.Set("x-organization-uuid", orgUUID)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			if !IsTransientNetworkError(err) {
				return nil, err
			}
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
