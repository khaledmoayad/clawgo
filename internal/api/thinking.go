package api

import (
	"github.com/anthropics/anthropic-sdk-go"
)

// ThinkingConfig holds the configuration for Claude's extended thinking feature.
// Mirrors the TS ThinkingConfig type from utils/thinking.ts.
type ThinkingConfig struct {
	// Enabled indicates thinking should be active (type:"enabled" with budget).
	Enabled bool
	// Adaptive uses type:"adaptive" (newer models like Opus 4.6+, Sonnet 4.6+).
	Adaptive bool
	// BudgetTokens is the thinking token budget for type:"enabled" mode.
	// Must be less than max_tokens.
	BudgetTokens int64
	// Disabled explicitly disables thinking (CLAUDE_CODE_DISABLE_THINKING env var).
	Disabled bool
}

// BuildThinkingParam constructs a ThinkingConfigParamUnion from the config.
// Returns nil if thinking should be disabled or the config is a zero value.
//
// Logic:
//   - If Disabled or zero-value config: return nil (no thinking param sent)
//   - If Adaptive: return {Type: "adaptive"}
//   - If Enabled with BudgetTokens: return {Type: "enabled", BudgetTokens: N}
func BuildThinkingParam(cfg ThinkingConfig) *anthropic.ThinkingConfigParamUnion {
	if cfg.Disabled {
		return nil
	}

	if cfg.Adaptive {
		result := anthropic.ThinkingConfigParamUnion{
			OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{},
		}
		return &result
	}

	if cfg.Enabled && cfg.BudgetTokens > 0 {
		result := anthropic.ThinkingConfigParamOfEnabled(cfg.BudgetTokens)
		return &result
	}

	// No thinking configured
	return nil
}

// EffortLevel represents the effort/quality tradeoff for model responses.
// Maps to the OutputConfigEffort in the Anthropic API.
type EffortLevel string

const (
	EffortHigh   EffortLevel = "high"
	EffortMedium EffortLevel = "medium"
	EffortLow    EffortLevel = "low"
	EffortMax    EffortLevel = "max"
)

// ToOutputConfigEffort converts an EffortLevel to the SDK's OutputConfigEffort type.
func (e EffortLevel) ToOutputConfigEffort() anthropic.OutputConfigEffort {
	return anthropic.OutputConfigEffort(e)
}
