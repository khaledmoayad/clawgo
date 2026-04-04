package api

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateSelfSignedCert creates a temporary self-signed cert and key
// for testing. Returns the paths to the cert and key files.
func generateSelfSignedCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(1 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	// Write cert file
	dir := t.TempDir()
	certPath = filepath.Join(dir, "client.crt")
	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("failed to create cert file: %v", err)
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("failed to encode cert: %v", err)
	}
	certFile.Close()

	// Write key file
	keyPath = filepath.Join(dir, "client.key")
	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatalf("failed to marshal private key: %v", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatalf("failed to encode key: %v", err)
	}
	keyFile.Close()

	return certPath, keyPath
}

func TestGetMTLSConfig(t *testing.T) {
	t.Run("returns nil when env vars not set", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_CLIENT_CERT", "")
		t.Setenv("CLAUDE_CODE_CLIENT_KEY", "")

		cfg := getMTLSConfig()
		if cfg != nil {
			t.Error("getMTLSConfig() should return nil when env vars are not set")
		}
	})

	t.Run("returns nil when only cert is set", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_CLIENT_CERT", "/some/path/cert.pem")
		t.Setenv("CLAUDE_CODE_CLIENT_KEY", "")

		cfg := getMTLSConfig()
		if cfg != nil {
			t.Error("getMTLSConfig() should return nil when only cert is set")
		}
	})

	t.Run("returns nil when only key is set", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_CLIENT_CERT", "")
		t.Setenv("CLAUDE_CODE_CLIENT_KEY", "/some/path/key.pem")

		cfg := getMTLSConfig()
		if cfg != nil {
			t.Error("getMTLSConfig() should return nil when only key is set")
		}
	})

	t.Run("returns nil when cert file doesn't exist", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_CLIENT_CERT", "/nonexistent/cert.pem")
		t.Setenv("CLAUDE_CODE_CLIENT_KEY", "/nonexistent/key.pem")

		cfg := getMTLSConfig()
		if cfg != nil {
			t.Error("getMTLSConfig() should return nil when cert file doesn't exist")
		}
	})

	t.Run("returns tls.Config with loaded certificate when valid", func(t *testing.T) {
		certPath, keyPath := generateSelfSignedCert(t)

		t.Setenv("CLAUDE_CODE_CLIENT_CERT", certPath)
		t.Setenv("CLAUDE_CODE_CLIENT_KEY", keyPath)

		cfg := getMTLSConfig()
		if cfg == nil {
			t.Fatal("getMTLSConfig() should return non-nil config for valid cert/key")
		}
		if len(cfg.Certificates) != 1 {
			t.Errorf("expected 1 certificate, got %d", len(cfg.Certificates))
		}
	})
}
