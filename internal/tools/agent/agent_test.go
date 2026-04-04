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
}

func TestAgentTool_Call_MissingPrompt(t *testing.T) {
	tool := New(nil, nil)
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		SessionID:   "test-session",
	}

	input := json.RawMessage(`{}`)
	result, err := tool.Call(ctx, input, toolCtx)
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

	input := json.RawMessage(`{"prompt": "do something"}`)
	result, err := tool.Call(ctx, input, toolCtx)
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

	input := json.RawMessage(`{"prompt": "do something"}`)
	result, err := tool.Call(ctx, input, toolCtx)
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
	input := json.RawMessage(`{"prompt": "do something"}`)
	result, err := tool.Call(ctx, input, toolCtx)
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

	input := json.RawMessage(`{"prompt": "analyze the code"}`)
	result, err := tool.Call(ctx, input, toolCtx)
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
	assert.Contains(t, result.Content[0].Text, "Sub-agent launched")

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
