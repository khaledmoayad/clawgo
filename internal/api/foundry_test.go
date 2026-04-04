package api

import (
	"testing"
)

func TestGetFoundryBaseURL(t *testing.T) {
	t.Run("constructs URL from ANTHROPIC_FOUNDRY_RESOURCE", func(t *testing.T) {
		t.Setenv("ANTHROPIC_FOUNDRY_BASE_URL", "")
		t.Setenv("ANTHROPIC_FOUNDRY_RESOURCE", "my-resource")

		got := getFoundryBaseURL()
		want := "https://my-resource.services.ai.azure.com/anthropic/"
		if got != want {
			t.Errorf("getFoundryBaseURL() = %q, want %q", got, want)
		}
	})

	t.Run("uses ANTHROPIC_FOUNDRY_BASE_URL directly if set", func(t *testing.T) {
		t.Setenv("ANTHROPIC_FOUNDRY_BASE_URL", "https://custom.foundry.example.com/")
		t.Setenv("ANTHROPIC_FOUNDRY_RESOURCE", "my-resource")

		got := getFoundryBaseURL()
		want := "https://custom.foundry.example.com/"
		if got != want {
			t.Errorf("getFoundryBaseURL() = %q, want %q", got, want)
		}
	})

	t.Run("returns empty when neither env var is set", func(t *testing.T) {
		t.Setenv("ANTHROPIC_FOUNDRY_BASE_URL", "")
		t.Setenv("ANTHROPIC_FOUNDRY_RESOURCE", "")

		got := getFoundryBaseURL()
		if got != "" {
			t.Errorf("getFoundryBaseURL() = %q, want empty string", got)
		}
	})
}

func TestBuildFoundryOptions(t *testing.T) {
	t.Run("applies api-key header when ANTHROPIC_FOUNDRY_API_KEY is set", func(t *testing.T) {
		t.Setenv("ANTHROPIC_FOUNDRY_RESOURCE", "test-resource")
		t.Setenv("ANTHROPIC_FOUNDRY_API_KEY", "my-azure-key")
		t.Setenv("ANTHROPIC_FOUNDRY_BASE_URL", "")
		t.Setenv("CLAUDE_CODE_SKIP_FOUNDRY_AUTH", "")

		cfg := ProviderClientConfig{}
		opts := buildFoundryOptions(nil, cfg)
		if opts == nil {
			t.Fatal("buildFoundryOptions() returned nil")
		}
		// Should have at least base URL + api-key header options
		if len(opts) < 2 {
			t.Errorf("expected at least 2 options (base URL + api key), got %d", len(opts))
		}
	})

	t.Run("skips auth when CLAUDE_CODE_SKIP_FOUNDRY_AUTH is set", func(t *testing.T) {
		t.Setenv("ANTHROPIC_FOUNDRY_RESOURCE", "test-resource")
		t.Setenv("ANTHROPIC_FOUNDRY_API_KEY", "")
		t.Setenv("ANTHROPIC_FOUNDRY_BASE_URL", "")
		t.Setenv("CLAUDE_CODE_SKIP_FOUNDRY_AUTH", "1")

		cfg := ProviderClientConfig{}
		opts := buildFoundryOptions(nil, cfg)
		if opts == nil {
			t.Fatal("buildFoundryOptions() returned nil")
		}
		// Should have only base URL option (no auth)
		if len(opts) != 1 {
			t.Errorf("expected 1 option (base URL only), got %d", len(opts))
		}
	})

	t.Run("adds Azure credential middleware when no API key and auth not skipped", func(t *testing.T) {
		t.Setenv("ANTHROPIC_FOUNDRY_RESOURCE", "test-resource")
		t.Setenv("ANTHROPIC_FOUNDRY_API_KEY", "")
		t.Setenv("ANTHROPIC_FOUNDRY_BASE_URL", "")
		t.Setenv("CLAUDE_CODE_SKIP_FOUNDRY_AUTH", "")
		// Set Azure env vars so DefaultAzureCredential can construct (it won't
		// actually fetch tokens in tests, but should not panic during construction)
		t.Setenv("AZURE_TENANT_ID", "test-tenant")
		t.Setenv("AZURE_CLIENT_ID", "test-client")
		t.Setenv("AZURE_CLIENT_SECRET", "test-secret")

		cfg := ProviderClientConfig{}
		opts := buildFoundryOptions(nil, cfg)
		if opts == nil {
			t.Fatal("buildFoundryOptions() returned nil")
		}
		// Should have base URL + middleware option
		if len(opts) < 2 {
			t.Errorf("expected at least 2 options (base URL + middleware), got %d", len(opts))
		}
	})
}
