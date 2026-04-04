package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	remoteSettingsFile     = "remote-settings.json"
	remoteFetchInterval    = 60 * time.Minute
	remoteFetchTimeout     = 5 * time.Second
	remoteSettingsEndpoint = "/v1/settings/remote"
)

// RemoteSettingsManager fetches and caches remote-managed settings.
// Remote settings are the highest-priority tier in the settings cascade.
type RemoteSettingsManager struct {
	configDir      string
	apiBaseURL     string
	cachedSettings *Settings
	mu             sync.RWMutex
	etag           string
	httpClient     *http.Client
}

// NewRemoteSettingsManager creates a manager and loads any cached settings from disk.
func NewRemoteSettingsManager(configDir, apiBaseURL string) *RemoteSettingsManager {
	m := &RemoteSettingsManager{
		configDir:  configDir,
		apiBaseURL: apiBaseURL,
		httpClient: &http.Client{Timeout: remoteFetchTimeout},
	}
	if s := m.loadFromDisk(); s != nil {
		m.cachedSettings = s
	}
	return m
}

// Start begins background periodic fetching of remote settings.
// The first fetch happens immediately in the goroutine.
// Does NOT block the caller -- fetching runs in a background goroutine.
func (m *RemoteSettingsManager) Start(ctx context.Context) {
	go func() {
		// Fetch immediately on start
		if _, err := m.FetchRemoteSettings(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "clawgo: initial remote settings fetch failed: %v\n", err)
		}

		ticker := time.NewTicker(remoteFetchInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := m.FetchRemoteSettings(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "clawgo: remote settings fetch failed: %v\n", err)
				}
			}
		}
	}()
}

// FetchRemoteSettings fetches settings from the remote API endpoint.
// Uses ETag-based caching: sends If-None-Match header, returns cached on 304.
// On error or timeout, returns cached settings from disk.
func (m *RemoteSettingsManager) FetchRemoteSettings(ctx context.Context) (*Settings, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, remoteFetchTimeout)
	defer cancel()

	url := m.apiBaseURL + remoteSettingsEndpoint
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, url, nil)
	if err != nil {
		return m.GetSettings(), fmt.Errorf("creating remote settings request: %w", err)
	}

	m.mu.RLock()
	etag := m.etag
	m.mu.RUnlock()

	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		// Network error -- return cached
		return m.GetSettings(), fmt.Errorf("fetching remote settings: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		// ETag matched, settings unchanged
		return m.GetSettings(), nil

	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return m.GetSettings(), fmt.Errorf("reading remote settings response: %w", err)
		}

		var s Settings
		if err := json.Unmarshal(body, &s); err != nil {
			return m.GetSettings(), fmt.Errorf("parsing remote settings: %w", err)
		}

		m.mu.Lock()
		m.cachedSettings = &s
		if newEtag := resp.Header.Get("ETag"); newEtag != "" {
			m.etag = newEtag
		}
		m.mu.Unlock()

		// Persist to disk (best-effort)
		if err := m.saveToDisk(&s); err != nil {
			fmt.Fprintf(os.Stderr, "clawgo: failed to persist remote settings: %v\n", err)
		}

		return &s, nil

	default:
		return m.GetSettings(), fmt.Errorf("remote settings API returned status %d", resp.StatusCode)
	}
}

// GetSettings returns the cached remote settings under a read lock.
// Returns an empty Settings if none have been loaded yet.
func (m *RemoteSettingsManager) GetSettings() *Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cachedSettings != nil {
		return m.cachedSettings
	}
	return &Settings{}
}

// loadFromDisk reads cached remote settings from configDir/remote-settings.json.
func (m *RemoteSettingsManager) loadFromDisk() *Settings {
	path := filepath.Join(m.configDir, remoteSettingsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}

// saveToDisk writes remote settings to configDir/remote-settings.json.
func (m *RemoteSettingsManager) saveToDisk(s *Settings) error {
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(m.configDir, remoteSettingsFile)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
