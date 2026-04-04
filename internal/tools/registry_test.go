package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTool is a minimal Tool implementation for testing.
type mockTool struct {
	name             string
	description      string
	schema           json.RawMessage
	readOnly         bool
	concurrencySafe  bool
}

func (m *mockTool) Name() string                { return m.name }
func (m *mockTool) Description() string          { return m.description }
func (m *mockTool) InputSchema() json.RawMessage { return m.schema }
func (m *mockTool) IsReadOnly() bool             { return m.readOnly }

func (m *mockTool) IsConcurrencySafe(_ json.RawMessage) bool { return m.concurrencySafe }

func (m *mockTool) Call(_ context.Context, _ json.RawMessage, _ *ToolUseContext) (*ToolResult, error) {
	return TextResult("ok"), nil
}

func (m *mockTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *PermissionContext) (PermissionResult, error) {
	return PermissionAllow, nil
}

func newMockTool(name, desc string) *mockTool {
	schema := json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`)
	return &mockTool{name: name, description: desc, schema: schema}
}

// --- Registry Tests ---

func TestNewRegistry_Get(t *testing.T) {
	mock := newMockTool("mock_tool", "A mock tool")
	reg := NewRegistry(mock)

	// Found
	tool, ok := reg.Get("mock_tool")
	assert.True(t, ok)
	assert.Equal(t, "mock_tool", tool.Name())

	// Not found
	tool, ok = reg.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, tool)
}

func TestRegistry_All(t *testing.T) {
	m1 := newMockTool("tool_a", "Tool A")
	m2 := newMockTool("tool_b", "Tool B")
	m3 := newMockTool("tool_c", "Tool C")
	reg := NewRegistry(m1, m2, m3)

	all := reg.All()
	require.Len(t, all, 3)
	assert.Equal(t, "tool_a", all[0].Name())
	assert.Equal(t, "tool_b", all[1].Name())
	assert.Equal(t, "tool_c", all[2].Name())
}

func TestRegistry_ToolDefinitions(t *testing.T) {
	mock := newMockTool("mock_tool", "A mock tool")
	reg := NewRegistry(mock)

	defs := reg.ToolDefinitions()
	require.Len(t, defs, 1)
	assert.Equal(t, "mock_tool", defs[0].Name)
	assert.Equal(t, "A mock tool", defs[0].Description)
	assert.JSONEq(t, `{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`, string(defs[0].InputSchema))
}

func TestRegistry_Names(t *testing.T) {
	m1 := newMockTool("alpha", "Alpha")
	m2 := newMockTool("beta", "Beta")
	m3 := newMockTool("gamma", "Gamma")
	reg := NewRegistry(m1, m2, m3)

	names := reg.Names()
	assert.Equal(t, []string{"alpha", "beta", "gamma"}, names)
}

// --- Validation Tests ---

type testInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
}

func (t *testInput) Validate() error {
	if t.Command == "" {
		return fmt.Errorf("required field %q is missing or empty", "command")
	}
	return nil
}

func TestValidateInput_Valid(t *testing.T) {
	input := json.RawMessage(`{"command":"ls"}`)
	var target testInput
	err := ValidateInput(input, &target)
	assert.NoError(t, err)
	assert.Equal(t, "ls", target.Command)
}

func TestValidateInput_InvalidJSON(t *testing.T) {
	input := json.RawMessage(`not json`)
	var target testInput
	err := ValidateInput(input, &target)
	assert.Error(t, err)
}

func TestValidateInput_MissingRequiredField(t *testing.T) {
	input := json.RawMessage(`{"timeout":10}`)
	var target testInput
	err := ValidateInput(input, &target)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "command")
}

func TestRequireString_Present(t *testing.T) {
	data := map[string]any{"key": "value"}
	val, err := RequireString(data, "key")
	assert.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestRequireString_Missing(t *testing.T) {
	data := map[string]any{}
	_, err := RequireString(data, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required field")
}

func TestRequireString_Empty(t *testing.T) {
	data := map[string]any{"key": ""}
	_, err := RequireString(data, "key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required field")
}

func TestOptionalInt_Present(t *testing.T) {
	data := map[string]any{"limit": float64(10)} // JSON numbers are float64
	val := OptionalInt(data, "limit", 100)
	assert.Equal(t, 10, val)
}

func TestOptionalInt_Missing(t *testing.T) {
	data := map[string]any{}
	val := OptionalInt(data, "limit", 100)
	assert.Equal(t, 100, val)
}

func TestOptionalString_Present(t *testing.T) {
	data := map[string]any{"mode": "fast"}
	val := OptionalString(data, "mode", "normal")
	assert.Equal(t, "fast", val)
}

func TestOptionalString_Missing(t *testing.T) {
	data := map[string]any{}
	val := OptionalString(data, "mode", "normal")
	assert.Equal(t, "normal", val)
}

func TestOptionalBool_Present(t *testing.T) {
	data := map[string]any{"verbose": true}
	val := OptionalBool(data, "verbose", false)
	assert.True(t, val)
}

func TestOptionalBool_Missing(t *testing.T) {
	data := map[string]any{}
	val := OptionalBool(data, "verbose", false)
	assert.False(t, val)
}

func TestParseRawInput_Valid(t *testing.T) {
	input := json.RawMessage(`{"command":"ls","limit":10}`)
	data, err := ParseRawInput(input)
	assert.NoError(t, err)
	assert.Equal(t, "ls", data["command"])
	assert.Equal(t, float64(10), data["limit"])
}

func TestParseRawInput_Invalid(t *testing.T) {
	input := json.RawMessage(`not json`)
	_, err := ParseRawInput(input)
	assert.Error(t, err)
}

// --- Type Helper Tests ---

func TestTextResult(t *testing.T) {
	r := TextResult("hello")
	require.Len(t, r.Content, 1)
	assert.Equal(t, "text", r.Content[0].Type)
	assert.Equal(t, "hello", r.Content[0].Text)
	assert.False(t, r.IsError)
}

func TestErrorResult(t *testing.T) {
	r := ErrorResult("something failed")
	require.Len(t, r.Content, 1)
	assert.Equal(t, "text", r.Content[0].Type)
	assert.Equal(t, "something failed", r.Content[0].Text)
	assert.True(t, r.IsError)
}

// --- IsConcurrencySafe Interface Tests ---

func TestToolInterface_RequiresIsConcurrencySafe(t *testing.T) {
	// Verify the interface includes IsConcurrencySafe by creating a mock that implements it
	var tool Tool = &mockTool{name: "test", concurrencySafe: true}
	assert.True(t, tool.IsConcurrencySafe(nil))

	tool = &mockTool{name: "test2", concurrencySafe: false}
	assert.False(t, tool.IsConcurrencySafe(nil))
}

// --- ContextModifier Tests ---

func TestToolResult_ContextModifier_NilByDefault(t *testing.T) {
	r := TextResult("hello")
	assert.Nil(t, r.ContextModifier)
}

func TestToolResult_ContextModifier_Applied(t *testing.T) {
	r := &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: "test"}},
		ContextModifier: func(ctx *ToolUseContext) {
			ctx.WorkingDir = "/new/dir"
		},
	}

	toolCtx := &ToolUseContext{WorkingDir: "/old/dir"}
	r.ContextModifier(toolCtx)
	assert.Equal(t, "/new/dir", toolCtx.WorkingDir)
}

// --- StreamEvent Tests ---

func TestStreamEvent_TypeExists(t *testing.T) {
	events := []StreamEvent{
		{Type: "text", Text: "hello"},
		{Type: "progress", Text: "50%"},
		{Type: "complete", Text: "done", Done: true},
		{Type: "error", Text: "failed"},
	}

	assert.Equal(t, "text", events[0].Type)
	assert.Equal(t, "hello", events[0].Text)
	assert.False(t, events[0].Done)
	assert.True(t, events[2].Done)
}
