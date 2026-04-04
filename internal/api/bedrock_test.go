package api

import (
	"context"
	"testing"
)

func TestBuildBedrockOptions(t *testing.T) {
	t.Run("returns non-nil options slice", func(t *testing.T) {
		// Set minimal AWS config to avoid credential panic
		t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
		t.Setenv("AWS_REGION", "us-west-2")

		cfg := ProviderClientConfig{}
		opts := buildBedrockOptions(context.Background(), cfg)
		if opts == nil {
			t.Error("buildBedrockOptions() returned nil, want non-nil options")
		}
		if len(opts) == 0 {
			t.Error("buildBedrockOptions() returned empty slice, want at least one option")
		}
	})

	t.Run("respects AWS_REGION env var", func(t *testing.T) {
		t.Setenv("AWS_ACCESS_KEY_ID", "test-key")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
		t.Setenv("AWS_REGION", "eu-west-1")

		cfg := ProviderClientConfig{}
		opts := buildBedrockOptions(context.Background(), cfg)
		if opts == nil {
			t.Fatal("buildBedrockOptions() returned nil")
		}
		// Options are opaque; we verify the function doesn't panic and returns options.
		// The SDK bedrock middleware internally uses the region from the loaded config.
		if len(opts) < 1 {
			t.Error("expected at least 1 option")
		}
	})

	t.Run("supports bearer token auth when AWS_BEARER_TOKEN_BEDROCK is set", func(t *testing.T) {
		t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "test-bearer-token")
		t.Setenv("AWS_REGION", "us-east-1")
		// No access key/secret needed when using bearer token

		cfg := ProviderClientConfig{}
		opts := buildBedrockOptions(context.Background(), cfg)
		if opts == nil {
			t.Error("buildBedrockOptions() with bearer token returned nil")
		}
		if len(opts) < 1 {
			t.Error("expected at least 1 option with bearer token")
		}
	})
}
