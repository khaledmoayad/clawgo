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

const bridgeEndpoint = "/v1/bridge"

// Environment represents a registered bridge environment.
type Environment struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// WorkItem represents a unit of work received from the bridge API.
type WorkItem struct {
	SessionID string `json:"session_id"`
	Prompt    string `json:"prompt"`
	OrgUUID   string `json:"org_uuid"`
}

// APIClient communicates with the bridge HTTP API for environment
// registration, work polling, and status reporting.
type APIClient struct {
	baseURL    string
	getToken   func() string
	httpClient *http.Client
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

// RegisterEnvironment registers this machine as a bridge environment.
// POST /v1/bridge/environments
func (c *APIClient) RegisterEnvironment(ctx context.Context, name string) (*Environment, error) {
	body, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return nil, fmt.Errorf("marshal register body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+bridgeEndpoint+"/environments", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create register request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("register environment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("register environment: status %d: %s", resp.StatusCode, string(respBody))
	}

	var env Environment
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("decode register response: %w", err)
	}
	return &env, nil
}

// PollWork polls for available work items for the given environment.
// GET /v1/bridge/environments/{envID}/work
func (c *APIClient) PollWork(ctx context.Context, envID string) ([]WorkItem, error) {
	url := fmt.Sprintf("%s%s/environments/%s/work", c.baseURL, bridgeEndpoint, envID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create poll request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("poll work: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("poll work: status %d: %s", resp.StatusCode, string(respBody))
	}

	var items []WorkItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("decode poll response: %w", err)
	}
	return items, nil
}

// ReportStatus updates the environment status on the bridge API.
// PUT /v1/bridge/environments/{envID}/status
func (c *APIClient) ReportStatus(ctx context.Context, envID string, status string) error {
	url := fmt.Sprintf("%s%s/environments/%s/status", c.baseURL, bridgeEndpoint, envID)
	body, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		return fmt.Errorf("marshal status body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create status request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("report status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("report status: status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// setHeaders sets common headers (auth, content-type) on an HTTP request.
func (c *APIClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.getToken())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
}
