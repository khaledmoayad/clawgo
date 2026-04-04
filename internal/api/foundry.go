package api

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// buildFoundryOptions returns SDK request options for Azure Foundry.
// It handles URL construction and authentication via API key or Azure Entra ID.
//
// Base URL: ANTHROPIC_FOUNDRY_BASE_URL or https://{ANTHROPIC_FOUNDRY_RESOURCE}.services.ai.azure.com/anthropic/
// Auth: ANTHROPIC_FOUNDRY_API_KEY (api-key header) or Azure DefaultAzureCredential (Bearer token)
func buildFoundryOptions(_ context.Context, cfg ProviderClientConfig) []option.RequestOption {
	var opts []option.RequestOption

	// Set base URL
	baseURL := getFoundryBaseURL()
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	// Authentication
	if apiKey := os.Getenv("ANTHROPIC_FOUNDRY_API_KEY"); apiKey != "" {
		// API key auth: set api-key header (Azure's auth header format)
		opts = append(opts, option.WithHeader("api-key", apiKey))
	} else if !isEnvTruthy("CLAUDE_CODE_SKIP_FOUNDRY_AUTH") {
		// Azure Entra ID auth via DefaultAzureCredential
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err == nil {
			opts = append(opts, option.WithMiddleware(azureTokenMiddleware(cred)))
		}
		// If credential creation fails and auth is not skipped, we continue
		// without auth -- the API call will fail with a 401, which is more
		// informative than a startup error.
	}

	return opts
}

// getFoundryBaseURL resolves the Azure Foundry base URL.
// Checks ANTHROPIC_FOUNDRY_BASE_URL first, then constructs from
// ANTHROPIC_FOUNDRY_RESOURCE.
func getFoundryBaseURL() string {
	if baseURL := os.Getenv("ANTHROPIC_FOUNDRY_BASE_URL"); baseURL != "" {
		return baseURL
	}

	if resource := os.Getenv("ANTHROPIC_FOUNDRY_RESOURCE"); resource != "" {
		return fmt.Sprintf("https://%s.services.ai.azure.com/anthropic/", resource)
	}

	return ""
}

// azureTokenMiddleware returns SDK middleware that acquires an Azure Entra ID
// token and sets the Authorization header on each request.
func azureTokenMiddleware(cred *azidentity.DefaultAzureCredential) option.Middleware {
	return func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		token, err := cred.GetToken(req.Context(), policy.TokenRequestOptions{
			Scopes: []string{"https://cognitiveservices.azure.com/.default"},
		})
		if err != nil {
			return nil, fmt.Errorf("azure token acquisition failed: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token.Token)
		return next(req)
	}
}
