package auth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/securestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClaudeAIOAuthTokens_ValidTokens(t *testing.T) {
	storage := testStorage(t)

	// Store tokens in the claudeAiOauth format matching Claude Code's secure storage
	oauthData := map[string]interface{}{
		"accessToken":  "test-access-token",
		"refreshToken": "test-refresh-token",
		"expiresAt":    time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		"scopes":       []string{"user:inference"},
	}
	data, err := json.Marshal(oauthData)
	require.NoError(t, err)
	require.NoError(t, storage.Set(ClaudeAIOAuthStorageKey, string(data)))

	creds, err := ParseClaudeAIOAuthTokens(storage)
	require.NoError(t, err)
	assert.Equal(t, "test-access-token", creds.AccessToken)
	assert.Equal(t, "test-refresh-token", creds.RefreshToken)
	assert.NotZero(t, creds.ExpiresAt)
}

func TestParseClaudeAIOAuthTokens_NoTokens(t *testing.T) {
	storage := testStorage(t)

	_, err := ParseClaudeAIOAuthTokens(storage)
	assert.Error(t, err, "expected error when no tokens stored")
}

func TestParseClaudeAIOAuthTokens_EmptyAccessToken(t *testing.T) {
	storage := testStorage(t)

	oauthData := map[string]interface{}{
		"accessToken":  "",
		"refreshToken": "test-refresh",
		"expiresAt":    time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	}
	data, err := json.Marshal(oauthData)
	require.NoError(t, err)
	require.NoError(t, storage.Set(ClaudeAIOAuthStorageKey, string(data)))

	_, err = ParseClaudeAIOAuthTokens(storage)
	assert.Error(t, err, "expected error when access token is empty")
}

func TestParseClaudeAIOAuthTokens_WithAccountUUID(t *testing.T) {
	storage := testStorage(t)

	oauthData := map[string]interface{}{
		"accessToken":      "test-access-token",
		"refreshToken":     "test-refresh-token",
		"expiresAt":        time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		"accountUUID":      "uuid-123",
		"subscriptionType": "pro",
		"rateLimitTier":    "tier_2",
	}
	data, err := json.Marshal(oauthData)
	require.NoError(t, err)
	require.NoError(t, storage.Set(ClaudeAIOAuthStorageKey, string(data)))

	creds, err := ParseClaudeAIOAuthTokens(storage)
	require.NoError(t, err)
	assert.Equal(t, "test-access-token", creds.AccessToken)
	assert.Equal(t, "uuid-123", creds.AccountUUID)
	assert.Equal(t, "pro", creds.SubscriptionType)
	assert.Equal(t, "tier_2", creds.RateLimitTier)
}

func TestIsClaudeAISubscriber_WithValidTokens(t *testing.T) {
	storage := testStorage(t)

	oauthData := map[string]interface{}{
		"accessToken":  "test-access-token",
		"refreshToken": "test-refresh-token",
		"expiresAt":    time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	}
	data, err := json.Marshal(oauthData)
	require.NoError(t, err)
	require.NoError(t, storage.Set(ClaudeAIOAuthStorageKey, string(data)))

	assert.True(t, IsClaudeAISubscriber(storage))
}

func TestIsClaudeAISubscriber_NoTokens(t *testing.T) {
	storage := testStorage(t)
	assert.False(t, IsClaudeAISubscriber(storage))
}

func TestIsClaudeAISubscriber_ExpiredTokens(t *testing.T) {
	storage := testStorage(t)

	oauthData := map[string]interface{}{
		"accessToken":  "test-access-token",
		"refreshToken": "test-refresh-token",
		"expiresAt":    time.Now().Add(-1 * time.Hour).Format(time.RFC3339), // expired
	}
	data, err := json.Marshal(oauthData)
	require.NoError(t, err)
	require.NoError(t, storage.Set(ClaudeAIOAuthStorageKey, string(data)))

	// Expired tokens still count as "subscriber" -- the refresh mechanism handles renewal
	assert.True(t, IsClaudeAISubscriber(storage))
}

// Helper to ensure testStorage is accessible from this file (defined in tokens_test.go)
func credTestStorage(t *testing.T) securestorage.SecureStorage {
	t.Helper()
	return testStorage(t)
}
