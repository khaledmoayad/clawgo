package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orchestrationMockTool is a mock tool for orchestration tests.
type orchestrationMockTool struct {
	name            string
	concurrencySafe bool
	callFn          func(ctx context.Context, input json.RawMessage, toolCtx *ToolUseContext) (*ToolResult, error)
}

func (m *orchestrationMockTool) Name() string                { return m.name }
func (m *orchestrationMockTool) Description() string          { return "mock" }
func (m *orchestrationMockTool) InputSchema() json.RawMessage { return json.RawMessage(`{}`) }
func (m *orchestrationMockTool) IsReadOnly() bool             { return m.concurrencySafe }

func (m *orchestrationMockTool) IsConcurrencySafe(_ json.RawMessage) bool {
	return m.concurrencySafe
}

func (m *orchestrationMockTool) Call(ctx context.Context, input json.RawMessage, toolCtx *ToolUseContext) (*ToolResult, error) {
	if m.callFn != nil {
		return m.callFn(ctx, input, toolCtx)
	}
	return TextResult(fmt.Sprintf("result from %s", m.name)), nil
}

func (m *orchestrationMockTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *PermissionContext) (PermissionResult, error) {
	return PermissionAllow, nil
}

// --- PartitionToolCalls Tests ---

func TestPartitionToolCalls_AllSafe(t *testing.T) {
	readTool := &orchestrationMockTool{name: "read", concurrencySafe: true}
	grepTool := &orchestrationMockTool{name: "grep", concurrencySafe: true}
	reg := NewRegistry(readTool, grepTool)

	entries := []ToolCallEntry{
		{ID: "1", Name: "read", Input: json.RawMessage(`{}`)},
		{ID: "2", Name: "grep", Input: json.RawMessage(`{}`)},
	}

	batches := PartitionToolCalls(entries, reg)
	require.Len(t, batches, 1)
	assert.True(t, batches[0].ConcurrencySafe)
	assert.Len(t, batches[0].Entries, 2)
}

func TestPartitionToolCalls_AllUnsafe(t *testing.T) {
	bashTool := &orchestrationMockTool{name: "bash", concurrencySafe: false}
	writeTool := &orchestrationMockTool{name: "write", concurrencySafe: false}
	reg := NewRegistry(bashTool, writeTool)

	entries := []ToolCallEntry{
		{ID: "1", Name: "bash", Input: json.RawMessage(`{}`)},
		{ID: "2", Name: "write", Input: json.RawMessage(`{}`)},
	}

	batches := PartitionToolCalls(entries, reg)
	// Each unsafe tool gets its own batch
	require.Len(t, batches, 2)
	assert.False(t, batches[0].ConcurrencySafe)
	assert.Len(t, batches[0].Entries, 1)
	assert.False(t, batches[1].ConcurrencySafe)
	assert.Len(t, batches[1].Entries, 1)
}

func TestPartitionToolCalls_MixedSequence(t *testing.T) {
	readTool := &orchestrationMockTool{name: "read", concurrencySafe: true}
	grepTool := &orchestrationMockTool{name: "grep", concurrencySafe: true}
	bashTool := &orchestrationMockTool{name: "bash", concurrencySafe: false}
	globTool := &orchestrationMockTool{name: "glob", concurrencySafe: true}
	reg := NewRegistry(readTool, grepTool, bashTool, globTool)

	// [safe, safe, unsafe, safe] -> 3 batches
	entries := []ToolCallEntry{
		{ID: "1", Name: "read", Input: json.RawMessage(`{}`)},
		{ID: "2", Name: "grep", Input: json.RawMessage(`{}`)},
		{ID: "3", Name: "bash", Input: json.RawMessage(`{}`)},
		{ID: "4", Name: "glob", Input: json.RawMessage(`{}`)},
	}

	batches := PartitionToolCalls(entries, reg)
	require.Len(t, batches, 3)
	// Batch 1: safe group [read, grep]
	assert.True(t, batches[0].ConcurrencySafe)
	assert.Len(t, batches[0].Entries, 2)
	// Batch 2: unsafe [bash]
	assert.False(t, batches[1].ConcurrencySafe)
	assert.Len(t, batches[1].Entries, 1)
	// Batch 3: safe group [glob]
	assert.True(t, batches[2].ConcurrencySafe)
	assert.Len(t, batches[2].Entries, 1)
}

func TestPartitionToolCalls_UnknownTool(t *testing.T) {
	reg := NewRegistry() // empty registry

	entries := []ToolCallEntry{
		{ID: "1", Name: "nonexistent", Input: json.RawMessage(`{}`)},
	}

	batches := PartitionToolCalls(entries, reg)
	require.Len(t, batches, 1)
	// Unknown tools are treated as non-safe
	assert.False(t, batches[0].ConcurrencySafe)
	assert.Len(t, batches[0].Entries, 1)
}

// --- RunConcurrentBatch Tests ---

func TestRunConcurrentBatch_ExecutesAllTools(t *testing.T) {
	readTool := &orchestrationMockTool{name: "read", concurrencySafe: true}
	grepTool := &orchestrationMockTool{name: "grep", concurrencySafe: true}
	reg := NewRegistry(readTool, grepTool)

	batch := Batch{
		ConcurrencySafe: true,
		Entries: []ToolCallEntry{
			{ID: "id-1", Name: "read", Input: json.RawMessage(`{}`)},
			{ID: "id-2", Name: "grep", Input: json.RawMessage(`{}`)},
		},
	}

	toolCtx := &ToolUseContext{WorkingDir: "/tmp"}
	permFn := func(name string, input json.RawMessage, tool Tool) (PermissionResult, error) {
		return PermissionAllow, nil
	}

	results, err := RunConcurrentBatch(context.Background(), batch, reg, toolCtx, permFn)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "id-1", results[0].ToolUseID)
	assert.Equal(t, "id-2", results[1].ToolUseID)
	assert.False(t, results[0].IsError)
	assert.False(t, results[1].IsError)
}

func TestRunConcurrentBatch_PreservesOrder(t *testing.T) {
	// Create tools with varying delays to ensure order is preserved
	var counter atomic.Int32
	tools := make([]*orchestrationMockTool, 5)
	for i := range tools {
		idx := i
		tools[i] = &orchestrationMockTool{
			name:            fmt.Sprintf("tool_%d", i),
			concurrencySafe: true,
			callFn: func(_ context.Context, _ json.RawMessage, _ *ToolUseContext) (*ToolResult, error) {
				counter.Add(1)
				return TextResult(fmt.Sprintf("result_%d", idx)), nil
			},
		}
	}

	toolInterfaces := make([]Tool, len(tools))
	for i, t := range tools {
		toolInterfaces[i] = t
	}
	reg := NewRegistry(toolInterfaces...)

	entries := make([]ToolCallEntry, len(tools))
	for i := range tools {
		entries[i] = ToolCallEntry{
			ID:    fmt.Sprintf("id-%d", i),
			Name:  fmt.Sprintf("tool_%d", i),
			Input: json.RawMessage(`{}`),
		}
	}

	batch := Batch{ConcurrencySafe: true, Entries: entries}
	toolCtx := &ToolUseContext{WorkingDir: "/tmp"}
	permFn := func(name string, input json.RawMessage, tool Tool) (PermissionResult, error) {
		return PermissionAllow, nil
	}

	results, err := RunConcurrentBatch(context.Background(), batch, reg, toolCtx, permFn)
	require.NoError(t, err)
	require.Len(t, results, 5)

	// Verify order is preserved regardless of execution order
	for i, r := range results {
		assert.Equal(t, fmt.Sprintf("id-%d", i), r.ToolUseID)
		assert.Contains(t, r.Content, fmt.Sprintf("result_%d", i))
	}
	assert.Equal(t, int32(5), counter.Load(), "all tools should have executed")
}

func TestRunConcurrentBatch_UnknownTool(t *testing.T) {
	reg := NewRegistry() // empty

	batch := Batch{
		ConcurrencySafe: true,
		Entries: []ToolCallEntry{
			{ID: "id-1", Name: "nonexistent", Input: json.RawMessage(`{}`)},
		},
	}

	toolCtx := &ToolUseContext{WorkingDir: "/tmp"}
	permFn := func(name string, input json.RawMessage, tool Tool) (PermissionResult, error) {
		return PermissionAllow, nil
	}

	results, err := RunConcurrentBatch(context.Background(), batch, reg, toolCtx, permFn)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, results[0].IsError)
	assert.Contains(t, results[0].Content, "Unknown tool")
}

// --- RunSerialBatch Tests ---

func TestRunSerialBatch_ExecutesSequentially(t *testing.T) {
	var order []string
	bashTool := &orchestrationMockTool{
		name:            "bash",
		concurrencySafe: false,
		callFn: func(_ context.Context, _ json.RawMessage, _ *ToolUseContext) (*ToolResult, error) {
			order = append(order, "bash")
			return TextResult("bash result"), nil
		},
	}
	writeTool := &orchestrationMockTool{
		name:            "write",
		concurrencySafe: false,
		callFn: func(_ context.Context, _ json.RawMessage, _ *ToolUseContext) (*ToolResult, error) {
			order = append(order, "write")
			return TextResult("write result"), nil
		},
	}
	reg := NewRegistry(bashTool, writeTool)

	batch := Batch{
		ConcurrencySafe: false,
		Entries: []ToolCallEntry{
			{ID: "id-1", Name: "bash", Input: json.RawMessage(`{}`)},
			{ID: "id-2", Name: "write", Input: json.RawMessage(`{}`)},
		},
	}

	toolCtx := &ToolUseContext{WorkingDir: "/tmp"}
	permFn := func(name string, input json.RawMessage, tool Tool) (PermissionResult, error) {
		return PermissionAllow, nil
	}

	results, err := RunSerialBatch(context.Background(), batch, reg, toolCtx, permFn)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, []string{"bash", "write"}, order)
	assert.Equal(t, "id-1", results[0].ToolUseID)
	assert.Equal(t, "id-2", results[1].ToolUseID)
}

// --- ExecuteBatches Tests ---

func TestExecuteBatches_ProcessesAllBatches(t *testing.T) {
	readTool := &orchestrationMockTool{name: "read", concurrencySafe: true}
	bashTool := &orchestrationMockTool{name: "bash", concurrencySafe: false}
	grepTool := &orchestrationMockTool{name: "grep", concurrencySafe: true}
	reg := NewRegistry(readTool, bashTool, grepTool)

	entries := []ToolCallEntry{
		{ID: "1", Name: "read", Input: json.RawMessage(`{}`)},
		{ID: "2", Name: "bash", Input: json.RawMessage(`{}`)},
		{ID: "3", Name: "grep", Input: json.RawMessage(`{}`)},
	}

	batches := PartitionToolCalls(entries, reg)
	toolCtx := &ToolUseContext{WorkingDir: "/tmp"}
	permFn := func(name string, input json.RawMessage, tool Tool) (PermissionResult, error) {
		return PermissionAllow, nil
	}

	results, err := ExecuteBatches(context.Background(), batches, reg, toolCtx, permFn)
	require.NoError(t, err)
	require.Len(t, results, 3)
	assert.Equal(t, "1", results[0].ToolUseID)
	assert.Equal(t, "2", results[1].ToolUseID)
	assert.Equal(t, "3", results[2].ToolUseID)
}

func TestExecuteBatches_AppliesContextModifier(t *testing.T) {
	modTool := &orchestrationMockTool{
		name:            "modifier",
		concurrencySafe: false,
		callFn: func(_ context.Context, _ json.RawMessage, _ *ToolUseContext) (*ToolResult, error) {
			return &ToolResult{
				Content: []ContentBlock{{Type: "text", Text: "modified"}},
				ContextModifier: func(ctx *ToolUseContext) {
					ctx.WorkingDir = "/modified"
				},
			}, nil
		},
	}
	checkTool := &orchestrationMockTool{
		name:            "check",
		concurrencySafe: false,
		callFn: func(_ context.Context, _ json.RawMessage, toolCtx *ToolUseContext) (*ToolResult, error) {
			return TextResult(toolCtx.WorkingDir), nil
		},
	}
	reg := NewRegistry(modTool, checkTool)

	entries := []ToolCallEntry{
		{ID: "1", Name: "modifier", Input: json.RawMessage(`{}`)},
		{ID: "2", Name: "check", Input: json.RawMessage(`{}`)},
	}

	batches := PartitionToolCalls(entries, reg)
	toolCtx := &ToolUseContext{WorkingDir: "/original"}
	permFn := func(name string, input json.RawMessage, tool Tool) (PermissionResult, error) {
		return PermissionAllow, nil
	}

	results, err := ExecuteBatches(context.Background(), batches, reg, toolCtx, permFn)
	require.NoError(t, err)
	require.Len(t, results, 2)
	// The second tool should see the modified context
	assert.Equal(t, "/modified", results[1].Content)
}

func TestRunConcurrentBatch_RespectsMaxConcurrency(t *testing.T) {
	var maxConcurrent atomic.Int32
	var current atomic.Int32

	// Create 15 tools to exceed the limit of 10
	toolList := make([]Tool, 15)
	entries := make([]ToolCallEntry, 15)
	for i := range toolList {
		idx := i
		toolList[i] = &orchestrationMockTool{
			name:            fmt.Sprintf("tool_%d", idx),
			concurrencySafe: true,
			callFn: func(_ context.Context, _ json.RawMessage, _ *ToolUseContext) (*ToolResult, error) {
				c := current.Add(1)
				// Track max concurrent
				for {
					old := maxConcurrent.Load()
					if c <= old || maxConcurrent.CompareAndSwap(old, c) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond) // Simulate work
				current.Add(-1)
				return TextResult(fmt.Sprintf("result_%d", idx)), nil
			},
		}
		entries[i] = ToolCallEntry{
			ID:    fmt.Sprintf("id-%d", idx),
			Name:  fmt.Sprintf("tool_%d", idx),
			Input: json.RawMessage(`{}`),
		}
	}

	reg := NewRegistry(toolList...)
	batch := Batch{ConcurrencySafe: true, Entries: entries}
	toolCtx := &ToolUseContext{WorkingDir: "/tmp"}
	permFn := func(name string, input json.RawMessage, tool Tool) (PermissionResult, error) {
		return PermissionAllow, nil
	}

	results, err := RunConcurrentBatch(context.Background(), batch, reg, toolCtx, permFn)
	require.NoError(t, err)
	require.Len(t, results, 15)
	// Max concurrent should not exceed 10
	assert.LessOrEqual(t, maxConcurrent.Load(), int32(10), "max concurrency should not exceed 10")
}

// --- StreamingExecutor Tests ---

func TestStreamingExecutor_SendsEvents(t *testing.T) {
	tool := &orchestrationMockTool{
		name:            "test_tool",
		concurrencySafe: true,
		callFn: func(_ context.Context, _ json.RawMessage, _ *ToolUseContext) (*ToolResult, error) {
			return TextResult("hello streaming"), nil
		},
	}
	reg := NewRegistry(tool)
	toolCtx := &ToolUseContext{WorkingDir: "/tmp"}

	executor := &StreamingExecutor{}
	entry := ToolCallEntry{ID: "id-1", Name: "test_tool", Input: json.RawMessage(`{}`)}

	ch := executor.Execute(context.Background(), entry, reg, toolCtx)

	var events []StreamEvent
	for event := range ch {
		events = append(events, event)
	}

	// Should have at least a complete event
	require.NotEmpty(t, events)
	lastEvent := events[len(events)-1]
	assert.True(t, lastEvent.Done)
	assert.Equal(t, "complete", lastEvent.Type)
	assert.Contains(t, lastEvent.Text, "hello streaming")
}

func TestStreamingExecutor_UnknownTool(t *testing.T) {
	reg := NewRegistry() // empty registry
	toolCtx := &ToolUseContext{WorkingDir: "/tmp"}

	executor := &StreamingExecutor{}
	entry := ToolCallEntry{ID: "id-1", Name: "nonexistent", Input: json.RawMessage(`{}`)}

	ch := executor.Execute(context.Background(), entry, reg, toolCtx)

	var events []StreamEvent
	for event := range ch {
		events = append(events, event)
	}

	require.NotEmpty(t, events)
	lastEvent := events[len(events)-1]
	assert.Equal(t, "error", lastEvent.Type)
	assert.Contains(t, lastEvent.Text, "Unknown tool")
}
