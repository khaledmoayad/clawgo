// Package auth implements OAuth PKCE authentication with token refresh and
// secure credential storage. Matches the TypeScript services/oauth/ layer.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// GenerateCodeVerifier creates a cryptographically random PKCE code_verifier
// per RFC 7636. Returns 32 random bytes, base64url-encoded (no padding).
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge computes the S256 code_challenge from a code_verifier.
// SHA256(verifier) -> base64url-encoded (no padding).
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GenerateState creates a cryptographically random OAuth state parameter
// to prevent CSRF attacks. Returns 32 random bytes, base64url-encoded.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
