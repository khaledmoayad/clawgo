package query

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTool implements the tools.Tool interface for testing.
type mockTool struct {
	name       string
	readOnly   bool
	callResult *tools.ToolResult
	callErr    error
	callCount  int
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Description() string          { return "Mock tool for testing" }
func (m *mockTool) IsReadOnly() bool             { return m.readOnly }
func (m *mockTool) IsConcurrencySafe(_ json.RawMessage) bool { return m.readOnly }
func (m *mockTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (m *mockTool) Call(_ context.Context, _ json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	m.callCount++
	return m.callResult, m.callErr
}
func (m *mockTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission(m.name, m.readOnly, permCtx), nil
}

// TestRunLoop_EndTurn tests that the loop exits on "end_turn" stop reason.
// This test requires a real API client or a mock, so we test buildRequest instead.
func TestBuildRequest_BasicMessage(t *testing.T) {
	registry := tools.NewRegistry()
	tracker := cost.NewTracker("claude-sonnet-4-20250514")

	msgs := []api.Message{api.UserMessage("hello")}
	params := &LoopParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:     registry,
		CostTracker:  tracker,
		PermCtx:      permissions.NewPermissionContext(permissions.ModeDefault, nil, nil),
		Messages:     msgs,
		SystemPrompt: "You are helpful",
	}

	state := NewLoopState(msgs)
	req := buildRequest(params, state)
	assert.Equal(t, "claude-sonnet-4-20250514", req.Model)
	assert.Equal(t, int64(4096), req.MaxTokens)
	assert.Len(t, req.Messages, 1)
	assert.NotEmpty(t, req.System)
}

func TestBuildRequest_WithTools(t *testing.T) {
	mt := &mockTool{name: "test_tool", readOnly: true, callResult: tools.TextResult("ok")}
	registry := tools.NewRegistry(mt)
	tracker := cost.NewTracker("claude-sonnet-4-20250514")

	msgs := []api.Message{api.UserMessage("hello")}
	params := &LoopParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:    registry,
		CostTracker: tracker,
		PermCtx:     permissions.NewPermissionContext(permissions.ModeDefault, nil, nil),
		Messages:    msgs,
	}

	state := NewLoopState(msgs)
	req := buildRequest(params, state)
	assert.Len(t, req.Tools, 1)
	assert.Equal(t, "test_tool", req.Tools[0].OfTool.Name)
}

func TestBuildRequest_NoSystemPrompt(t *testing.T) {
	registry := tools.NewRegistry()
	tracker := cost.NewTracker("claude-sonnet-4-20250514")

	msgs := []api.Message{api.UserMessage("hello")}
	params := &LoopParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:    registry,
		CostTracker: tracker,
		PermCtx:     permissions.NewPermissionContext(permissions.ModeDefault, nil, nil),
		Messages:    msgs,
	}

	state := NewLoopState(msgs)
	req := buildRequest(params, state)
	assert.Nil(t, req.System)
}

func TestExecuteToolUses_UnknownTool(t *testing.T) {
	registry := tools.NewRegistry()
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)
	tracker := cost.NewTracker("claude-sonnet-4-20250514")

	params := &LoopParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:    registry,
		PermCtx:     permCtx,
		CostTracker: tracker,
	}

	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type:  "tool_use",
				ID:    "test-id",
				Name:  "nonexistent_tool",
				Input: json.RawMessage(`{}`),
			},
		},
	}

	results, err := executeToolUses(context.Background(), msg, params)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Content, "Unknown tool")
	assert.True(t, results[0].IsError)
}

func TestExecuteToolUses_ReadOnlyAutoApproved(t *testing.T) {
	mt := &mockTool{
		name:       "file_read",
		readOnly:   true,
		callResult: tools.TextResult("file contents"),
	}
	registry := tools.NewRegistry(mt)
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)
	tracker := cost.NewTracker("claude-sonnet-4-20250514")

	params := &LoopParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:    registry,
		PermCtx:     permCtx,
		CostTracker: tracker,
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
	}

	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type:  "tool_use",
				ID:    "read-id",
				Name:  "file_read",
				Input: json.RawMessage(`{"path":"test.go"}`),
			},
		},
	}

	results, err := executeToolUses(context.Background(), msg, params)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "file contents", results[0].Content)
	assert.False(t, results[0].IsError)
	assert.Equal(t, 1, mt.callCount)
}

func TestExecuteToolUses_PermissionDenied(t *testing.T) {
	mt := &mockTool{
		name:       "bash",
		readOnly:   false,
		callResult: tools.TextResult("output"),
	}
	registry := tools.NewRegistry(mt)
	// Disallow bash explicitly
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, []string{"bash"})
	tracker := cost.NewTracker("claude-sonnet-4-20250514")

	params := &LoopParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:    registry,
		PermCtx:     permCtx,
		CostTracker: tracker,
	}

	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type:  "tool_use",
				ID:    "bash-id",
				Name:  "bash",
				Input: json.RawMessage(`{"command":"rm -rf /"}`),
			},
		},
	}

	results, err := executeToolUses(context.Background(), msg, params)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Content, "disallowed")
	assert.True(t, results[0].IsError)
	assert.Equal(t, 0, mt.callCount) // Should not have been called
}

func TestExecuteToolUses_PermissionAskDenied(t *testing.T) {
	mt := &mockTool{
		name:       "bash",
		readOnly:   false,
		callResult: tools.TextResult("output"),
	}
	registry := tools.NewRegistry(mt)
	// Default mode with non-read-only tool = Ask
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)
	tracker := cost.NewTracker("claude-sonnet-4-20250514")

	permissionCh := make(chan permissions.PermissionResult, 1)
	permissionCh <- permissions.Deny // User denies

	params := &LoopParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:     registry,
		PermCtx:      permCtx,
		CostTracker:  tracker,
		PermissionCh: permissionCh,
	}

	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type:  "tool_use",
				ID:    "bash-id",
				Name:  "bash",
				Input: json.RawMessage(`{"command":"ls"}`),
			},
		},
	}

	results, err := executeToolUses(context.Background(), msg, params)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Content, "disallowed")
	assert.True(t, results[0].IsError)
	assert.Equal(t, 0, mt.callCount)
}

func TestExecuteToolUses_PermissionAskApproved(t *testing.T) {
	mt := &mockTool{
		name:       "bash",
		readOnly:   false,
		callResult: tools.TextResult("command output"),
	}
	registry := tools.NewRegistry(mt)
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)
	tracker := cost.NewTracker("claude-sonnet-4-20250514")

	permissionCh := make(chan permissions.PermissionResult, 1)
	permissionCh <- permissions.Allow // User approves

	params := &LoopParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:     registry,
		PermCtx:      permCtx,
		CostTracker:  tracker,
		PermissionCh: permissionCh,
		WorkingDir:   "/tmp",
		ProjectRoot:  "/tmp",
	}

	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type:  "tool_use",
				ID:    "bash-id",
				Name:  "bash",
				Input: json.RawMessage(`{"command":"ls"}`),
			},
		},
	}

	results, err := executeToolUses(context.Background(), msg, params)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "command output", results[0].Content)
	assert.False(t, results[0].IsError)
	assert.Equal(t, 1, mt.callCount)
}

func TestExecuteToolUses_AutoModeNoAsk(t *testing.T) {
	mt := &mockTool{
		name:       "bash",
		readOnly:   false,
		callResult: tools.TextResult("auto output"),
	}
	registry := tools.NewRegistry(mt)
	// Auto mode should auto-approve
	permCtx := permissions.NewPermissionContext(permissions.ModeAuto, nil, nil)
	tracker := cost.NewTracker("claude-sonnet-4-20250514")

	params := &LoopParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:    registry,
		PermCtx:     permCtx,
		CostTracker: tracker,
		WorkingDir:  "/tmp",
		ProjectRoot: "/tmp",
		// No PermissionCh needed in auto mode
	}

	msg := &anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type:  "tool_use",
				ID:    "bash-id",
				Name:  "bash",
				Input: json.RawMessage(`{"command":"ls"}`),
			},
		},
	}

	results, err := executeToolUses(context.Background(), msg, params)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "auto output", results[0].Content)
	assert.False(t, results[0].IsError)
	assert.Equal(t, 1, mt.callCount)
}

func TestTextCallback(t *testing.T) {
	// Verify TextCallback is called from LoopParams context
	var captured string
	params := &LoopParams{
		TextCallback: func(text string) {
			captured += text
		},
	}
	params.TextCallback("hello ")
	params.TextCallback("world")
	assert.Equal(t, "hello world", captured)
}

func TestLoopParams_ToolUseContext(t *testing.T) {
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)
	params := &LoopParams{
		WorkingDir:  "/home/user/project",
		ProjectRoot: "/home/user/project",
		SessionID:   "test-session",
		PermCtx:     permCtx,
	}

	ctx := params.toolUseContext(context.Background())
	assert.Equal(t, "/home/user/project", ctx.WorkingDir)
	assert.Equal(t, "/home/user/project", ctx.ProjectRoot)
	assert.Equal(t, "test-session", ctx.SessionID)
	assert.NotNil(t, ctx.AbortCtx)
	assert.Equal(t, permCtx, ctx.PermCtx)
}
