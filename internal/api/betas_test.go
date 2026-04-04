package api

import (
	"testing"
)

func TestBetaConstants_Defined(t *testing.T) {
	// Verify all 15 beta header constants are defined and non-empty
	betas := []struct {
		name  string
		value string
	}{
		{"BetaClaudeCode", BetaClaudeCode},
		{"BetaInterleavedThinking", BetaInterleavedThinking},
		{"BetaContext1M", BetaContext1M},
		{"BetaContextManagement", BetaContextManagement},
		{"BetaStructuredOutputs", BetaStructuredOutputs},
		{"BetaWebSearch", BetaWebSearch},
		{"BetaToolSearch1P", BetaToolSearch1P},
		{"BetaToolSearch3P", BetaToolSearch3P},
		{"BetaEffort", BetaEffort},
		{"BetaTaskBudgets", BetaTaskBudgets},
		{"BetaPromptCachingScope", BetaPromptCachingScope},
		{"BetaFastMode", BetaFastMode},
		{"BetaRedactThinking", BetaRedactThinking},
		{"BetaTokenEfficientTools", BetaTokenEfficientTools},
		{"BetaAdvisor", BetaAdvisor},
	}

	for _, b := range betas {
		if b.value == "" {
			t.Errorf("beta constant %s is empty", b.name)
		}
	}
}

func TestBetaConstants_CorrectValues(t *testing.T) {
	// Verify key beta strings match the TS constants exactly
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"ClaudeCode", BetaClaudeCode, "claude-code-20250219"},
		{"InterleavedThinking", BetaInterleavedThinking, "interleaved-thinking-2025-05-14"},
		{"Context1M", BetaContext1M, "context-1m-2025-08-07"},
		{"ContextManagement", BetaContextManagement, "context-management-2025-06-27"},
		{"StructuredOutputs", BetaStructuredOutputs, "structured-outputs-2025-12-15"},
		{"WebSearch", BetaWebSearch, "web-search-2025-03-05"},
		{"ToolSearch1P", BetaToolSearch1P, "advanced-tool-use-2025-11-20"},
		{"ToolSearch3P", BetaToolSearch3P, "tool-search-tool-2025-10-19"},
		{"Effort", BetaEffort, "effort-2025-11-24"},
		{"TaskBudgets", BetaTaskBudgets, "task-budgets-2026-03-13"},
		{"PromptCachingScope", BetaPromptCachingScope, "prompt-caching-scope-2026-01-05"},
		{"FastMode", BetaFastMode, "fast-mode-2026-02-01"},
		{"RedactThinking", BetaRedactThinking, "redact-thinking-2026-02-12"},
		{"TokenEfficientTools", BetaTokenEfficientTools, "token-efficient-tools-2026-03-28"},
		{"Advisor", BetaAdvisor, "advisor-tool-2026-03-01"},
	}

	for _, tt := range tests {
		if tt.got != tt.expected {
			t.Errorf("%s: got %q, want %q", tt.name, tt.got, tt.expected)
		}
	}
}

func TestGetMessagesBetas_Direct(t *testing.T) {
	betas := GetMessagesBetas(ProviderFirstParty)
	if len(betas) < 14 {
		t.Errorf("direct provider should have at least 14 betas, got %d", len(betas))
	}

	// Must contain key betas
	required := []string{
		BetaClaudeCode,
		BetaInterleavedThinking,
		BetaContext1M,
		BetaContextManagement,
		BetaStructuredOutputs,
		BetaWebSearch,
		BetaToolSearch1P,
		BetaEffort,
		BetaTaskBudgets,
		BetaPromptCachingScope,
		BetaFastMode,
		BetaRedactThinking,
		BetaTokenEfficientTools,
		BetaAdvisor,
	}

	betaSet := make(map[string]bool)
	for _, b := range betas {
		betaSet[b] = true
	}
	for _, r := range required {
		if !betaSet[r] {
			t.Errorf("direct betas missing %q", r)
		}
	}

	// Direct should use 1P tool search header, NOT 3P
	if betaSet[BetaToolSearch3P] {
		t.Error("direct betas should NOT include 3P tool search header")
	}
}

func TestGetMessagesBetas_Bedrock(t *testing.T) {
	betas := GetMessagesBetas(ProviderBedrock)

	// Bedrock should include only the subset that goes in extraBodyParams
	betaSet := make(map[string]bool)
	for _, b := range betas {
		betaSet[b] = true
	}

	// Bedrock uses 3P tool search header
	if !betaSet[BetaToolSearch3P] {
		t.Error("bedrock betas should include 3P tool search header")
	}

	// Should include interleaved thinking
	if !betaSet[BetaInterleavedThinking] {
		t.Error("bedrock betas should include interleaved thinking")
	}
}

func TestGetMessagesBetas_Vertex(t *testing.T) {
	betas := GetMessagesBetas(ProviderVertex)

	// Vertex uses 3P tool search header
	betaSet := make(map[string]bool)
	for _, b := range betas {
		betaSet[b] = true
	}

	if !betaSet[BetaToolSearch3P] {
		t.Error("vertex betas should include 3P tool search header")
	}
}

func TestGetMessagesBetas_Foundry(t *testing.T) {
	betas := GetMessagesBetas(ProviderFoundry)

	// Foundry should behave like direct (1P) since it's an Anthropic proxy
	betaSet := make(map[string]bool)
	for _, b := range betas {
		betaSet[b] = true
	}

	if !betaSet[BetaToolSearch1P] {
		t.Error("foundry betas should include 1P tool search header")
	}
}

func TestBedrockExtraParamsBetas(t *testing.T) {
	betas := BedrockExtraParamsBetas()
	if len(betas) == 0 {
		t.Error("BedrockExtraParamsBetas should return non-empty slice")
	}

	betaSet := make(map[string]bool)
	for _, b := range betas {
		betaSet[b] = true
	}

	// Should include the three bedrock extra params betas
	if !betaSet[BetaInterleavedThinking] {
		t.Error("missing interleaved thinking in bedrock extra params")
	}
	if !betaSet[BetaContext1M] {
		t.Error("missing context 1M in bedrock extra params")
	}
	if !betaSet[BetaToolSearch3P] {
		t.Error("missing 3P tool search in bedrock extra params")
	}
}
