package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// XAAConfig holds the configuration for XAA enterprise SSO (OIDC).
type XAAConfig struct {
	IssuerURL   string   // OIDC issuer URL (e.g., "https://idp.example.com")
	ClientID    string   // OAuth client ID
	RedirectURI string   // Redirect URI for the authorization callback
	Scopes      []string // OAuth scopes to request
}

// XAALogin performs an OIDC authorization_code + PKCE flow to obtain an
// access token for MCP server authentication.
//
// It opens a local HTTP server to receive the redirect callback, prints
// the authorization URL to stderr for the user to open, waits for the
// callback with the authorization code, and exchanges it for an access token.
func XAALogin(ctx context.Context, cfg XAAConfig) (string, error) {
	verifier := generateCodeVerifier()
	challenge := computeCodeChallenge(verifier)
	state := generateState()

	// Determine redirect URI -- use a local server if not specified
	redirectURI := cfg.RedirectURI
	var listener net.Listener
	var err error

	if redirectURI == "" {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", fmt.Errorf("failed to start local callback server: %w", err)
		}
		defer listener.Close()
		port := listener.Addr().(*net.TCPAddr).Port
		redirectURI = fmt.Sprintf("http://localhost:%d/callback", port)
	}

	authURL := buildAuthURL(XAAConfig{
		IssuerURL:   cfg.IssuerURL,
		ClientID:    cfg.ClientID,
		RedirectURI: redirectURI,
		Scopes:      cfg.Scopes,
	}, challenge, state)

	// Print auth URL to stderr (not stdout to avoid protocol corruption)
	fmt.Fprintf(os.Stderr, "Open the following URL in your browser to authenticate:\n%s\n", authURL)

	if listener == nil {
		return "", fmt.Errorf("no listener configured; specify RedirectURI or use local callback server")
	}

	// Wait for the callback
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		callbackState := r.URL.Query().Get("state")
		if callbackState != state {
			errCh <- fmt.Errorf("state mismatch: expected %q, got %q", state, callbackState)
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code received")
			http.Error(w, "Missing code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h1>Authentication successful</h1><p>You can close this window.</p></body></html>")
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}

	srv.Shutdown(ctx)

	// Exchange the code for a token
	tokenURL := cfg.IssuerURL + "/token"
	return exchangeToken(ctx, tokenURL, code, verifier, cfg.ClientID, redirectURI)
}

// buildAuthURL constructs the OIDC authorization URL with PKCE parameters.
func buildAuthURL(cfg XAAConfig, codeChallenge, state string) string {
	params := url.Values{}
	params.Set("client_id", cfg.ClientID)
	params.Set("redirect_uri", cfg.RedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", strings.Join(cfg.Scopes, " "))
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("state", state)

	return cfg.IssuerURL + "/authorize?" + params.Encode()
}

// exchangeToken exchanges an authorization code for an access token via
// the OIDC token endpoint.
func exchangeToken(ctx context.Context, tokenURL, code, codeVerifier, clientID, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return tokenResp.AccessToken, nil
}

// generateCodeVerifier generates a PKCE code verifier (43-128 chars).
// Uses crypto/rand for secure random bytes, base64url-encoded without padding.
func generateCodeVerifier() string {
	// 32 bytes -> 43 base64url characters (no padding)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// computeCodeChallenge computes the PKCE code challenge from a verifier.
// Uses SHA-256 hash, base64url-encoded without padding (S256 method).
func computeCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// generateState generates a random state parameter for CSRF protection.
func generateState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
