package enterprise

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	teamMemoryEndpoint     = "/v1/team/memory"
	teamMemoryFetchTimeout = 10 * time.Second
)

// TeamMemoryEntry represents a single team memory item shared across collaborators.
type TeamMemoryEntry struct {
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TeamMemorySyncManager handles synchronization of team memory entries
// keyed by git remote URL. Uses server-wins pull and delta push.
type TeamMemorySyncManager struct {
	apiBaseURL string
	getToken   func() string
	httpClient *http.Client
	repoKey    string
}

// NewTeamMemorySyncManager creates a team memory sync manager.
// The gitRemoteURL is hashed (SHA-256, truncated to 16 chars) to create
// a stable repo key for server-side partitioning.
func NewTeamMemorySyncManager(apiBaseURL string, getToken func() string, gitRemoteURL string) *TeamMemorySyncManager {
	return &TeamMemorySyncManager{
		apiBaseURL: apiBaseURL,
		getToken:   getToken,
		httpClient: &http.Client{Timeout: teamMemoryFetchTimeout},
		repoKey:    hashRepoKey(gitRemoteURL),
	}
}

// RepoKey returns the hashed repo key used for server-side partitioning.
func (m *TeamMemorySyncManager) RepoKey() string {
	return m.repoKey
}

// Pull fetches team memory entries from the server.
// Server-wins on conflict: the returned entries are authoritative.
func (m *TeamMemorySyncManager) Pull(ctx context.Context) ([]TeamMemoryEntry, error) {
	url := m.apiBaseURL + teamMemoryEndpoint + "?repo=" + m.repoKey
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating team memory pull request: %w", err)
	}

	if token := m.getToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pulling team memory: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("team memory pull failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading team memory response: %w", err)
	}

	var entries []TeamMemoryEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parsing team memory entries: %w", err)
	}

	return entries, nil
}

// pushPayload is the request body for pushing team memory entries.
type pushPayload struct {
	Repo    string            `json:"repo"`
	Entries []TeamMemoryEntry `json:"entries"`
}

// Push sends local team memory entries to the server.
// The server merges using updated_at timestamps (delta push).
func (m *TeamMemorySyncManager) Push(ctx context.Context, entries []TeamMemoryEntry) error {
	payload := pushPayload{
		Repo:    m.repoKey,
		Entries: entries,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling team memory push payload: %w", err)
	}

	url := m.apiBaseURL + teamMemoryEndpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating team memory push request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token := m.getToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pushing team memory: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("team memory push failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// Sync performs a full sync: pull first (server-wins), then push local entries
// that are newer than their server counterparts.
// Returns the merged list of entries.
func (m *TeamMemorySyncManager) Sync(ctx context.Context, localEntries []TeamMemoryEntry) ([]TeamMemoryEntry, error) {
	// Pull server entries (server-wins)
	serverEntries, err := m.Pull(ctx)
	if err != nil {
		return nil, fmt.Errorf("sync pull failed: %w", err)
	}

	// Build server index by key for quick lookup
	serverIndex := make(map[string]TeamMemoryEntry, len(serverEntries))
	for _, e := range serverEntries {
		serverIndex[e.Key] = e
	}

	// Find local entries that are newer than server versions (delta)
	var toPush []TeamMemoryEntry
	for _, local := range localEntries {
		if server, exists := serverIndex[local.Key]; exists {
			if local.UpdatedAt.After(server.UpdatedAt) {
				toPush = append(toPush, local)
			}
		} else {
			// New entry not on server
			toPush = append(toPush, local)
		}
	}

	// Push newer local entries
	if len(toPush) > 0 {
		if err := m.Push(ctx, toPush); err != nil {
			return nil, fmt.Errorf("sync push failed: %w", err)
		}
	}

	// Merge: server entries take precedence, add local-only newer entries
	merged := make([]TeamMemoryEntry, 0, len(serverEntries)+len(toPush))
	merged = append(merged, serverEntries...)

	// Add local entries that were not on the server
	for _, local := range toPush {
		if _, exists := serverIndex[local.Key]; !exists {
			merged = append(merged, local)
		}
	}

	return merged, nil
}

// hashRepoKey produces a stable 16-character hex key from a git remote URL.
func hashRepoKey(gitRemoteURL string) string {
	h := sha256.Sum256([]byte(gitRemoteURL))
	return hex.EncodeToString(h[:])[:16]
}
