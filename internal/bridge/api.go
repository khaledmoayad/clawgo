package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient communicates with the bridge HTTP API for environment
// registration, work polling, acknowledgement, heartbeating, and cleanup.
type APIClient struct {
	baseURL    string
	getToken   func() string
	httpClient *http.Client
	onDebug    func(string)
}

// NewAPIClient creates a new bridge API client.
func NewAPIClient(baseURL string, getToken func() string) *APIClient {
	return &APIClient{
		baseURL:  baseURL,
		getToken: getToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetDebugFunc sets a debug logging callback.
func (c *APIClient) SetDebugFunc(fn func(string)) {
	c.onDebug = fn
}

func (c *APIClient) debug(msg string) {
	if c.onDebug != nil {
		c.onDebug(msg)
	}
}

// RegisterBridgeEnvironment registers this machine as a bridge environment.
// POST /v1/environments/bridge
func (c *APIClient) RegisterBridgeEnvironment(ctx context.Context, config BridgeConfig) (envID, envSecret string, err error) {
	body := map[string]interface{}{
		"machine_name": config.EnvironmentName,
		"directory":    config.Dir,
		"branch":       config.Branch,
		"git_repo_url": config.GitRepoURL,
		"max_sessions": config.MaxConcurrentSessions,
		"metadata":     map[string]string{"worker_type": config.WorkerType},
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", "", fmt.Errorf("marshal register body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/environments/bridge", bytes.NewReader(data))
	if err != nil {
		return "", "", fmt.Errorf("create register request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("register environment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("register environment: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		EnvironmentID     string `json:"environment_id"`
		EnvironmentSecret string `json:"environment_secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decode register response: %w", err)
	}

	c.debug(fmt.Sprintf("[bridge:api] registered env=%s", result.EnvironmentID))
	return result.EnvironmentID, result.EnvironmentSecret, nil
}

// PollForWork polls for a single work item for the given environment.
// GET /v1/environments/{envID}/work/poll
// Returns nil when no work is available.
func (c *APIClient) PollForWork(ctx context.Context, envID, envSecret string, reclaimOlderThanMs *int) (*WorkResponse, error) {
	url := fmt.Sprintf("%s/v1/environments/%s/work/poll", c.baseURL, envID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create poll request: %w", err)
	}
	// Poll uses environment secret for auth, not the user OAuth token
	req.Header.Set("Authorization", "Bearer "+envSecret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	if reclaimOlderThanMs != nil {
		q := req.URL.Query()
		q.Set("reclaim_older_than_ms", fmt.Sprintf("%d", *reclaimOlderThanMs))
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll work: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("poll work: status %d: %s", resp.StatusCode, string(respBody))
	}

	// Read the body; empty body or "null" means no work available
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read poll response: %w", err)
	}
	if len(respBody) == 0 || string(respBody) == "null" {
		return nil, nil
	}

	var work WorkResponse
	if err := json.Unmarshal(respBody, &work); err != nil {
		return nil, fmt.Errorf("decode poll response: %w", err)
	}
	if work.ID == "" {
		return nil, nil
	}

	c.debug(fmt.Sprintf("[bridge:api] poll -> workId=%s type=%s", work.ID, work.Data.Type))
	return &work, nil
}

// AcknowledgeWork confirms receipt of a work item.
// POST /v1/environments/{envID}/work/{workID}/ack
func (c *APIClient) AcknowledgeWork(ctx context.Context, envID, workID, sessionToken string) error {
	url := fmt.Sprintf("%s/v1/environments/%s/work/%s/ack", c.baseURL, envID, workID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("create ack request: %w", err)
	}
	// Acknowledge uses session ingress token
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("acknowledge work: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("acknowledge work: status %d: %s", resp.StatusCode, string(respBody))
	}

	c.debug(fmt.Sprintf("[bridge:api] ack workId=%s -> %d", workID, resp.StatusCode))
	return nil
}

// HeartbeatWork sends a heartbeat for an active work item, extending its lease.
// POST /v1/environments/{envID}/work/{workID}/heartbeat
func (c *APIClient) HeartbeatWork(ctx context.Context, envID, workID, sessionToken string) (leaseExtended bool, state string, err error) {
	url := fmt.Sprintf("%s/v1/environments/%s/work/%s/heartbeat", c.baseURL, envID, workID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return false, "", fmt.Errorf("create heartbeat request: %w", err)
	}
	// Heartbeat uses session ingress token (lightweight JWT, no DB hit)
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("heartbeat work: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return false, "", fmt.Errorf("heartbeat work: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result HeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, "", fmt.Errorf("decode heartbeat response: %w", err)
	}

	c.debug(fmt.Sprintf("[bridge:api] heartbeat workId=%s lease=%v state=%s", workID, result.LeaseExtended, result.State))
	return result.LeaseExtended, result.State, nil
}

// ArchiveSession archives a completed session so it no longer appears as active.
// POST /v1/sessions/{sessionID}/archive
func (c *APIClient) ArchiveSession(ctx context.Context, sessionID string) error {
	url := fmt.Sprintf("%s/v1/sessions/%s/archive", c.baseURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("create archive request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("archive session: %w", err)
	}
	defer resp.Body.Close()

	// 409 = already archived (idempotent, not an error)
	if resp.StatusCode == http.StatusConflict {
		c.debug(fmt.Sprintf("[bridge:api] archive session=%s -> 409 (already archived)", sessionID))
		return nil
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("archive session: status %d: %s", resp.StatusCode, string(respBody))
	}

	c.debug(fmt.Sprintf("[bridge:api] archive session=%s -> %d", sessionID, resp.StatusCode))
	return nil
}

// StopWork stops a work item via the environments API.
// POST /v1/environments/{envID}/work/{workID}/stop
func (c *APIClient) StopWork(ctx context.Context, envID, workID string, force bool) error {
	body, _ := json.Marshal(map[string]bool{"force": force})
	url := fmt.Sprintf("%s/v1/environments/%s/work/%s/stop", c.baseURL, envID, workID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create stop request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("stop work: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop work: status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// DeregisterEnvironment removes the bridge environment on graceful shutdown.
// DELETE /v1/environments/bridge/{envID}
func (c *APIClient) DeregisterEnvironment(ctx context.Context, envID string) error {
	url := fmt.Sprintf("%s/v1/environments/bridge/%s", c.baseURL, envID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("create deregister request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deregister environment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deregister environment: status %d: %s", resp.StatusCode, string(respBody))
	}

	c.debug(fmt.Sprintf("[bridge:api] deregistered env=%s", envID))
	return nil
}

// setHeaders sets common headers (auth, content-type) on an HTTP request.
func (c *APIClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.getToken())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
}
