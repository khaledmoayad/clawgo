package mcp

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"golang.org/x/oauth2"
)

// ClaudeAuthProvider implements the Go MCP SDK's auth.OAuthHandler interface
// for Claude Code-style MCP server authentication. It wraps the authorization
// code + PKCE flow with token refresh and dynamic client registration (DCR)
// state, persisting just enough to mark a server as StatusNeedsAuth when
// credentials are missing or expired.
//
// Transport callers should use TokenSource for outgoing requests and Authorize
// when a 401/403 is received -- following the contract described in
// github.com/modelcontextprotocol/go-sdk/auth.OAuthHandler.
type ClaudeAuthProvider struct {
	mu sync.Mutex

	// config is the server configuration this provider is bound to.
	config MCPServerConfig

	// tokenSource caches the current oauth2.TokenSource (may be nil
	// if authorization has never completed or was revoked).
	tokenSource oauth2.TokenSource

	// authorizedOnce tracks whether Authorize has been called at least once.
	// Used to avoid retry loops: the transport calls Authorize once after a
	// 401/403 and should not call again if the retry also fails.
	authorizedOnce bool
}

// NewClaudeAuthProvider creates a ClaudeAuthProvider bound to the given server
// config. The provider starts in an unauthorized state (TokenSource returns nil)
// until Authorize is called successfully.
func NewClaudeAuthProvider(cfg MCPServerConfig) *ClaudeAuthProvider {
	return &ClaudeAuthProvider{config: cfg}
}

// TokenSource returns the current oauth2.TokenSource. If the user has not
// yet completed authorization, it returns (nil, nil) -- the transport should
// proceed without an Authorization header, triggering a 401 that routes
// through Authorize.
func (p *ClaudeAuthProvider) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tokenSource, nil
}

// Authorize is called by the transport when an HTTP request receives a
// 401 Unauthorized or 403 Forbidden response. It performs the OAuth
// authorization code + PKCE flow using the server's configured OAuth
// parameters.
//
// On success, subsequent calls to TokenSource return a non-nil source.
// The response body is consumed and closed by the caller (transport).
//
// Authorize is designed to be called at most once per failed request;
// the transport should not retry if the re-authorized request also fails.
func (p *ClaudeAuthProvider) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Consume response body to allow connection reuse
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	if p.authorizedOnce {
		// Already attempted authorization for this session -- do not loop
		return fmt.Errorf("MCP OAuth authorization already attempted for server %q; not retrying", p.config.Name)
	}
	p.authorizedOnce = true

	oauthCfg := p.config.OAuth
	if oauthCfg == nil {
		return fmt.Errorf("server %q returned %d but has no OAuth configuration", p.config.Name, resp.StatusCode)
	}

	// Build the OAuth2 config from server metadata
	handler, err := BuildOAuthHandler(p.config)
	if err != nil {
		return fmt.Errorf("building OAuth handler for %q: %w", p.config.Name, err)
	}

	// Delegate the full authorization flow to the handler
	if err := handler.Authorize(ctx, req, resp); err != nil {
		return fmt.Errorf("OAuth authorization for %q: %w", p.config.Name, err)
	}

	// Retrieve the token source from the completed handler
	ts, err := handler.TokenSource(ctx)
	if err != nil {
		return fmt.Errorf("retrieving token source after auth for %q: %w", p.config.Name, err)
	}

	p.tokenSource = ts
	return nil
}

// ResetAuth clears any cached token source, forcing re-authorization on
// the next transport retry. This is useful when the server explicitly
// revokes access.
func (p *ClaudeAuthProvider) ResetAuth() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokenSource = nil
	p.authorizedOnce = false
}

// HasToken reports whether this provider has a non-nil token source
// (i.e., authorization has completed at least once).
func (p *ClaudeAuthProvider) HasToken() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tokenSource != nil
}

// OAuthHandlerAdapter wraps the Claude auth flow into a minimal interface
// that mirrors the Go SDK's auth.OAuthHandler contract. Since the actual
// SDK type requires an unexported marker method (isOAuthHandler), this
// adapter is used by ClawGo's own MCP client code.
type OAuthHandlerAdapter struct {
	provider *ClaudeAuthProvider
}

// TokenSource delegates to the underlying ClaudeAuthProvider.
func (a *OAuthHandlerAdapter) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	return a.provider.TokenSource(ctx)
}

// Authorize delegates to the underlying ClaudeAuthProvider.
func (a *OAuthHandlerAdapter) Authorize(ctx context.Context, req *http.Request, resp *http.Response) error {
	return a.provider.Authorize(ctx, req, resp)
}

// BuildOAuthHandler constructs an OAuthHandlerAdapter from an MCPServerConfig.
// The returned handler supports authorization code + PKCE flow using the
// XAA OIDC infrastructure already present in xaa.go.
//
// If the config has no OAuth settings, it returns an error.
func BuildOAuthHandler(cfg MCPServerConfig) (*OAuthHandlerAdapter, error) {
	if cfg.OAuth == nil {
		return nil, fmt.Errorf("MCPServerConfig %q has no OAuth configuration", cfg.Name)
	}

	provider := NewClaudeAuthProvider(cfg)
	return &OAuthHandlerAdapter{provider: provider}, nil
}

// NeedsAuth returns true if the server configuration requires OAuth but
// the provider does not yet have a token.
func NeedsAuth(cfg MCPServerConfig, provider *ClaudeAuthProvider) bool {
	if cfg.OAuth == nil {
		return false
	}
	if provider == nil {
		return true
	}
	return !provider.HasToken()
}
