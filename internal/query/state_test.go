package query

import (
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoopState_Defaults(t *testing.T) {
	msgs := []api.Message{api.UserMessage("hello")}
	state := NewLoopState(msgs)

	require.NotNil(t, state)
	assert.Len(t, state.Messages, 1)
	assert.Equal(t, "hello", state.Messages[0].Content[0].Text)
	assert.Equal(t, 0, state.MaxOutputTokensRecoveryCount)
	assert.False(t, state.HasAttemptedReactiveCompact)
	assert.Equal(t, 0, state.MaxOutputTokensOverride)
	assert.Nil(t, state.PendingToolUseSummary)
	assert.False(t, state.StopHookActive)
	assert.Equal(t, 0, state.TurnCount)
	assert.Nil(t, state.Transition)
	assert.NotNil(t, state.BudgetTracker)
	assert.NotNil(t, state.CompactWarningState)
	assert.NotNil(t, state.CachedMicroCompactState)
}

func TestNewLoopState_EmptyMessages(t *testing.T) {
	state := NewLoopState(nil)

	require.NotNil(t, state)
	assert.Nil(t, state.Messages)
	assert.Equal(t, 0, state.TurnCount)
}

func TestLoopState_ResetForToolUse(t *testing.T) {
	state := NewLoopState([]api.Message{api.UserMessage("test")})

	// Set some recovery state
	state.MaxOutputTokensRecoveryCount = 2
	state.MaxOutputTokensOverride = EscalatedMaxTokens
	state.HasAttemptedReactiveCompact = true
	state.StopHookActive = true

	// Reset
	state.ResetForToolUse()

	assert.Equal(t, 0, state.MaxOutputTokensRecoveryCount)
	assert.Equal(t, 0, state.MaxOutputTokensOverride)
	assert.False(t, state.HasAttemptedReactiveCompact)
	assert.False(t, state.StopHookActive)
}

func TestLoopState_ResetForToolUse_PreservesOtherState(t *testing.T) {
	state := NewLoopState([]api.Message{api.UserMessage("test")})
	state.TurnCount = 5
	site := SiteToolUse
	state.Transition = &site

	state.ResetForToolUse()

	// These should NOT be reset
	assert.Equal(t, 5, state.TurnCount)
	assert.NotNil(t, state.Transition)
	assert.NotNil(t, state.BudgetTracker)
}

func TestLoopState_SetTransition(t *testing.T) {
	state := NewLoopState([]api.Message{api.UserMessage("test")})

	// Initially nil
	assert.Nil(t, state.Transition)

	// Set a transition
	state.SetTransition(SiteMaxOutputEscalate)
	require.NotNil(t, state.Transition)
	assert.Equal(t, SiteMaxOutputEscalate, *state.Transition)

	// Override with another
	state.SetTransition(SiteToolUse)
	require.NotNil(t, state.Transition)
	assert.Equal(t, SiteToolUse, *state.Transition)
}

func TestLoopState_SetTransition_AllSites(t *testing.T) {
	// Verify all 7 continue sites can be set
	sites := []ContinueSite{
		SiteCollapseDrain,
		SiteReactiveCompact,
		SiteMaxOutputEscalate,
		SiteMaxOutputRecovery,
		SiteStopHook,
		SiteTokenBudget,
		SiteToolUse,
	}

	state := NewLoopState(nil)
	for _, site := range sites {
		state.SetTransition(site)
		require.NotNil(t, state.Transition, "transition should be set for site %s", site)
		assert.Equal(t, site, *state.Transition)
	}
}

func TestContinueSite_Values(t *testing.T) {
	// Verify the string values match Claude Code's transition reasons
	assert.Equal(t, ContinueSite("collapse_drain"), SiteCollapseDrain)
	assert.Equal(t, ContinueSite("reactive_compact"), SiteReactiveCompact)
	assert.Equal(t, ContinueSite("max_output_escalate"), SiteMaxOutputEscalate)
	assert.Equal(t, ContinueSite("max_output_recovery"), SiteMaxOutputRecovery)
	assert.Equal(t, ContinueSite("stop_hook"), SiteStopHook)
	assert.Equal(t, ContinueSite("token_budget"), SiteTokenBudget)
	assert.Equal(t, ContinueSite("tool_use"), SiteToolUse)
}
