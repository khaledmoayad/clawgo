package api

// Beta header constants matching Claude Code's constants/betas.ts.
// These are sent as "anthropic-beta" headers on messages API requests
// to activate various features.
const (
	BetaClaudeCode          = "claude-code-20250219"
	BetaInterleavedThinking = "interleaved-thinking-2025-05-14"
	BetaContext1M           = "context-1m-2025-08-07"
	BetaContextManagement   = "context-management-2025-06-27"
	BetaStructuredOutputs   = "structured-outputs-2025-12-15"
	BetaWebSearch           = "web-search-2025-03-05"
	// Tool search beta headers differ by provider:
	// - Claude API / Foundry: advanced-tool-use-2025-11-20
	// - Vertex AI / Bedrock: tool-search-tool-2025-10-19
	BetaToolSearch1P        = "advanced-tool-use-2025-11-20"
	BetaToolSearch3P        = "tool-search-tool-2025-10-19"
	BetaEffort              = "effort-2025-11-24"
	BetaTaskBudgets         = "task-budgets-2026-03-13"
	BetaPromptCachingScope  = "prompt-caching-scope-2026-01-05"
	BetaFastMode            = "fast-mode-2026-02-01"
	BetaRedactThinking      = "redact-thinking-2026-02-12"
	BetaTokenEfficientTools = "token-efficient-tools-2026-03-28"
	BetaAdvisor             = "advisor-tool-2026-03-01"
)

// GetMessagesBetas returns the set of beta header strings to send on
// messages API requests for the given provider. Mirrors the TS betas.ts
// and claude.ts getMergedBetas / getModelBetas logic.
//
// For direct (1P) and Foundry: includes all betas with 1P tool search header.
// For Bedrock: all betas but uses 3P tool search header (Bedrock routes through 3P).
// For Vertex: all betas but uses 3P tool search header.
func GetMessagesBetas(provider ProviderType) []string {
	// Core betas shared across all providers
	betas := []string{
		BetaInterleavedThinking,
		BetaStructuredOutputs,
		BetaWebSearch,
		BetaEffort,
		BetaTokenEfficientTools,
	}

	switch provider {
	case ProviderBedrock, ProviderVertex:
		// 3P providers use the 3P tool search header
		betas = append(betas, BetaToolSearch3P)
	default:
		// 1P (direct) and Foundry use the 1P tool search header
		betas = append(betas, BetaToolSearch1P)
	}

	return betas
}

// BedrockExtraParamsBetas returns the subset of beta headers that Bedrock
// requires in extraBodyParams rather than HTTP headers. Mirrors the TS
// BEDROCK_EXTRA_PARAMS_HEADERS set.
func BedrockExtraParamsBetas() []string {
	return []string{
		BetaInterleavedThinking,
		BetaContext1M,
		BetaToolSearch3P,
	}
}
