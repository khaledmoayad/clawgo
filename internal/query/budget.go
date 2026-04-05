package query

import (
	"fmt"
	"time"

	"golang.org/x/text/message"
	"golang.org/x/text/language"
)

const (
	// CompletionThreshold is the fraction of the budget at which the tracker
	// decides to stop (90% consumed -> stop).
	CompletionThreshold = 0.9

	// DiminishingThreshold is the minimum token delta between consecutive
	// checks. If the delta falls below this for 3+ consecutive checks,
	// diminishing returns are detected and the tracker stops.
	DiminishingThreshold = 500
)

// BudgetTracker tracks cumulative output token usage across continuation
// turns and decides whether to continue or stop the query loop.
type BudgetTracker struct {
	ContinuationCount    int
	LastDeltaTokens      int
	LastGlobalTurnTokens int
	StartedAt            time.Time
}

// NewBudgetTracker creates a fresh BudgetTracker with zero counts and
// the current timestamp.
func NewBudgetTracker() *BudgetTracker {
	return &BudgetTracker{
		ContinuationCount:    0,
		LastDeltaTokens:      0,
		LastGlobalTurnTokens: 0,
		StartedAt:            time.Now(),
	}
}

// TokenBudgetDecision is the interface for continue/stop decisions
// returned by CheckTokenBudget.
type TokenBudgetDecision interface {
	Action() string // "continue" or "stop"
}

// ContinueDecision indicates the query loop should continue with a
// nudge message injected into the conversation.
type ContinueDecision struct {
	NudgeMessage      string
	ContinuationCount int
	Pct               int
	TurnTokens        int
	Budget            int
}

// Action returns "continue".
func (d ContinueDecision) Action() string { return "continue" }

// StopDecision indicates the query loop should stop. CompletionEvent
// is non-nil when there is telemetry to log.
type StopDecision struct {
	CompletionEvent *CompletionEvent
}

// Action returns "stop".
func (d StopDecision) Action() string { return "stop" }

// CompletionEvent carries telemetry data about a completed budget run.
type CompletionEvent struct {
	ContinuationCount  int
	Pct                int
	TurnTokens         int
	Budget             int
	DiminishingReturns bool
	DurationMs         int64
}

// CheckTokenBudget evaluates whether the query loop should continue or
// stop based on the current token budget state.
//
// It returns StopDecision{nil} when:
//   - agentID is non-empty (sub-agents don't use budgets)
//   - budget is <= 0 (no budget configured)
//
// It returns ContinueDecision when under the 90% threshold and not
// experiencing diminishing returns.
//
// It returns StopDecision with a CompletionEvent when the budget is
// exhausted or diminishing returns are detected.
func CheckTokenBudget(
	tracker *BudgetTracker,
	agentID string,
	budget int,
	globalTurnTokens int,
) TokenBudgetDecision {
	// Sub-agents and no-budget cases always stop
	if agentID != "" || budget <= 0 {
		return StopDecision{CompletionEvent: nil}
	}

	turnTokens := globalTurnTokens
	pct := 0
	if budget > 0 {
		pct = int(float64(turnTokens) / float64(budget) * 100 + 0.5)
	}

	deltaSinceLastCheck := globalTurnTokens - tracker.LastGlobalTurnTokens

	isDiminishing := tracker.ContinuationCount >= 3 &&
		deltaSinceLastCheck < DiminishingThreshold &&
		tracker.LastDeltaTokens < DiminishingThreshold

	// Under threshold and not diminishing -> continue
	if !isDiminishing && float64(turnTokens) < float64(budget)*CompletionThreshold {
		tracker.ContinuationCount++
		tracker.LastDeltaTokens = deltaSinceLastCheck
		tracker.LastGlobalTurnTokens = globalTurnTokens
		return ContinueDecision{
			NudgeMessage:      GetBudgetContinuationMessage(pct, turnTokens, budget),
			ContinuationCount: tracker.ContinuationCount,
			Pct:               pct,
			TurnTokens:        turnTokens,
			Budget:            budget,
		}
	}

	// Diminishing returns or had prior continuations -> stop with event
	if isDiminishing || tracker.ContinuationCount > 0 {
		return StopDecision{
			CompletionEvent: &CompletionEvent{
				ContinuationCount:  tracker.ContinuationCount,
				Pct:                pct,
				TurnTokens:         turnTokens,
				Budget:             budget,
				DiminishingReturns: isDiminishing,
				DurationMs:         time.Since(tracker.StartedAt).Milliseconds(),
			},
		}
	}

	// Default: stop with no event (first call, already at threshold)
	return StopDecision{CompletionEvent: nil}
}

// GetBudgetContinuationMessage builds the nudge message injected when
// the query loop auto-continues under budget.
func GetBudgetContinuationMessage(pct int, turnTokens int, budget int) string {
	p := message.NewPrinter(language.English)
	return fmt.Sprintf(
		"Stopped at %d%% of token target (%s / %s). Keep working \u2014 do not summarize.",
		pct,
		p.Sprintf("%d", turnTokens),
		p.Sprintf("%d", budget),
	)
}
