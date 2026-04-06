package enterprise

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	policyEndpoint        = "/v1/organizations/policy-limits"
	policyPollingInterval = 30 * time.Minute
	policyFetchTimeout    = 5 * time.Second
)

// PolicyLimits represents organization-level restrictions fetched from the API.
// These limits are enforced at the tool/feature level to implement enterprise
// policy controls.
type PolicyLimits struct {
	DisabledTools    []string `json:"disabled_tools"`
	DisabledCommands []string `json:"disabled_commands"`
	MaxTurns         int      `json:"max_turns"`
	DisableWebSearch bool     `json:"disable_web_search"`
	DisableFileWrite bool     `json:"disable_file_write"`
	RequireSandbox   bool     `json:"require_sandbox"`
	CustomMessage    string   `json:"custom_message"`
}

// PolicyLimitsManager fetches and caches organization-level policy limits.
// It polls the API periodically with ETag-based caching and enforces
// tool/command restrictions.
//
// The getToken func() string pattern is shared with RemoteSettingsManager
// for OAuth auth headers. At app wiring time, pass the same token getter
// to both managers (ENT-01 integration point).
type PolicyLimitsManager struct {
	mu         sync.RWMutex
	limits     *PolicyLimits
	apiBaseURL string
	getToken   func() string
	etag       string
	httpClient *http.Client
}

// NewPolicyLimitsManager creates a new manager that fetches policy limits
// from the given API base URL. The getToken function should return a valid
// OAuth bearer token for the Authorization header.
func NewPolicyLimitsManager(apiBaseURL string, getToken func() string) *PolicyLimitsManager {
	return &PolicyLimitsManager{
		apiBaseURL: apiBaseURL,
		getToken:   getToken,
		httpClient: &http.Client{Timeout: policyFetchTimeout},
	}
}

// Start begins background periodic fetching of policy limits.
// The first fetch happens immediately. Does NOT block the caller.
func (m *PolicyLimitsManager) Start(ctx context.Context) {
	go func() {
		// Fetch immediately on start
		if err := m.Fetch(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "clawgo: initial policy limits fetch failed: %v\n", err)
		}

		ticker := time.NewTicker(policyPollingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := m.Fetch(ctx); err != nil {
					fmt.Fprintf(os.Stderr, "clawgo: policy limits fetch failed: %v\n", err)
				}
			}
		}
	}()
}

// Fetch retrieves policy limits from the API endpoint.
// Uses ETag-based caching: sends If-None-Match header, skips update on 304.
// On error, keeps the cached limits (graceful degradation).
func (m *PolicyLimitsManager) Fetch(ctx context.Context) error {
	fetchCtx, cancel := context.WithTimeout(ctx, policyFetchTimeout)
	defer cancel()

	url := m.apiBaseURL + policyEndpoint
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating policy limits request: %w", err)
	}

	// OAuth auth header (ENT-01: same pattern used for remote settings auth enhancement)
	if token := m.getToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	m.mu.RLock()
	etag := m.etag
	m.mu.RUnlock()

	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching policy limits: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		// ETag matched, limits unchanged
		return nil

	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading policy limits response: %w", err)
		}

		var limits PolicyLimits
		if err := json.Unmarshal(body, &limits); err != nil {
			return fmt.Errorf("parsing policy limits: %w", err)
		}

		m.mu.Lock()
		m.limits = &limits
		if newEtag := resp.Header.Get("ETag"); newEtag != "" {
			m.etag = newEtag
		}
		m.mu.Unlock()

		return nil

	default:
		return fmt.Errorf("policy limits API returned status %d", resp.StatusCode)
	}
}

// GetLimits returns the cached policy limits under a read lock.
// Returns an empty PolicyLimits if none have been fetched yet.
func (m *PolicyLimitsManager) GetLimits() *PolicyLimits {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.limits != nil {
		return m.limits
	}
	return &PolicyLimits{}
}

// IsToolAllowed checks whether the given tool name is allowed by policy.
// Returns true if the tool is not in the disabled list or if no limits
// have been fetched yet.
func (m *PolicyLimitsManager) IsToolAllowed(toolName string) bool {
	limits := m.GetLimits()
	for _, disabled := range limits.DisabledTools {
		if disabled == toolName {
			return false
		}
	}
	return true
}

// IsCommandAllowed checks whether the given command name is allowed by policy.
// Returns true if the command is not in the disabled list or if no limits
// have been fetched yet.
func (m *PolicyLimitsManager) IsCommandAllowed(cmdName string) bool {
	limits := m.GetLimits()
	for _, disabled := range limits.DisabledCommands {
		if disabled == cmdName {
			return false
		}
	}
	return true
}

// GetMaxTurns returns the organization-level maximum number of query loop turns,
// or 0 if unlimited. The query loop should call this to enforce org-level turn limits.
func (m *PolicyLimitsManager) GetMaxTurns() int {
	return m.GetLimits().MaxTurns
}

// GetCustomMessage returns the custom message to display when policy blocks something.
// Returns "" if no custom message is configured.
func (m *PolicyLimitsManager) GetCustomMessage() string {
	return m.GetLimits().CustomMessage
}

// ShouldRequireSandbox returns true if the organization policy requires sandbox mode.
func (m *PolicyLimitsManager) ShouldRequireSandbox() bool {
	return m.GetLimits().RequireSandbox
}

// IsWebSearchAllowed returns true if web search is allowed by policy.
// Returns true if no limits have been fetched (permissive default).
func (m *PolicyLimitsManager) IsWebSearchAllowed() bool {
	return !m.GetLimits().DisableWebSearch
}

// IsFileWriteAllowed returns true if file write operations are allowed by policy.
// Returns true if no limits have been fetched (permissive default).
func (m *PolicyLimitsManager) IsFileWriteAllowed() bool {
	return !m.GetLimits().DisableFileWrite
}
