package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/khaledmoayad/clawgo/internal/securestorage"
)

// ClaudeAIOAuthStorageKey is the secure storage key for Claude.ai OAuth credentials.
// This matches the key used by Claude Code's secure storage for the claudeAiOauth
// credential format.
const ClaudeAIOAuthStorageKey = "claudeAiOauth"

// ClaudeAICredentials represents OAuth credentials from a Claude.ai subscription.
// These are distinct from API keys -- they come from the Claude.ai subscriber
// authentication flow (OAuth PKCE).
type ClaudeAICredentials struct {
	// AccessToken is the OAuth bearer token for API authentication.
	AccessToken string `json:"accessToken"`

	// RefreshToken is used to obtain a new access token when the current one expires.
	RefreshToken string `json:"refreshToken"`

	// ExpiresAt is when the access token expires.
	ExpiresAt time.Time `json:"expiresAt"`

	// AccountUUID is the Claude.ai account identifier (optional).
	AccountUUID string `json:"accountUUID,omitempty"`

	// Scopes lists the OAuth scopes granted (e.g., "user:inference").
	Scopes []string `json:"scopes,omitempty"`

	// SubscriptionType is the user's Claude.ai subscription tier (e.g., "pro", "max").
	SubscriptionType string `json:"subscriptionType,omitempty"`

	// RateLimitTier is the rate limit tier for this account.
	RateLimitTier string `json:"rateLimitTier,omitempty"`
}

// ParseClaudeAIOAuthTokens reads OAuth tokens from secure storage in the
// claudeAiOauth format used by Claude Code. The JSON format matches:
//
//	{
//	  "accessToken": "...",
//	  "refreshToken": "...",
//	  "expiresAt": "2026-04-05T12:00:00Z",
//	  "accountUUID": "...",
//	  "scopes": ["user:inference"],
//	  "subscriptionType": "pro",
//	  "rateLimitTier": "tier_2"
//	}
//
// Returns the credentials or an error if not found, unreadable, or the
// access token is empty.
func ParseClaudeAIOAuthTokens(storage securestorage.SecureStorage) (*ClaudeAICredentials, error) {
	data, err := storage.Get(ClaudeAIOAuthStorageKey)
	if err != nil {
		return nil, fmt.Errorf("read claudeAiOauth credentials: %w", err)
	}

	var creds ClaudeAICredentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return nil, fmt.Errorf("parse claudeAiOauth credentials: %w", err)
	}

	if creds.AccessToken == "" {
		return nil, fmt.Errorf("claudeAiOauth credentials: access token is empty")
	}

	return &creds, nil
}

// IsClaudeAISubscriber checks if the user has valid Claude.ai OAuth tokens
// in secure storage. Returns true if credentials exist with a non-empty
// access token. Note: expired tokens still return true because the refresh
// mechanism handles renewal.
func IsClaudeAISubscriber(storage securestorage.SecureStorage) bool {
	creds, err := ParseClaudeAIOAuthTokens(storage)
	if err != nil {
		return false
	}
	return creds.AccessToken != ""
}
