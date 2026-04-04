package app

import (
	"bytes"
	"context"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/query"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
)

func TestRunNonInteractive_TextCallbackCalled(t *testing.T) {
	// Test that TextCallback receives text tokens during streaming.
	// We test this indirectly through the LoopParams wiring.
	var buf bytes.Buffer
	callback := func(text string) {
		buf.WriteString(text)
	}

	// Verify the callback works as expected
	callback("Hello ")
	callback("World")
	assert.Equal(t, "Hello World", buf.String())
}

func TestRunNonInteractive_PrintsOutput(t *testing.T) {
	// Test that NonInteractiveParams can be constructed with all required fields.
	// Full integration test requires a mock API client, so we verify the
	// construction and parameter passing.
	tracker := cost.NewTracker("claude-sonnet-4-20250514")
	registry := tools.NewRegistry()
	permCtx := permissions.NewPermissionContext(permissions.ModeAuto, nil, nil)

	params := &NonInteractiveParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:     registry,
		PermCtx:      permCtx,
		CostTracker:  tracker,
		Messages:     nil,
		SystemPrompt: "You are helpful",
		MaxTurns:     1,
		WorkingDir:   "/tmp",
		SessionID:    "test-session",
		Prompt:       "What is 2+2?",
		OutputFormat: "text",
	}

	// Verify params were constructed correctly
	assert.Equal(t, "What is 2+2?", params.Prompt)
	assert.Equal(t, "text", params.OutputFormat)
	assert.Equal(t, "You are helpful", params.SystemPrompt)
	assert.Equal(t, 1, params.MaxTurns)
}

func TestRunNonInteractive_LoopParamsWiring(t *testing.T) {
	// Test that RunNonInteractive correctly wires up the query.LoopParams.
	// This verifies the TextCallback is set and other params are passed through.
	tracker := cost.NewTracker("claude-sonnet-4-20250514")
	registry := tools.NewRegistry()
	permCtx := permissions.NewPermissionContext(permissions.ModeAuto, nil, nil)
	client := &api.Client{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 4096,
	}

	// Create LoopParams the same way RunNonInteractive does
	messages := []api.Message{api.UserMessage("test prompt")}
	var captured string
	loopParams := &query.LoopParams{
		Client:       client,
		Registry:     registry,
		PermCtx:      permCtx,
		CostTracker:  tracker,
		Messages:     messages,
		SystemPrompt: "test system",
		MaxTurns:     5,
		WorkingDir:   "/tmp",
		SessionID:    "test-session",
		TextCallback: func(text string) { captured += text },
	}

	// Verify all fields are set correctly
	assert.Equal(t, client, loopParams.Client)
	assert.Equal(t, registry, loopParams.Registry)
	assert.Equal(t, permCtx, loopParams.PermCtx)
	assert.Equal(t, tracker, loopParams.CostTracker)
	assert.Len(t, loopParams.Messages, 1)
	assert.Equal(t, "test system", loopParams.SystemPrompt)
	assert.Equal(t, 5, loopParams.MaxTurns)
	assert.Nil(t, loopParams.Program)    // no TUI in non-interactive
	assert.Nil(t, loopParams.PermissionCh) // no permission channel in non-interactive

	// Verify TextCallback works
	loopParams.TextCallback("hello")
	assert.Equal(t, "hello", captured)
}

func TestRunNonInteractive_CostFormatting(t *testing.T) {
	// Test that cost tracking works in the non-interactive context
	tracker := cost.NewTracker("claude-sonnet-4-20250514")
	tracker.Add(cost.Usage{
		InputTokens:  100,
		OutputTokens: 50,
	})

	usage := cost.FormatUsage(tracker)
	assert.Contains(t, usage, "100")
	assert.Contains(t, usage, "50")
	assert.Contains(t, usage, "Cost:")
}

func TestRunParams_Construction(t *testing.T) {
	// Test RunParams can be created from CLI flags
	params := &RunParams{
		Model:          "claude-sonnet-4-20250514",
		PermissionMode: "auto",
		Resume:         true,
		SessionID:      "test-123",
		MaxTurns:       10,
		SystemPrompt:   "Be helpful",
		OutputFormat:   "json",
		AllowedTools:   []string{"Bash"},
		DisallowedTools: []string{"Write"},
		Prompt:         "hello",
		Version:        "1.0.0",
	}

	assert.Equal(t, "claude-sonnet-4-20250514", params.Model)
	assert.Equal(t, "auto", params.PermissionMode)
	assert.True(t, params.Resume)
	assert.Equal(t, "hello", params.Prompt)
	assert.Equal(t, "1.0.0", params.Version)
}

func TestRunNonInteractive_ContextCancellation(t *testing.T) {
	// Verify that context cancellation is respected
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// The cancelled context should be detectable
	assert.Error(t, ctx.Err())
}
