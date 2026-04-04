package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestXAABuildAuthURL(t *testing.T) {
	cfg := XAAConfig{
		IssuerURL:   "https://idp.example.com",
		ClientID:    "test-client",
		RedirectURI: "http://localhost:8080/callback",
		Scopes:      []string{"openid", "profile"},
	}

	verifier := generateCodeVerifier()
	challenge := computeCodeChallenge(verifier)

	authURL := buildAuthURL(cfg, challenge, "test-state")

	// Verify URL structure
	assert.Contains(t, authURL, "https://idp.example.com/authorize")
	assert.Contains(t, authURL, "client_id=test-client")
	assert.Contains(t, authURL, "redirect_uri=")
	assert.Contains(t, authURL, "response_type=code")
	assert.Contains(t, authURL, "scope=openid+profile")
	assert.Contains(t, authURL, "code_challenge="+challenge)
	assert.Contains(t, authURL, "code_challenge_method=S256")
	assert.Contains(t, authURL, "state=test-state")
}

func TestXAAExchangeToken(t *testing.T) {
	// Set up mock token endpoint
	var receivedBody map[string]string
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		err := r.ParseForm()
		require.NoError(t, err)

		receivedBody = make(map[string]string)
		for k, v := range r.Form {
			if len(v) > 0 {
				receivedBody[k] = v[0]
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "test-access-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "test-refresh-token",
		})
	}))
	defer tokenServer.Close()

	ctx := context.Background()
	token, err := exchangeToken(ctx, tokenServer.URL+"/token", "test-code", "test-verifier", "test-client", "http://localhost:8080/callback")
	require.NoError(t, err)
	assert.Equal(t, "test-access-token", token)

	// Verify the request body was correct
	assert.Equal(t, "authorization_code", receivedBody["grant_type"])
	assert.Equal(t, "test-code", receivedBody["code"])
	assert.Equal(t, "test-verifier", receivedBody["code_verifier"])
	assert.Equal(t, "test-client", receivedBody["client_id"])
	assert.Equal(t, "http://localhost:8080/callback", receivedBody["redirect_uri"])
}

func TestGenerateCodeVerifier(t *testing.T) {
	verifier := generateCodeVerifier()

	// Must be at least 43 characters (RFC 7636)
	assert.GreaterOrEqual(t, len(verifier), 43)
	// Must be at most 128 characters
	assert.LessOrEqual(t, len(verifier), 128)

	// Must only contain unreserved characters: [A-Z] / [a-z] / [0-9] / "-" / "." / "_" / "~"
	for _, c := range verifier {
		valid := (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~'
		assert.True(t, valid, "invalid character in code verifier: %c", c)
	}

	// Should be random (two calls produce different values)
	verifier2 := generateCodeVerifier()
	assert.NotEqual(t, verifier, verifier2, "code verifiers should be random")
}

func TestComputeCodeChallenge(t *testing.T) {
	// Known test vector from RFC 7636 Appendix B
	// verifier: "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	// expected challenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := computeCodeChallenge(verifier)
	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", challenge)
}

func TestComputeCodeChallengeNoPadding(t *testing.T) {
	// Verify that base64url encoding has no padding characters
	verifier := generateCodeVerifier()
	challenge := computeCodeChallenge(verifier)
	assert.NotContains(t, challenge, "=", "code challenge should not contain padding")
}
