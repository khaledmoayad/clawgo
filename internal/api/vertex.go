package api

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/vertex"
)

const defaultVertexRegion = "us-east5"

// buildVertexOptions returns SDK request options for GCP Vertex AI.
// It uses the anthropic-sdk-go vertex subpackage which handles OAuth2
// authentication and endpoint URL construction.
//
// Region resolution: model-specific env var -> CLOUD_ML_REGION -> "us-east5"
// Project resolution: ANTHROPIC_VERTEX_PROJECT_ID (required)
func buildVertexOptions(ctx context.Context, cfg ProviderClientConfig) []option.RequestOption {
	region := getVertexRegion(cfg.Model)
	projectID := getVertexProjectID()

	opts := []option.RequestOption{
		vertex.WithGoogleAuth(ctx, region, projectID),
	}

	// Override base URL if explicitly configured
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	return opts
}

// getVertexRegion resolves the GCP region for Vertex AI requests.
// It checks model-specific env vars first (e.g., VERTEX_REGION_CLAUDE_SONNET_4),
// then CLOUD_ML_REGION, and finally defaults to "us-east5".
func getVertexRegion(model string) string {
	// Check model-specific region override
	if model != "" {
		envKey := modelToVertexRegionEnvKey(model)
		if envKey != "" {
			if region := os.Getenv(envKey); region != "" {
				return region
			}
		}
	}

	// Check general region env var
	if region := os.Getenv("CLOUD_ML_REGION"); region != "" {
		return region
	}

	return defaultVertexRegion
}

// getVertexProjectID returns the GCP project ID from ANTHROPIC_VERTEX_PROJECT_ID.
func getVertexProjectID() string {
	return os.Getenv("ANTHROPIC_VERTEX_PROJECT_ID")
}

// modelToVertexRegionEnvKey converts a model name to its corresponding
// Vertex region override env var key. For example:
// "claude-sonnet-4-20250514" -> "VERTEX_REGION_CLAUDE_SONNET_4"
//
// The pattern matches TS: extract model family and major version,
// uppercase and join with underscores.
func modelToVertexRegionEnvKey(model string) string {
	// Split on hyphens: "claude-sonnet-4-20250514"
	parts := strings.Split(model, "-")
	if len(parts) < 3 {
		return ""
	}

	// Build env key from model name parts, stopping at the date portion
	// (which is all digits and 8+ chars long)
	var keyParts []string
	for _, p := range parts {
		// Stop at date-like parts (e.g., "20250514")
		if len(p) >= 8 && isAllDigits(p) {
			break
		}
		keyParts = append(keyParts, strings.ToUpper(p))
	}

	if len(keyParts) == 0 {
		return ""
	}

	return fmt.Sprintf("VERTEX_REGION_%s", strings.Join(keyParts, "_"))
}

// isAllDigits returns true if the string is non-empty and all characters are digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
