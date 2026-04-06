package api

import (
	"encoding/json"
	"testing"
)

func TestCacheBreakDetector_FirstCall_NoBreak(t *testing.T) {
	d := NewCacheBreakDetector()
	params := CacheBreakParams{
		SystemBlocks: []string{"You are a helpful assistant."},
		Model:        "claude-sonnet-4-20250514",
		Betas:        []string{"beta1"},
		Effort:       "high",
	}

	report := d.DetectBreak(params)
	if report.HasBreak() {
		t.Error("first call should not detect a break (no previous state)")
	}
}

func TestCacheBreakDetector_NoChange_NoBreak(t *testing.T) {
	d := NewCacheBreakDetector()
	params := CacheBreakParams{
		SystemBlocks: []string{"You are a helpful assistant."},
		Model:        "claude-sonnet-4-20250514",
		Betas:        []string{"beta1"},
		Effort:       "high",
	}

	d.DetectBreak(params)
	report := d.DetectBreak(params)
	if report.HasBreak() {
		t.Error("identical params should not detect a break")
	}
}

func TestCacheBreakDetector_SystemChanged(t *testing.T) {
	d := NewCacheBreakDetector()
	params := CacheBreakParams{
		SystemBlocks: []string{"System prompt v1"},
		Model:        "claude-sonnet-4-20250514",
	}

	d.DetectBreak(params)

	params.SystemBlocks = []string{"System prompt v2"}
	report := d.DetectBreak(params)

	if !report.HasBreak() {
		t.Fatal("expected a break for system prompt change")
	}
	if !containsReason(report.Reasons, CacheBreakSystem) {
		t.Errorf("expected CacheBreakSystem in reasons, got %v", report.Reasons)
	}
}

func TestCacheBreakDetector_ToolChanged(t *testing.T) {
	d := NewCacheBreakDetector()
	toolsV1 := map[string]json.RawMessage{
		"bash":      json.RawMessage(`{"type":"object","properties":{"cmd":{"type":"string"}}}`),
		"file_read": json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	}
	toolsV2 := map[string]json.RawMessage{
		"bash":      json.RawMessage(`{"type":"object","properties":{"cmd":{"type":"string"},"timeout":{"type":"number"}}}`),
		"file_read": json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
	}

	d.DetectBreak(CacheBreakParams{
		SystemBlocks: []string{"sys"},
		Tools:        toolsV1,
		Model:        "claude-sonnet-4-20250514",
	})

	report := d.DetectBreak(CacheBreakParams{
		SystemBlocks: []string{"sys"},
		Tools:        toolsV2,
		Model:        "claude-sonnet-4-20250514",
	})

	if !report.HasBreak() {
		t.Fatal("expected a break for tool schema change")
	}
	if !containsReason(report.Reasons, CacheBreakTools) {
		t.Errorf("expected CacheBreakTools in reasons, got %v", report.Reasons)
	}
	if len(report.ChangedTools) != 1 || report.ChangedTools[0] != "bash" {
		t.Errorf("expected [bash] in ChangedTools, got %v", report.ChangedTools)
	}
}

func TestCacheBreakDetector_ToolAdded(t *testing.T) {
	d := NewCacheBreakDetector()
	d.DetectBreak(CacheBreakParams{
		Tools: map[string]json.RawMessage{
			"bash": json.RawMessage(`{}`),
		},
		Model: "claude-sonnet-4-20250514",
	})

	report := d.DetectBreak(CacheBreakParams{
		Tools: map[string]json.RawMessage{
			"bash":    json.RawMessage(`{}`),
			"new_tool": json.RawMessage(`{"new":true}`),
		},
		Model: "claude-sonnet-4-20250514",
	})

	if !report.HasBreak() {
		t.Fatal("expected a break for added tool")
	}
	if !containsReason(report.Reasons, CacheBreakTools) {
		t.Errorf("expected CacheBreakTools, got %v", report.Reasons)
	}
}

func TestCacheBreakDetector_ModelChanged(t *testing.T) {
	d := NewCacheBreakDetector()
	d.DetectBreak(CacheBreakParams{
		Model: "claude-sonnet-4-20250514",
	})

	report := d.DetectBreak(CacheBreakParams{
		Model: "claude-opus-4-20250514",
	})

	if !report.HasBreak() {
		t.Fatal("expected a break for model change")
	}
	if !containsReason(report.Reasons, CacheBreakModel) {
		t.Errorf("expected CacheBreakModel in reasons, got %v", report.Reasons)
	}
}

func TestCacheBreakDetector_BetaChanged(t *testing.T) {
	d := NewCacheBreakDetector()
	d.DetectBreak(CacheBreakParams{
		Model: "claude-sonnet-4-20250514",
		Betas: []string{"beta1"},
	})

	report := d.DetectBreak(CacheBreakParams{
		Model: "claude-sonnet-4-20250514",
		Betas: []string{"beta1", "beta2"},
	})

	if !report.HasBreak() {
		t.Fatal("expected a break for beta change")
	}
	if !containsReason(report.Reasons, CacheBreakBetas) {
		t.Errorf("expected CacheBreakBetas in reasons, got %v", report.Reasons)
	}
}

func TestCacheBreakDetector_EffortChanged(t *testing.T) {
	d := NewCacheBreakDetector()
	d.DetectBreak(CacheBreakParams{
		Model:  "claude-sonnet-4-20250514",
		Effort: "low",
	})

	report := d.DetectBreak(CacheBreakParams{
		Model:  "claude-sonnet-4-20250514",
		Effort: "high",
	})

	if !report.HasBreak() {
		t.Fatal("expected a break for effort change")
	}
	if !containsReason(report.Reasons, CacheBreakEffort) {
		t.Errorf("expected CacheBreakEffort in reasons, got %v", report.Reasons)
	}
}

func TestCacheBreakDetector_CacheControlChanged(t *testing.T) {
	d := NewCacheBreakDetector()
	d.DetectBreak(CacheBreakParams{
		Model:               "claude-sonnet-4-20250514",
		CacheControlEnabled: false,
	})

	report := d.DetectBreak(CacheBreakParams{
		Model:               "claude-sonnet-4-20250514",
		CacheControlEnabled: true,
	})

	if !report.HasBreak() {
		t.Fatal("expected a break for cache control change")
	}
	if !containsReason(report.Reasons, CacheBreakCacheControl) {
		t.Errorf("expected CacheBreakCacheControl in reasons, got %v", report.Reasons)
	}
}

func TestCacheBreakDetector_MultipleChanges(t *testing.T) {
	d := NewCacheBreakDetector()
	d.DetectBreak(CacheBreakParams{
		SystemBlocks: []string{"old system"},
		Model:        "claude-sonnet-4-20250514",
		Effort:       "low",
	})

	report := d.DetectBreak(CacheBreakParams{
		SystemBlocks: []string{"new system"},
		Model:        "claude-opus-4-20250514",
		Effort:       "high",
	})

	if !report.HasBreak() {
		t.Fatal("expected breaks for multiple changes")
	}
	if len(report.Reasons) != 3 {
		t.Errorf("expected 3 reasons, got %d: %v", len(report.Reasons), report.Reasons)
	}
}

func TestCacheBreakDetector_RecordCacheReadTokens(t *testing.T) {
	d := NewCacheBreakDetector()
	d.RecordCacheReadTokens(1234)
	if d.CacheReadTokens != 1234 {
		t.Errorf("expected 1234, got %d", d.CacheReadTokens)
	}
}

func TestDjb2Hash(t *testing.T) {
	// DJB2 should produce consistent results
	h1 := djb2Hash("hello")
	h2 := djb2Hash("hello")
	if h1 != h2 {
		t.Error("djb2Hash should be deterministic")
	}

	// Different inputs should produce different hashes
	h3 := djb2Hash("world")
	if h1 == h3 {
		t.Error("different inputs should produce different hashes")
	}

	// Empty string should produce a valid hash
	h4 := djb2Hash("")
	if h4 == "" {
		t.Error("empty string should produce a non-empty hash")
	}
}

func TestCacheBreakDetector_BetaOrder_DoesNotMatter(t *testing.T) {
	d := NewCacheBreakDetector()
	d.DetectBreak(CacheBreakParams{
		Model: "claude-sonnet-4-20250514",
		Betas: []string{"beta2", "beta1"},
	})

	report := d.DetectBreak(CacheBreakParams{
		Model: "claude-sonnet-4-20250514",
		Betas: []string{"beta1", "beta2"},
	})

	if report.HasBreak() {
		t.Error("beta order should not cause a cache break (sorted before comparison)")
	}
}

// containsReason checks if a reason is in the list.
func containsReason(reasons []CacheBreakReason, target CacheBreakReason) bool {
	for _, r := range reasons {
		if r == target {
			return true
		}
	}
	return false
}
