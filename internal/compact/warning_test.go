package compact

import (
	"testing"
)

func TestWarning_At80Pct_FiresOnce(t *testing.T) {
	state := &CompactWarningState{}
	// defaultContextWindow=200000, minus MaxOutputTokensForSummary=20000 = 180000 effective
	// 80% of 180000 = 144000
	tokens := 144000

	warning, level := CheckCompactWarning(state, tokens, "claude-sonnet-4-20250514")
	if level != "warning" {
		t.Errorf("expected level=warning, got %q", level)
	}
	if warning == "" {
		t.Error("expected non-empty warning message")
	}
	if !state.WarningFired {
		t.Error("expected WarningFired=true")
	}

	// Second call at same level should not fire again
	warning2, level2 := CheckCompactWarning(state, tokens, "claude-sonnet-4-20250514")
	if level2 != "" {
		t.Errorf("expected empty level on second call, got %q", level2)
	}
	if warning2 != "" {
		t.Errorf("expected empty warning on second call, got %q", warning2)
	}
}

func TestWarning_At90Pct_FiresCritical(t *testing.T) {
	state := &CompactWarningState{}
	// 90% of 180000 = 162000
	tokens := 162000

	// Should fire warning first (80% is also exceeded)
	warning, level := CheckCompactWarning(state, tokens, "claude-sonnet-4-20250514")
	// Since 90% check is done first, it should fire critical
	if level != "critical" {
		t.Errorf("expected level=critical, got %q", level)
	}
	if warning == "" {
		t.Error("expected non-empty critical warning message")
	}
	if !state.CriticalFired {
		t.Error("expected CriticalFired=true")
	}

	// Second call should not fire critical again, but warning hasn't fired yet
	warning2, level2 := CheckCompactWarning(state, tokens, "claude-sonnet-4-20250514")
	if level2 != "warning" {
		t.Errorf("expected level=warning on second call (80%% not yet fired), got %q", level2)
	}
	if warning2 == "" {
		t.Error("expected non-empty warning on second call")
	}

	// Third call: both fired, nothing
	warning3, level3 := CheckCompactWarning(state, tokens, "claude-sonnet-4-20250514")
	if level3 != "" {
		t.Errorf("expected empty level on third call, got %q", level3)
	}
	if warning3 != "" {
		t.Errorf("expected empty warning on third call, got %q", warning3)
	}
}

func TestWarning_BelowThreshold_NoWarning(t *testing.T) {
	state := &CompactWarningState{}
	// 50% of 180000 = 90000
	tokens := 90000

	warning, level := CheckCompactWarning(state, tokens, "claude-sonnet-4-20250514")
	if level != "" {
		t.Errorf("expected empty level below threshold, got %q", level)
	}
	if warning != "" {
		t.Errorf("expected empty warning below threshold, got %q", warning)
	}
}

func TestWarning_ProgressiveThresholds(t *testing.T) {
	state := &CompactWarningState{}

	// Start below 80%
	_, level := CheckCompactWarning(state, 100000, "claude-sonnet-4-20250514")
	if level != "" {
		t.Error("should not warn below 80%")
	}

	// Hit 80%
	_, level = CheckCompactWarning(state, 144000, "claude-sonnet-4-20250514")
	if level != "warning" {
		t.Errorf("expected warning at 80%%, got %q", level)
	}

	// Hit 90%
	_, level = CheckCompactWarning(state, 162000, "claude-sonnet-4-20250514")
	if level != "critical" {
		t.Errorf("expected critical at 90%%, got %q", level)
	}

	// Both fired, nothing more
	_, level = CheckCompactWarning(state, 170000, "claude-sonnet-4-20250514")
	if level != "" {
		t.Errorf("expected no more warnings, got %q", level)
	}
}
