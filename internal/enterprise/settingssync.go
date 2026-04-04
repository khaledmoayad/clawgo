package enterprise

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	settingsSyncEndpoint    = "/v1/settings/sync"
	settingsSyncFetchTimeout = 10 * time.Second
)

// SettingsSyncManager handles cross-environment settings synchronization.
// In interactive CLI mode (isRemote=false), it uploads local settings.
// In CCR mode (isRemote=true), it downloads remote settings.
type SettingsSyncManager struct {
	apiBaseURL string
	getToken   func() string
	httpClient *http.Client
	isRemote   bool
}

// NewSettingsSyncManager creates a settings sync manager.
// Set isRemote=true for Claude Code Remote (CCR) environments,
// false for interactive CLI sessions.
func NewSettingsSyncManager(apiBaseURL string, getToken func() string, isRemote bool) *SettingsSyncManager {
	return &SettingsSyncManager{
		apiBaseURL: apiBaseURL,
		getToken:   getToken,
		httpClient: &http.Client{Timeout: settingsSyncFetchTimeout},
		isRemote:   isRemote,
	}
}

// Upload sends local settings to the sync endpoint.
// Only meaningful in interactive CLI mode (isRemote=false).
func (m *SettingsSyncManager) Upload(ctx context.Context, settings json.RawMessage) error {
	url := m.apiBaseURL + settingsSyncEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(settings))
	if err != nil {
		return fmt.Errorf("creating settings upload request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token := m.getToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("uploading settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("settings upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Download fetches settings from the sync endpoint.
// Only meaningful in CCR mode (isRemote=true).
func (m *SettingsSyncManager) Download(ctx context.Context) (json.RawMessage, error) {
	url := m.apiBaseURL + settingsSyncEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating settings download request: %w", err)
	}

	if token := m.getToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("settings download failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading settings response: %w", err)
	}

	return json.RawMessage(body), nil
}

// Sync performs the appropriate sync operation based on environment:
// - If isRemote (CCR): downloads settings from the server
// - If !isRemote (interactive CLI): uploads local settings to the server
// Returns the effective settings (downloaded remote or passed-through local).
func (m *SettingsSyncManager) Sync(ctx context.Context, localSettings json.RawMessage) (json.RawMessage, error) {
	if m.isRemote {
		return m.Download(ctx)
	}
	if err := m.Upload(ctx, localSettings); err != nil {
		return nil, err
	}
	return localSettings, nil
}
