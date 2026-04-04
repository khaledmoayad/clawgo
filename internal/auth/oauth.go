package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/securestorage"
)

// OAuth endpoint constants matching the TypeScript services/oauth/.
const (
	AuthBaseURL        = "https://console.anthropic.com"
	TokenEndpoint      = "/v1/oauth/token"
	AuthorizeEndpoint  = "/oauth/authorize"
	ClientID           = "claude-code"
)

// OAuthTokens holds the token set returned by the OAuth token endpoint.
type OAuthTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	IDToken      string    `json:"id_token,omitempty"`
}

// OAuthService orchestrates the OAuth PKCE flow.
type OAuthService struct {
	storage securestorage.SecureStorage
}

// NewOAuthService creates an OAuthService backed by the given SecureStorage.
func NewOAuthService(storage securestorage.SecureStorage) *OAuthService {
	return &OAuthService{storage: storage}
}

// StartOAuthFlow performs the full OAuth PKCE authorization code flow:
// 1. Generate PKCE verifier/challenge and state
// 2. Start localhost callback server
// 3. Open browser to authorization URL
// 4. Wait for callback with auth code
// 5. Exchange code for tokens
// 6. Save tokens
func (s *OAuthService) StartOAuthFlow(ctx context.Context) (*OAuthTokens, error) {
	// Step 1: Generate PKCE parameters
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		return nil, fmt.Errorf("generate code verifier: %w", err)
	}
	challenge := GenerateCodeChallenge(verifier)

	state, err := GenerateState()
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	// Step 2: Start callback listener
	listener := NewAuthCodeListener()
	port, err := listener.Start()
	if err != nil {
		return nil, fmt.Errorf("start callback listener: %w", err)
	}
	defer listener.Close()

	// Step 3: Build authorization URL and open browser
	redirectURI := fmt.Sprintf("http://localhost:%d/oauth/callback", port)
	authURL := buildAuthURL(redirectURI, challenge, state)

	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	// Step 4: Wait for authorization code
	code, err := listener.WaitForCode(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("wait for auth code: %w", err)
	}

	// Step 5: Exchange code for tokens
	tokens, err := s.ExchangeCode(code, verifier, port)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	// Step 6: Save tokens
	tm := NewTokenManager(s.storage)
	if err := tm.SaveTokens(tokens); err != nil {
		return nil, fmt.Errorf("save tokens: %w", err)
	}

	return tokens, nil
}

// ExchangeCode exchanges an authorization code for tokens via HTTP POST
// to the token endpoint with PKCE code_verifier.
func (s *OAuthService) ExchangeCode(code, verifier string, port int) (*OAuthTokens, error) {
	redirectURI := fmt.Sprintf("http://localhost:%d/oauth/callback", port)

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {ClientID},
		"code_verifier": {verifier},
	}

	resp, err := http.Post(
		AuthBaseURL+TokenEndpoint,
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the token response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		IDToken      string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("parse token response: %w", err)
	}

	return &OAuthTokens{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
		IDToken:      tokenResp.IDToken,
	}, nil
}

// buildAuthURL constructs the full OAuth authorization URL with PKCE parameters.
func buildAuthURL(redirectURI, challenge, state string) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {ClientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"scope":                 {"user:inference"},
	}
	return AuthBaseURL + AuthorizeEndpoint + "?" + params.Encode()
}

// openBrowser opens the given URL in the user's default browser.
// Uses platform-specific commands matching the TypeScript implementation.
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default: // linux and others
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
