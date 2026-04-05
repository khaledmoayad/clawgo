package mcp

import (
	"context"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeAuthProviderTokenSourceInitiallyNil(t *testing.T) {
	cfg := MCPServerConfig{
		Name: "test-server",
		OAuth: &MCPOAuthConfig{
			ClientID: "test-client",
		},
	}
	provider := NewClaudeAuthProvider(cfg)

	ts, err := provider.TokenSource(context.Background())
	require.NoError(t, err)
	assert.Nil(t, ts, "initial token source should be nil")
}

func TestClaudeAuthProviderHasToken(t *testing.T) {
	cfg := MCPServerConfig{Name: "test"}
	provider := NewClaudeAuthProvider(cfg)

	assert.False(t, provider.HasToken(), "should not have token initially")

	// After ResetAuth, still no token
	provider.ResetAuth()
	assert.False(t, provider.HasToken(), "should not have token after reset")
}

func TestClaudeAuthProviderResetAuth(t *testing.T) {
	cfg := MCPServerConfig{Name: "test"}
	provider := NewClaudeAuthProvider(cfg)

	// Simulate that authorizedOnce is set
	provider.mu.Lock()
	provider.authorizedOnce = true
	provider.mu.Unlock()

	provider.ResetAuth()

	provider.mu.Lock()
	assert.False(t, provider.authorizedOnce, "authorizedOnce should be reset")
	provider.mu.Unlock()
}

func TestNeedsAuth(t *testing.T) {
	tests := []struct {
		name     string
		cfg      MCPServerConfig
		provider *ClaudeAuthProvider
		want     bool
	}{
		{
			name:     "no oauth config",
			cfg:      MCPServerConfig{Name: "s"},
			provider: nil,
			want:     false,
		},
		{
			name:     "oauth config but nil provider",
			cfg:      MCPServerConfig{Name: "s", OAuth: &MCPOAuthConfig{ClientID: "c"}},
			provider: nil,
			want:     true,
		},
		{
			name:     "oauth config and provider without token",
			cfg:      MCPServerConfig{Name: "s", OAuth: &MCPOAuthConfig{ClientID: "c"}},
			provider: NewClaudeAuthProvider(MCPServerConfig{Name: "s"}),
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NeedsAuth(tt.cfg, tt.provider)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildOAuthHandlerRequiresOAuth(t *testing.T) {
	cfg := MCPServerConfig{Name: "no-oauth"}
	_, err := BuildOAuthHandler(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no OAuth configuration")
}

func TestBuildOAuthHandlerSuccess(t *testing.T) {
	cfg := MCPServerConfig{
		Name: "test-server",
		OAuth: &MCPOAuthConfig{
			ClientID: "test-client",
		},
	}
	handler, err := BuildOAuthHandler(cfg)
	require.NoError(t, err)
	assert.NotNil(t, handler)

	// Initial token source should be nil
	ts, err := handler.TokenSource(context.Background())
	require.NoError(t, err)
	assert.Nil(t, ts)
}

// ---- Policy tests ----

func TestEvaluateServerPolicyNilSettings(t *testing.T) {
	decision := EvaluateServerPolicy("any-server", nil)
	assert.Equal(t, PolicyAllowed, decision)
}

func TestEvaluateServerPolicyEmptySettings(t *testing.T) {
	settings := &config.Settings{}
	decision := EvaluateServerPolicy("any-server", settings)
	assert.Equal(t, PolicyAllowed, decision)
}

func TestEvaluateServerPolicyDenyListMatch(t *testing.T) {
	settings := &config.Settings{
		DeniedMCPServerNames: []string{"blocked-server", "another-blocked"},
	}
	assert.Equal(t, PolicyDenied, EvaluateServerPolicy("blocked-server", settings))
	assert.Equal(t, PolicyDenied, EvaluateServerPolicy("another-blocked", settings))
	assert.Equal(t, PolicyAllowed, EvaluateServerPolicy("ok-server", settings))
}

func TestEvaluateServerPolicyAllowListRestriction(t *testing.T) {
	settings := &config.Settings{
		AllowedMCPServerNames: []string{"allowed-a", "allowed-b"},
	}
	assert.Equal(t, PolicyAllowed, EvaluateServerPolicy("allowed-a", settings))
	assert.Equal(t, PolicyAllowed, EvaluateServerPolicy("allowed-b", settings))
	assert.Equal(t, PolicyDenied, EvaluateServerPolicy("not-in-list", settings))
}

func TestEvaluateServerPolicyDenyOverridesAllow(t *testing.T) {
	// A server on both lists should be denied (deny list checked first)
	settings := &config.Settings{
		AllowedMCPServerNames: []string{"server-a"},
		DeniedMCPServerNames:  []string{"server-a"},
	}
	assert.Equal(t, PolicyDenied, EvaluateServerPolicy("server-a", settings))
}

func TestEvaluateServerPolicyNoLists(t *testing.T) {
	settings := &config.Settings{
		Model: "claude-sonnet-4-20250514", // unrelated field
	}
	assert.Equal(t, PolicyAllowed, EvaluateServerPolicy("any-server", settings))
}
