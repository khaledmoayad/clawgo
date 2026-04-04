package api

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go/bedrock"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// buildBedrockOptions returns SDK request options for AWS Bedrock.
// It uses the anthropic-sdk-go bedrock subpackage which handles SigV4 signing,
// model ID translation, and event stream deserialization.
//
// Authentication priority:
// 1. AWS_BEARER_TOKEN_BEDROCK env var (bearer token auth)
// 2. Default AWS config (SigV4 signing via aws-sdk-go-v2 credential chain)
//
// Region is determined by the AWS_REGION env var via the default config.
func buildBedrockOptions(ctx context.Context, cfg ProviderClientConfig) []option.RequestOption {
	// bedrock.WithLoadDefaultConfig loads AWS credentials and region from the
	// standard aws-sdk-go-v2 config chain (env vars, shared credentials file,
	// EC2 instance role, etc.). It also checks AWS_BEARER_TOKEN_BEDROCK for
	// bearer token auth.
	opts := []option.RequestOption{
		bedrock.WithLoadDefaultConfig(ctx),
	}

	// Override base URL if explicitly configured
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	return opts
}
