package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/config"
	"github.com/khaledmoayad/clawgo/internal/securestorage"
)

// Token management constants matching the TypeScript implementation.
const (
	// TokenStorageKey is the key used in SecureStorage for OAuth tokens.
	TokenStorageKey = "oauth_tokens"

	// TokenExpiryBuffer is how far before expiry we trigger a refresh.
	// Matches the TS 5-minute buffer.
	TokenExpiryBuffer = 5 * time.Minute

	// LockRetryDelay is the base delay between lock retry attempts.
	LockRetryDelay = time.Second

	// MaxLockRetries is the maximum number of lock acquisition retries.
	MaxLockRetries = 5
)

// TokenManager handles loading, saving, and refreshing OAuth tokens with
// file-based concurrency control.
type TokenManager struct {
	storage   securestorage.SecureStorage
	configDir string
}

// NewTokenManager creates a TokenManager backed by the given SecureStorage.
func NewTokenManager(storage securestorage.SecureStorage) *TokenManager {
	return &TokenManager{
		storage:   storage,
		configDir: config.ConfigDir(),
	}
}

// LoadTokens reads the stored OAuth tokens from SecureStorage.
// Returns securestorage.ErrNotFound if no tokens are stored.
func (t *TokenManager) LoadTokens() (*OAuthTokens, error) {
	data, err := t.storage.Get(TokenStorageKey)
	if err != nil {
		return nil, err
	}

	var tokens OAuthTokens
	if err := json.Unmarshal([]byte(data), &tokens); err != nil {
		return nil, fmt.Errorf("parse stored tokens: %w", err)
	}
	return &tokens, nil
}

// SaveTokens serializes and stores the OAuth tokens in SecureStorage.
func (t *TokenManager) SaveTokens(tokens *OAuthTokens) error {
	data, err := json.Marshal(tokens)
	if err != nil {
		return fmt.Errorf("serialize tokens: %w", err)
	}
	return t.storage.Set(TokenStorageKey, string(data))
}

// IsExpired returns true if the tokens are expired or will expire within
// the TokenExpiryBuffer window (5 minutes).
func (t *TokenManager) IsExpired(tokens *OAuthTokens) bool {
	return time.Now().After(tokens.ExpiresAt.Add(-TokenExpiryBuffer))
}

// RefreshIfNeeded loads the stored tokens and refreshes them if expired.
// Uses file-based locking with O_CREATE|O_EXCL to coordinate concurrent
// refresh attempts across processes (matching TS re-read-after-lock pattern).
func (t *TokenManager) RefreshIfNeeded(ctx context.Context) (*OAuthTokens, error) {
	// Load current tokens
	tokens, err := t.LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("load tokens: %w", err)
	}

	// If not expired, return immediately
	if !t.IsExpired(tokens) {
		return tokens, nil
	}

	// Acquire file lock for refresh
	lockPath := filepath.Join(t.configDir, ".token-refresh.lock")
	lockFile, err := t.acquireLock(ctx, lockPath)
	if err != nil {
		return nil, fmt.Errorf("acquire refresh lock: %w", err)
	}
	defer t.releaseLock(lockFile, lockPath)

	// Re-read tokens after acquiring lock (another process may have refreshed)
	tokens, err = t.LoadTokens()
	if err != nil {
		return nil, fmt.Errorf("reload tokens after lock: %w", err)
	}
	if !t.IsExpired(tokens) {
		return tokens, nil
	}

	// Perform the refresh
	newTokens, err := t.doRefresh(tokens.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("refresh tokens: %w", err)
	}

	// Save refreshed tokens
	if err := t.SaveTokens(newTokens); err != nil {
		return nil, fmt.Errorf("save refreshed tokens: %w", err)
	}

	return newTokens, nil
}

// DeleteTokens removes stored tokens from SecureStorage (used for logout).
func (t *TokenManager) DeleteTokens() error {
	return t.storage.Delete(TokenStorageKey)
}

// acquireLock attempts to create an exclusive lock file using O_CREATE|O_EXCL.
// Retries up to MaxLockRetries times with jittered delays.
func (t *TokenManager) acquireLock(ctx context.Context, lockPath string) (*os.File, error) {
	for i := 0; i <= MaxLockRetries; i++ {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			return f, nil
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("lock file: %w", err)
		}

		// Lock held by another process — wait with jitter
		if i == MaxLockRetries {
			return nil, fmt.Errorf("failed to acquire lock after %d retries", MaxLockRetries)
		}

		jitter := time.Duration(rand.Int63n(int64(time.Second)))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(LockRetryDelay + jitter):
			// retry
		}
	}
	// unreachable, but satisfy the compiler
	return nil, fmt.Errorf("lock acquisition failed")
}

// releaseLock closes and removes the lock file.
func (t *TokenManager) releaseLock(f *os.File, lockPath string) {
	if f != nil {
		f.Close()
	}
	os.Remove(lockPath)
}

// doRefresh sends a refresh token grant to the OAuth token endpoint.
func (t *TokenManager) doRefresh(refreshToken string) (*OAuthTokens, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {ClientID},
	}

	resp, err := http.Post(
		AuthBaseURL+TokenEndpoint,
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("refresh endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse refresh response: %w", err)
	}

	return &OAuthTokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		IDToken:      tokenResp.IDToken,
	}, nil
}
