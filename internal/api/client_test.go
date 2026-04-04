package api

import (
	"os"
	"testing"
)

func TestNewClient_WithKey(t *testing.T) {
	client, err := NewClient("test-key-123", "")
	if err != nil {
		t.Fatalf("NewClient with key returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil client")
	}
	if client.Model != DefaultModel {
		t.Errorf("expected model %q, got %q", DefaultModel, client.Model)
	}
	if client.MaxTokens != DefaultMaxTokens {
		t.Errorf("expected max tokens %d, got %d", DefaultMaxTokens, client.MaxTokens)
	}
}

func TestNewClient_NoKey(t *testing.T) {
	// Ensure env var is not set
	orig := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if orig != "" {
			os.Setenv("ANTHROPIC_API_KEY", orig)
		}
	}()

	client, err := NewClient("", "")
	if err == nil {
		t.Fatal("NewClient with no key should return error")
	}
	if client != nil {
		t.Fatal("NewClient with no key should return nil client")
	}
}

func TestNewClient_WithEnvKey(t *testing.T) {
	orig := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "env-key-456")
	defer func() {
		if orig != "" {
			os.Setenv("ANTHROPIC_API_KEY", orig)
		} else {
			os.Unsetenv("ANTHROPIC_API_KEY")
		}
	}()

	client, err := NewClient("", "")
	if err != nil {
		t.Fatalf("NewClient with env key returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil client")
	}
}

func TestNewClient_BaseURL(t *testing.T) {
	client, err := NewClient("test-key", "https://custom.api.example.com")
	if err != nil {
		t.Fatalf("NewClient with base URL returned error: %v", err)
	}
	if client == nil {
		t.Fatal("NewClient returned nil client")
	}
}
