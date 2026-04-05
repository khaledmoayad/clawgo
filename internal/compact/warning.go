package compact

// Compaction warning thresholds. CheckCompactWarning fires warnings
// when context usage approaches the limit, giving users a chance to
// manually compact before auto-compact triggers.

const (
	// WarningThresholdPct is the context usage percentage at which a
	// warning is first fired (80%).
	WarningThresholdPct = 80

	// CriticalThresholdPct is the context usage percentage at which a
	// critical warning is fired (90%).
	CriticalThresholdPct = 90
)

// CompactWarningState tracks whether each warning level has already
// fired for the current session. Each level fires at most once to
// avoid spamming the user.
type CompactWarningState struct {
	WarningFired  bool
	CriticalFired bool
}

// CheckCompactWarning evaluates whether a compaction warning should be
// shown based on current token usage relative to the model's effective
// context window.
//
// Returns:
//   - warning: the warning message to display (empty if no warning)
//   - level: "warning", "critical", or "" (empty if no warning)
//
// Each threshold fires at most once. After firing, subsequent calls
// at the same level return empty strings.
func CheckCompactWarning(state *CompactWarningState, tokenCount int, model string) (warning string, level string) {
	effective := GetEffectiveContextWindowSize(model)
	if effective <= 0 {
		return "", ""
	}

	pct := int(float64(tokenCount) / float64(effective) * 100)

	// Check critical first (90%)
	if pct >= CriticalThresholdPct && !state.CriticalFired {
		state.CriticalFired = true
		return "Context window 90% full. Auto-compact will trigger soon.", "critical"
	}

	// Check warning (80%)
	if pct >= WarningThresholdPct && !state.WarningFired {
		state.WarningFired = true
		return "Context window 80% full. Consider using /compact.", "warning"
	}

	return "", ""
}
