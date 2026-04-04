package api

import (
	"testing"
)

func TestGetVertexRegion(t *testing.T) {
	t.Run("returns default us-east5 when no env vars set", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "")

		got := getVertexRegion("")
		if got != "us-east5" {
			t.Errorf("getVertexRegion('') = %q, want %q", got, "us-east5")
		}
	})

	t.Run("uses CLOUD_ML_REGION env var", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "europe-west4")

		got := getVertexRegion("")
		if got != "europe-west4" {
			t.Errorf("getVertexRegion('') = %q, want %q", got, "europe-west4")
		}
	})

	t.Run("supports per-model region override via VERTEX_REGION_CLAUDE_SONNET", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "us-east5")
		t.Setenv("VERTEX_REGION_CLAUDE_SONNET_4", "us-central1")

		got := getVertexRegion("claude-sonnet-4-20250514")
		if got != "us-central1" {
			t.Errorf("getVertexRegion('claude-sonnet-4-20250514') = %q, want %q", got, "us-central1")
		}
	})

	t.Run("falls back to CLOUD_ML_REGION when no model override", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "asia-southeast1")

		got := getVertexRegion("claude-haiku-4-20250514")
		if got != "asia-southeast1" {
			t.Errorf("getVertexRegion('claude-haiku-4-20250514') = %q, want %q", got, "asia-southeast1")
		}
	})
}

func TestBuildVertexOptions(t *testing.T) {
	t.Run("uses CLOUD_ML_REGION default us-east5", func(t *testing.T) {
		t.Setenv("CLOUD_ML_REGION", "")
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "test-project")

		// buildVertexOptions calls vertex.WithGoogleAuth which panics without
		// valid GCP credentials. We test the region/project resolution logic
		// instead of the full options construction.
		region := getVertexRegion("")
		if region != "us-east5" {
			t.Errorf("expected default region us-east5, got %q", region)
		}
	})

	t.Run("reads ANTHROPIC_VERTEX_PROJECT_ID env var", func(t *testing.T) {
		t.Setenv("ANTHROPIC_VERTEX_PROJECT_ID", "my-gcp-project")

		got := getVertexProjectID()
		if got != "my-gcp-project" {
			t.Errorf("getVertexProjectID() = %q, want %q", got, "my-gcp-project")
		}
	})
}
