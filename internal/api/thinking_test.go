package api

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

func TestBuildThinkingParam_Adaptive(t *testing.T) {
	cfg := ThinkingConfig{
		Adaptive: true,
	}
	result := BuildThinkingParam(cfg)
	if result == nil {
		t.Fatal("expected non-nil ThinkingConfigParamUnion for adaptive")
	}
	if result.OfAdaptive == nil {
		t.Fatal("expected OfAdaptive to be set")
	}
	if param.IsOmitted(result.OfAdaptive) {
		t.Fatal("expected OfAdaptive to not be omitted")
	}
}

func TestBuildThinkingParam_Enabled(t *testing.T) {
	cfg := ThinkingConfig{
		Enabled:      true,
		BudgetTokens: 8000,
	}
	result := BuildThinkingParam(cfg)
	if result == nil {
		t.Fatal("expected non-nil ThinkingConfigParamUnion for enabled")
	}
	if result.OfEnabled == nil {
		t.Fatal("expected OfEnabled to be set")
	}
	if result.OfEnabled.BudgetTokens != 8000 {
		t.Errorf("expected BudgetTokens 8000, got %d", result.OfEnabled.BudgetTokens)
	}
}

func TestBuildThinkingParam_Disabled(t *testing.T) {
	cfg := ThinkingConfig{
		Disabled: true,
	}
	result := BuildThinkingParam(cfg)
	if result != nil {
		t.Error("expected nil for disabled thinking")
	}
}

func TestBuildThinkingParam_Default(t *testing.T) {
	// All fields zero -- should return nil (no thinking param)
	cfg := ThinkingConfig{}
	result := BuildThinkingParam(cfg)
	if result != nil {
		t.Error("expected nil for default (zero-value) thinking config")
	}
}

func TestEffortLevel_Values(t *testing.T) {
	if EffortHigh != "high" {
		t.Errorf("expected EffortHigh to be %q, got %q", "high", EffortHigh)
	}
	if EffortMedium != "medium" {
		t.Errorf("expected EffortMedium to be %q, got %q", "medium", EffortMedium)
	}
	if EffortLow != "low" {
		t.Errorf("expected EffortLow to be %q, got %q", "low", EffortLow)
	}
	if EffortMax != "max" {
		t.Errorf("expected EffortMax to be %q, got %q", "max", EffortMax)
	}
}

func TestEffortLevel_Empty(t *testing.T) {
	var e EffortLevel
	if e != "" {
		t.Errorf("expected zero-value EffortLevel to be empty, got %q", e)
	}
}
