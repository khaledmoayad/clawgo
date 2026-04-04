package api

import (
	"crypto/tls"
	"os"
)

// getMTLSConfig returns a TLS configuration with client certificates loaded
// from the CLAUDE_CODE_CLIENT_CERT and CLAUDE_CODE_CLIENT_KEY environment
// variables. Returns nil if the env vars are not set or the cert/key files
// cannot be loaded.
func getMTLSConfig() *tls.Config {
	certFile := os.Getenv("CLAUDE_CODE_CLIENT_CERT")
	keyFile := os.Getenv("CLAUDE_CODE_CLIENT_KEY")

	if certFile == "" || keyFile == "" {
		return nil
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
}
