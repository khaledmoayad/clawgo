package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ProviderType identifies which LLM provider to use.
type ProviderType string

const (
	// ProviderFirstParty is the direct Anthropic API.
	ProviderFirstParty ProviderType = "firstParty"
	// ProviderBedrock is AWS Bedrock.
	ProviderBedrock ProviderType = "bedrock"
	// ProviderVertex is GCP Vertex AI.
	ProviderVertex ProviderType = "vertex"
	// ProviderFoundry is Azure Foundry.
	ProviderFoundry ProviderType = "foundry"
)

// GetProvider determines the active provider from environment variables.
// Priority: Bedrock > Vertex > Foundry > FirstParty.
// Mirrors TS utils/model/providers.ts:getAPIProvider().
func GetProvider() ProviderType {
	if isEnvTruthy("CLAUDE_CODE_USE_BEDROCK") {
		return ProviderBedrock
	}
	if isEnvTruthy("CLAUDE_CODE_USE_VERTEX") {
		return ProviderVertex
	}
	if isEnvTruthy("CLAUDE_CODE_USE_FOUNDRY") {
		return ProviderFoundry
	}
	return ProviderFirstParty
}

// isEnvTruthy returns true if the environment variable is set to a truthy
// value: "1", "true", or "yes" (case-insensitive).
func isEnvTruthy(key string) bool {
	val := strings.ToLower(os.Getenv(key))
	return val == "1" || val == "true" || val == "yes"
}

// ProviderClientConfig holds configuration for creating a provider-aware client.
type ProviderClientConfig struct {
	APIKey    string
	BaseURL   string
	Model     string
	MaxTokens int64
}

// NewProviderClient creates a Client using the active provider determined from
// environment variables. It applies provider-specific SDK options, mTLS config,
// and proxy transport.
func NewProviderClient(ctx context.Context, cfg ProviderClientConfig) (*Client, error) {
	provider := GetProvider()

	var opts []option.RequestOption

	switch provider {
	case ProviderBedrock:
		bedrockOpts := buildBedrockOptions(ctx, cfg)
		opts = append(opts, bedrockOpts...)
	case ProviderVertex:
		vertexOpts := buildVertexOptions(ctx, cfg)
		opts = append(opts, vertexOpts...)
	case ProviderFoundry:
		foundryOpts := buildFoundryOptions(ctx, cfg)
		opts = append(opts, foundryOpts...)
	default:
		// First-party Anthropic API
		apiKey := cfg.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("no API key provided: set ANTHROPIC_API_KEY environment variable or pass an API key")
		}
		opts = append(opts, option.WithAPIKey(apiKey))

		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = os.Getenv("ANTHROPIC_BASE_URL")
			if baseURL == "" {
				baseURL = os.Getenv("CLAUDE_CODE_API_BASE_URL")
			}
		}
		if baseURL != "" {
			opts = append(opts, option.WithBaseURL(baseURL))
		}
	}

	// Build HTTP transport with proxy and optional mTLS
	transport := NewProxyTransport()
	if tlsCfg := getMTLSConfig(); tlsCfg != nil {
		transport.TLSClientConfig = tlsCfg
	}

	httpClient := &http.Client{Transport: transport}
	opts = append(opts, option.WithHTTPClient(httpClient))

	sdk := anthropic.NewClient(opts...)

	// Determine model
	model := cfg.Model
	if model == "" {
		model = DefaultModel
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = DefaultMaxTokens
	}

	fallback, _ := GetFallbackModel(model)

	return &Client{
		SDK:           sdk,
		Model:         model,
		MaxTokens:     maxTokens,
		FallbackModel: fallback,
	}, nil
}
