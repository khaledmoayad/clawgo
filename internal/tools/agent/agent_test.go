package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/khaledmoayad/clawgo/internal/tools/tasks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubClient creates a minimal Client for testing (no real API connection).
func stubClient() *api.Client {
	return &api.Client{
		Model:     "test-model",
		MaxTokens: 1024,
	}
}

func TestAgentTool_Name(t *testing.T) {
	tool := New(nil, nil)
	assert.Equal(t, "Agent", tool.Name())
}

func TestAgentTool_IsReadOnly(t *testing.T) {
	tool := New(nil, nil)
	assert.False(t, tool.IsReadOnly())
}

func TestAgentTool_IsConcurrencySafe(t *testing.T) {
	tool := New(nil, nil)
	assert.False(t, tool.IsConcurrencySafe(nil))
}

func TestAgentTool_CheckPermissions_ReturnsAsk(t *testing.T) {
	tool := New(nil, nil)
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)
	result, err := tool.CheckPermissions(context.Background(), nil, permCtx)
	require.NoError(t, err)
	assert.Equal(t, permissions.Ask, result)
}

func TestAgentTool_InputSchema(t *testing.T) {
	tool := New(nil, nil)
	schema := tool.InputSchema()
	assert.NotEmpty(t, schema)

	var schemaMap map[string]any
	err := json.Unmarshal(schema, &schemaMap)
	require.NoError(t, err)
	assert.Equal(t, "object", schemaMap["type"])

	props := schemaMap["properties"].(map[string]any)
	assert.Contains(t, props, "prompt")
	assert.Contains(t, props, "model")
	assert.Contains(t, props, "permitted_tools")
	assert.Contains(t, props, "subagent_type")
	assert.Contains(t, props, "name")
	assert.Contains(t, props, "description")
	assert.Contains(t, props, "run_in_background")
	assert.Contains(t, props, "isolation")
	assert.Contains(t, props, "team_name")
	assert.Contains(t, props, "cwd")
}

func TestAgentTool_Call_MissingPrompt(t *testing.T) {
	tool := New(nil, nil)
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
	}

	inp := json.RawMessage(`{}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "prompt")
}

func TestAgentTool_Call_MaxNestingDepth(t *testing.T) {
	tool := New(nil, nil)
	tool.NestingDepth = MaxNestingDepth // At max depth
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
	}

	inp := json.RawMessage(`{"prompt": "do something"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Maximum agent nesting depth reached")
}

func TestAgentTool_Call_BeyondMaxNestingDepth(t *testing.T) {
	tool := New(nil, nil)
	tool.NestingDepth = MaxNestingDepth + 1 // Beyond max depth
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
	}

	inp := json.RawMessage(`{"prompt": "do something"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Maximum agent nesting depth reached")
}

func TestAgentTool_Call_ContextCancellation(t *testing.T) {
	tool := New(nil, nil)
	tool.NestingDepth = 0

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	// Without a real client, the call will fail at the RunLoop step.
	// This test verifies the input validation and nesting check pass.
	inp := json.RawMessage(`{"prompt": "do something"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	// With cancelled context and no client, we get an error.
	// The important thing is it doesn't panic and handles gracefully.
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestAgentTool_Call_ValidInput_SubAgentParams(t *testing.T) {
	// Test that valid input with prompt gets past validation and nesting check.
	// Without a real API client, RunLoop will fail, but we can verify the error
	// is from the loop execution (not from validation or nesting).
	reg := tools.NewRegistry()
	tool := New(reg, nil)
	tool.NestingDepth = 0

	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "analyze the code"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	// Without a client, we expect an error result about the sub-agent
	assert.NotNil(t, result)
}

func TestAgentTool_MaxNestingDepthConstant(t *testing.T) {
	assert.Equal(t, 3, MaxNestingDepth)
}

func TestAgentTool_MaxAgentTurnsConstant(t *testing.T) {
	assert.Equal(t, 20, MaxAgentTurns)
}

func TestAgentTool_CoordinatorMode_ReturnsImmediately(t *testing.T) {
	// In coordinator mode with "worker" subagent_type, Call should return
	// immediately with a task ID rather than blocking.
	store := tasks.NewStore()
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.CoordinatorMode = true
	tool.TaskStore = store

	// Cancel ctx immediately so the goroutine finishes quickly
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "background work", "subagent_type": "worker"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "task_id: task_1")
	assert.Contains(t, result.Content[0].Text, "launched")

	// Verify task was created in store
	task, ok := store.Get("task_1")
	assert.True(t, ok)
	assert.Equal(t, "running", task.Status)
	assert.Equal(t, "local_agent", task.Type)
}

func TestAgentTool_DefaultMode_BlocksUntilComplete(t *testing.T) {
	// In default mode (CoordinatorMode=false), Call should block.
	// Without a real API client, RunLoop returns an error immediately,
	// verifying the blocking path is taken.
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.CoordinatorMode = false

	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	// With no real API client, this will error from RunLoop but NOT return a task ID
	inp := json.RawMessage(`{"prompt": "blocking task"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Result should NOT contain "task_id" (not async mode)
	assert.NotContains(t, result.Content[0].Text, "task_id")
}

func TestAgentTool_TaskStore_UpdatedAfterGoroutine(t *testing.T) {
	// Verify that the task store is updated when the goroutine completes.
	store := tasks.NewStore()
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.CoordinatorMode = true
	tool.TaskStore = store

	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "async task", "subagent_type": "worker"}`)
	_, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)

	// Wait for the goroutine to finish (it will fail quickly without a real client)
	time.Sleep(200 * time.Millisecond)

	task, ok := store.Get("task_1")
	assert.True(t, ok)
	// Without a client, RunLoop fails, so task should be "failed"
	assert.Equal(t, "failed", task.Status)
}

func TestAgentTool_ContextCancellation_StopsGoroutine(t *testing.T) {
	// Verify that cancelling the parent context stops the sub-agent goroutine.
	store := tasks.NewStore()
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.CoordinatorMode = true
	tool.TaskStore = store

	ctx, cancel := context.WithCancel(context.Background())
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "cancellable task", "subagent_type": "worker"}`)
	_, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)

	// Cancel the context
	cancel()

	// Wait for goroutine to handle cancellation
	time.Sleep(200 * time.Millisecond)

	task, ok := store.Get("task_1")
	assert.True(t, ok)
	// Task should end up in failed state due to cancelled context
	assert.Contains(t, []string{"failed", "stopped"}, task.Status)
}

func TestAgentTool_NestingDepth_EnforcedInGoroutineMode(t *testing.T) {
	// Nesting depth enforcement should work in both blocking and async modes.
	store := tasks.NewStore()
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = MaxNestingDepth
	tool.CoordinatorMode = true
	tool.TaskStore = store

	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "deep task", "subagent_type": "worker"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Maximum agent nesting depth reached")
}

func TestAgentTool_SubagentType_DefaultIsSubagent(t *testing.T) {
	// When subagent_type is not specified, it should default to "subagent"
	// (blocking behavior even in coordinator mode).
	store := tasks.NewStore()
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.CoordinatorMode = true
	tool.TaskStore = store

	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	// No subagent_type specified - should use blocking mode (default "subagent")
	inp := json.RawMessage(`{"prompt": "regular task"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Should NOT contain task_id since default subagent_type triggers blocking mode
	assert.NotContains(t, result.Content[0].Text, "task_id")
}

// --- New tests for Task 2: parameter handling ---

func TestAgentRunInBackground(t *testing.T) {
	// run_in_background=true should trigger async execution and return a task_id immediately,
	// regardless of coordinator mode or subagent_type.
	store := tasks.NewStore()
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.CoordinatorMode = false // NOT in coordinator mode
	tool.TaskStore = store

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	// Explicit run_in_background=true with default subagent_type
	inp := json.RawMessage(`{"prompt": "background research", "run_in_background": true}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "task_id: task_1")
	assert.Contains(t, result.Content[0].Text, "background")

	// Verify task was created in store
	task, ok := store.Get("task_1")
	assert.True(t, ok)
	assert.Equal(t, "running", task.Status)
	assert.Equal(t, "local_agent", task.Type)
}

func TestAgentCwdOverride(t *testing.T) {
	// cwd="/tmp" should be accepted (absolute path).
	// We verify it passes validation and reaches the loop (which fails without a client).
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0

	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/home/user/project",
		ProjectRoot: "/home/user/project",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "work in tmp", "cwd": "/tmp"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Should NOT be a validation error -- the error should be from RunLoop, not from input validation
	if result.IsError {
		assert.NotContains(t, result.Content[0].Text, "cwd must be an absolute path")
		assert.NotContains(t, result.Content[0].Text, "mutually exclusive")
	}
}

func TestAgentCwdRelativeError(t *testing.T) {
	// A relative cwd should be rejected with a validation error.
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0

	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "work in relative", "cwd": "relative/path"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "cwd must be an absolute path")
}

func TestAgentCwdWithWorktreeError(t *testing.T) {
	// Setting both cwd and isolation="worktree" should return a mutual exclusivity error.
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0

	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "conflicting options", "cwd": "/tmp", "isolation": "worktree"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "mutually exclusive")
}

func TestAgentIsolationInvalid(t *testing.T) {
	// An invalid isolation value should be rejected.
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0

	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "invalid isolation", "isolation": "invalid"}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "invalid isolation mode")
}

func TestAgentNameInResult(t *testing.T) {
	// When name is provided for a background agent, the result message should include the name.
	store := tasks.NewStore()
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.TaskStore = store

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "named agent work", "name": "test-agent", "run_in_background": true}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Agent 'test-agent'")

	// Verify the name is stored in the task
	task, ok := store.Get("task_1")
	assert.True(t, ok)
	assert.Equal(t, "test-agent", task.Name)
}

func TestAgentDescriptionInResult(t *testing.T) {
	// When description is provided, it should appear in the result message.
	store := tasks.NewStore()
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.TaskStore = store

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "run tests", "description": "Run unit tests", "run_in_background": true}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "(Run unit tests)")

	// Verify description is used as task description
	task, ok := store.Get("task_1")
	assert.True(t, ok)
	assert.Equal(t, "Run unit tests", task.Description)
}

func TestAgentTeamNameInSystemPrompt(t *testing.T) {
	// When team_name is set, it should be included in the system prompt.
	tool := &AgentTool{NestingDepth: 0}

	in := &input{
		Prompt:   "do work",
		TeamName: "my-team",
	}

	sysPrompt := tool.buildSystemPrompt(in)
	assert.Contains(t, sysPrompt, "You are part of team 'my-team'.")
	assert.Contains(t, sysPrompt, "sub-agent")
}

func TestAgentTeamNameInSystemPrompt_BackgroundWorker(t *testing.T) {
	// For a coordinator-mode worker, the system prompt should include both team and worker role.
	tool := &AgentTool{NestingDepth: 0, CoordinatorMode: true}

	in := &input{
		Prompt:       "do work",
		TeamName:     "alpha-team",
		SubagentType: "worker",
	}

	sysPrompt := tool.buildSystemPrompt(in)
	assert.Contains(t, sysPrompt, "You are part of team 'alpha-team'.")
	// Coordinator+worker triggers "worker agent" in system prompt
	assert.Contains(t, sysPrompt, "worker agent")
}

func TestAgentTeamNameInSystemPrompt_RunInBackground(t *testing.T) {
	// With run_in_background=true, system prompt should say "worker agent"
	tool := &AgentTool{NestingDepth: 0}

	in := &input{
		Prompt:          "background work",
		TeamName:        "beta-team",
		RunInBackground: true,
	}

	sysPrompt := tool.buildSystemPrompt(in)
	assert.Contains(t, sysPrompt, "You are part of team 'beta-team'.")
	assert.Contains(t, sysPrompt, "worker agent")
}

func TestAgentInputParsing(t *testing.T) {
	// Verify that a full input JSON with all fields parses correctly.
	rawInput := json.RawMessage(`{
		"prompt": "analyze codebase",
		"description": "Code analysis",
		"model": "opus",
		"permitted_tools": ["FileRead", "Grep"],
		"subagent_type": "worker",
		"name": "analyzer",
		"team_name": "review-team",
		"run_in_background": true,
		"isolation": "worktree",
		"cwd": ""
	}`)

	var in input
	err := json.Unmarshal(rawInput, &in)
	require.NoError(t, err)

	assert.Equal(t, "analyze codebase", in.Prompt)
	assert.Equal(t, "Code analysis", in.Description)
	assert.Equal(t, "opus", in.Model)
	assert.Equal(t, []string{"FileRead", "Grep"}, in.PermittedTools)
	assert.Equal(t, "worker", in.SubagentType)
	assert.Equal(t, "analyzer", in.Name)
	assert.Equal(t, "review-team", in.TeamName)
	assert.True(t, in.RunInBackground)
	assert.Equal(t, "worktree", in.Isolation)
	assert.Equal(t, "", in.Cwd)
}

func TestFormatResultMessage(t *testing.T) {
	tests := []struct {
		name     string
		in       *input
		baseMsg  string
		expected string
	}{
		{
			name:     "no name or description",
			in:       &input{Prompt: "work"},
			baseMsg:  "launched",
			expected: "Sub-agent launched",
		},
		{
			name:     "with name only",
			in:       &input{Prompt: "work", Name: "my-agent"},
			baseMsg:  "launched",
			expected: "Agent 'my-agent' launched",
		},
		{
			name:     "with description only",
			in:       &input{Prompt: "work", Description: "Code review"},
			baseMsg:  "launched",
			expected: "Sub-agent (Code review) launched",
		},
		{
			name:     "with name and description",
			in:       &input{Prompt: "work", Name: "reviewer", Description: "Code review"},
			baseMsg:  "launched",
			expected: "Agent 'reviewer' (Code review) launched",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatResultMessage(tt.in, tt.baseMsg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAgentDescriptionAsTaskDescription(t *testing.T) {
	// When description is provided, it should be used as the task store description.
	// When not provided, the prompt should be used as fallback.
	store := tasks.NewStore()
	reg := tools.NewRegistry()

	// Test with description
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.TaskStore = store

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp := json.RawMessage(`{"prompt": "long prompt text here", "description": "Short desc", "run_in_background": true}`)
	_, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)

	task, ok := store.Get("task_1")
	assert.True(t, ok)
	assert.Equal(t, "Short desc", task.Description)

	// Test without description -- prompt should be used
	store2 := tasks.NewStore()
	tool2 := New(reg, stubClient())
	tool2.NestingDepth = 0
	tool2.TaskStore = store2

	toolCtx2 := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	inp2 := json.RawMessage(`{"prompt": "do the work", "run_in_background": true}`)
	_, err = tool2.Call(ctx, inp2, toolCtx2)
	require.NoError(t, err)

	task2, ok := store2.Get("task_1")
	assert.True(t, ok)
	assert.Equal(t, "do the work", task2.Description)
}

func TestAgentRunInBackground_WithSubagentType(t *testing.T) {
	// run_in_background=true should work with any subagent_type, not just "worker".
	store := tasks.NewStore()
	reg := tools.NewRegistry()
	tool := New(reg, stubClient())
	tool.NestingDepth = 0
	tool.CoordinatorMode = false
	tool.TaskStore = store

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
		AbortCtx:    ctx,
	}

	// subagent_type is "subagent" (default) + run_in_background=true
	inp := json.RawMessage(`{"prompt": "background subagent", "subagent_type": "subagent", "run_in_background": true}`)
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "task_id: task_1")
}
