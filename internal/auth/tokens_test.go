package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/securestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testStorage creates a PlaintextStorage in a temp directory for testing.
func testStorage(t *testing.T) securestorage.SecureStorage {
	t.Helper()
	dir := t.TempDir()
	return securestorage.NewPlaintextStorageWithDir(dir)
}

func TestIsExpired_NotExpired(t *testing.T) {
	storage := testStorage(t)
	tm := NewTokenManager(storage)

	tokens := &OAuthTokens{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(1 * time.Hour), // expires in 1 hour
	}

	assert.False(t, tm.IsExpired(tokens), "token expiring in 1 hour should not be expired")
}

func TestIsExpired_WithinBuffer(t *testing.T) {
	storage := testStorage(t)
	tm := NewTokenManager(storage)

	tokens := &OAuthTokens{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(2 * time.Minute), // expires in 2 minutes (within 5-min buffer)
	}

	assert.True(t, tm.IsExpired(tokens), "token expiring in 2 minutes should be considered expired (5-min buffer)")
}

func TestIsExpired_AlreadyExpired(t *testing.T) {
	storage := testStorage(t)
	tm := NewTokenManager(storage)

	tokens := &OAuthTokens{
		AccessToken:  "test-access",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(-10 * time.Minute), // expired 10 minutes ago
	}

	assert.True(t, tm.IsExpired(tokens), "already-expired token should be considered expired")
}

func TestSaveLoadTokens_Roundtrip(t *testing.T) {
	storage := testStorage(t)
	tm := NewTokenManager(storage)

	original := &OAuthTokens{
		AccessToken:  "access-abc123",
		RefreshToken: "refresh-def456",
		ExpiresAt:    time.Now().Add(1 * time.Hour).Truncate(time.Millisecond), // truncate for JSON precision
		IDToken:      "id-ghi789",
	}

	err := tm.SaveTokens(original)
	require.NoError(t, err)

	loaded, err := tm.LoadTokens()
	require.NoError(t, err)

	assert.Equal(t, original.AccessToken, loaded.AccessToken)
	assert.Equal(t, original.RefreshToken, loaded.RefreshToken)
	assert.Equal(t, original.IDToken, loaded.IDToken)
	// Compare time within a small delta to account for JSON serialization precision
	assert.WithinDuration(t, original.ExpiresAt, loaded.ExpiresAt, time.Millisecond)
}

func TestDeleteTokens(t *testing.T) {
	storage := testStorage(t)
	tm := NewTokenManager(storage)

	// Save tokens first
	err := tm.SaveTokens(&OAuthTokens{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-def",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	})
	require.NoError(t, err)

	// Verify they exist
	_, err = tm.LoadTokens()
	require.NoError(t, err)

	// Delete
	err = tm.DeleteTokens()
	require.NoError(t, err)

	// Verify gone
	_, err = tm.LoadTokens()
	assert.True(t, errors.Is(err, securestorage.ErrNotFound), "expected ErrNotFound after delete, got %v", err)
}

func TestLoadTokens_NoTokens(t *testing.T) {
	storage := testStorage(t)
	tm := NewTokenManager(storage)

	_, err := tm.LoadTokens()
	assert.True(t, errors.Is(err, securestorage.ErrNotFound), "expected ErrNotFound when no tokens stored, got %v", err)
}
