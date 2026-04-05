package query

import (
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/compact"
)

// ContinueSite identifies why the query loop continued instead of stopping.
// Each variant corresponds to a specific recovery or continuation path in
// the loop. This matches Claude Code's query.ts Continue.reason values.
type ContinueSite string

const (
	// SiteCollapseDrain means the loop continued because staged context
	// collapses were drained to recover from a 413 prompt-too-long error.
	SiteCollapseDrain ContinueSite = "collapse_drain"

	// SiteReactiveCompact means the loop continued after a reactive
	// compaction was performed to recover from a prompt-too-long error.
	SiteReactiveCompact ContinueSite = "reactive_compact"

	// SiteMaxOutputEscalate means the loop continued after escalating
	// max_tokens from the capped default to the full limit (EscalatedMaxTokens).
	SiteMaxOutputEscalate ContinueSite = "max_output_escalate"

	// SiteMaxOutputRecovery means the loop continued with a continuation
	// message after hitting the max_tokens limit (multi-turn retry).
	SiteMaxOutputRecovery ContinueSite = "max_output_recovery"

	// SiteStopHook means the loop continued because a stop hook returned
	// a blocking error that needs to be surfaced to the model.
	SiteStopHook ContinueSite = "stop_hook"

	// SiteTokenBudget means the loop continued because the token budget
	// has not been exhausted yet (nudge message injected).
	SiteTokenBudget ContinueSite = "token_budget"

	// SiteToolUse means the loop continued because the model returned
	// tool_use blocks that need to be executed (normal continuation).
	SiteToolUse ContinueSite = "tool_use"
)

// LoopState is the mutable state carried between query loop iterations.
// It matches the TypeScript State type from query.ts, tracking recovery
// counters, compaction state, and the transition that caused continuation.
type LoopState struct {
	// Messages is the conversation history (mutated each iteration).
	Messages []api.Message

	// MaxOutputTokensRecoveryCount tracks how many continuation retries
	// have been attempted for max_tokens truncation.
	MaxOutputTokensRecoveryCount int

	// HasAttemptedReactiveCompact is true if a reactive compact has been
	// tried during this query loop invocation.
	HasAttemptedReactiveCompact bool

	// MaxOutputTokensOverride is the escalated max_tokens value. Zero
	// means no override is active.
	MaxOutputTokensOverride int

	// PendingToolUseSummary receives the result of an async tool use
	// summary generation (fire-and-forget pattern). Nil when no summary
	// is pending.
	PendingToolUseSummary <-chan *ToolUseSummaryResult

	// StopHookActive is true when stop hooks are currently executing
	// for the current turn.
	StopHookActive bool

	// TurnCount tracks the number of loop iterations completed.
	TurnCount int

	// Transition records why the previous iteration continued. Nil on
	// the first iteration. Lets tests assert recovery paths fired
	// without inspecting message contents.
	Transition *ContinueSite

	// BudgetTracker tracks token budget usage across continuation turns.
	BudgetTracker *BudgetTracker

	// CompactWarningState tracks whether compaction warnings have fired.
	CompactWarningState *compact.CompactWarningState

	// CachedMicroCompactState tracks cache-aware microcompact state for
	// APIMicroCompact.
	CachedMicroCompactState *compact.CachedMicroCompactState
}

// NewLoopState creates a fresh LoopState from an initial message slice.
// All recovery counters start at zero, and no transition is recorded.
func NewLoopState(messages []api.Message) *LoopState {
	return &LoopState{
		Messages:                     messages,
		MaxOutputTokensRecoveryCount: 0,
		HasAttemptedReactiveCompact:  false,
		MaxOutputTokensOverride:      0,
		PendingToolUseSummary:        nil,
		StopHookActive:               false,
		TurnCount:                    0,
		Transition:                   nil,
		BudgetTracker:                NewBudgetTracker(),
		CompactWarningState:          &compact.CompactWarningState{},
		CachedMicroCompactState:      compact.NewCachedMicroCompactState(),
	}
}

// ResetForToolUse resets recovery counters when entering a new tool-use
// continuation. The recovery state is per-assistant-turn: once the model
// responds with tool_use, previous max_tokens recovery attempts no longer
// apply.
func (s *LoopState) ResetForToolUse() {
	s.MaxOutputTokensRecoveryCount = 0
	s.MaxOutputTokensOverride = 0
	s.HasAttemptedReactiveCompact = false
	s.StopHookActive = false
}

// SetTransition records the continue-site that caused the loop to iterate
// again. This is set at each continue point and read by tests and logging.
func (s *LoopState) SetTransition(site ContinueSite) {
	s.Transition = &site
}
