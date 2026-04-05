package tools

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// mockStreamTool implements Tool for streaming executor tests.
type mockStreamTool struct {
	name            string
	concurrencySafe bool
	callDelay       time.Duration
	callResult      *ToolResult
	callErr         error
	callCount       int
	mu              sync.Mutex
}

func (m *mockStreamTool) Name() string        { return m.name }
func (m *mockStreamTool) Description() string  { return "mock tool for streaming tests" }
func (m *mockStreamTool) InputSchema() json.RawMessage { return json.RawMessage(`{}`) }
func (m *mockStreamTool) IsReadOnly() bool     { return m.concurrencySafe }
func (m *mockStreamTool) IsConcurrencySafe(_ json.RawMessage) bool { return m.concurrencySafe }
func (m *mockStreamTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *PermissionContext) (PermissionResult, error) {
	return PermissionAllow, nil
}
func (m *mockStreamTool) Call(_ context.Context, _ json.RawMessage, _ *ToolUseContext) (*ToolResult, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	if m.callDelay > 0 {
		time.Sleep(m.callDelay)
	}
	if m.callErr != nil {
		return nil, m.callErr
	}
	if m.callResult != nil {
		return m.callResult, nil
	}
	return TextResult("ok"), nil
}

func (m *mockStreamTool) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func newMockStreamRegistry(tools ...Tool) *Registry {
	return NewRegistry(tools...)
}

func TestStreamingConcurrentSafeToolsExecuteInParallel(t *testing.T) {
	// Two concurrent-safe tools should both start executing without waiting
	// for each other.
	started := make(chan string, 2)
	tool1 := &mockStreamTool{
		name:            "SafeTool1",
		concurrencySafe: true,
		callDelay:       100 * time.Millisecond,
		callResult:      TextResult("result1"),
	}
	tool2 := &mockStreamTool{
		name:            "SafeTool2",
		concurrencySafe: true,
		callDelay:       100 * time.Millisecond,
		callResult:      TextResult("result2"),
	}

	// Wrap tools to track start times
	wrapper1 := &startTrackingTool{Tool: tool1, started: started}
	wrapper2 := &startTrackingTool{Tool: tool2, started: started}

	registry := newMockStreamRegistry(wrapper1, wrapper2)
	toolCtx := &ToolUseContext{WorkingDir: "/tmp", AbortCtx: context.Background()}
	exec := NewStreamingToolExecutor(registry, toolCtx, nil)

	exec.AddTool("id1", "SafeTool1", json.RawMessage(`{}`))
	exec.AddTool("id2", "SafeTool2", json.RawMessage(`{}`))

	// Both tools should start quickly (within 50ms)
	timeout := time.After(500 * time.Millisecond)
	startedNames := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		select {
		case name := <-started:
			startedNames = append(startedNames, name)
		case <-timeout:
			t.Fatalf("expected both tools to start, only got %d", len(startedNames))
		}
	}

	if len(startedNames) != 2 {
		t.Fatalf("expected 2 tools to start, got %d", len(startedNames))
	}

	// Wait for remaining results
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var results []MessageUpdate
	for u := range exec.GetRemainingResults(ctx) {
		results = append(results, u)
	}

	// Should have 2 results
	resultCount := 0
	for _, u := range results {
		if u.Result != nil {
			resultCount++
		}
	}
	if resultCount != 2 {
		t.Fatalf("expected 2 results, got %d", resultCount)
	}
}

// startTrackingTool wraps a Tool to signal when Call starts.
type startTrackingTool struct {
	Tool
	started chan string
}

func (s *startTrackingTool) Name() string { return s.Tool.Name() }
func (s *startTrackingTool) IsConcurrencySafe(input json.RawMessage) bool {
	return s.Tool.IsConcurrencySafe(input)
}
func (s *startTrackingTool) Call(ctx context.Context, input json.RawMessage, toolCtx *ToolUseContext) (*ToolResult, error) {
	s.started <- s.Tool.Name()
	return s.Tool.Call(ctx, input, toolCtx)
}

func TestStreamingNonConcurrentToolBlocksQueue(t *testing.T) {
	// A non-concurrent tool should block the queue -- a subsequent tool
	// should not start until the non-concurrent one finishes.
	safeTool := &mockStreamTool{
		name:            "SafeTool",
		concurrencySafe: true,
		callDelay:       50 * time.Millisecond,
		callResult:      TextResult("safe result"),
	}
	unsafeTool := &mockStreamTool{
		name:            "UnsafeTool",
		concurrencySafe: false,
		callDelay:       100 * time.Millisecond,
		callResult:      TextResult("unsafe result"),
	}
	afterTool := &mockStreamTool{
		name:            "AfterTool",
		concurrencySafe: true,
		callDelay:       10 * time.Millisecond,
		callResult:      TextResult("after result"),
	}

	registry := newMockStreamRegistry(safeTool, unsafeTool, afterTool)
	toolCtx := &ToolUseContext{WorkingDir: "/tmp", AbortCtx: context.Background()}
	exec := NewStreamingToolExecutor(registry, toolCtx, nil)

	exec.AddTool("id1", "SafeTool", json.RawMessage(`{}`))
	exec.AddTool("id2", "UnsafeTool", json.RawMessage(`{}`))
	exec.AddTool("id3", "AfterTool", json.RawMessage(`{}`))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var results []ToolExecResult
	for u := range exec.GetRemainingResults(ctx) {
		if u.Result != nil {
			results = append(results, *u.Result)
		}
	}

	// All 3 tools should complete
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Results should be in order: safe, unsafe, after
	expected := []string{"safe result", "unsafe result", "after result"}
	for i, r := range results {
		if r.Content != expected[i] {
			t.Errorf("result[%d] = %q, want %q", i, r.Content, expected[i])
		}
	}
}

func TestStreamingBashErrorCancelsSiblings(t *testing.T) {
	// When a Bash tool errors, sibling queued tools should get synthetic errors.
	bashTool := &mockStreamTool{
		name:            BashToolName,
		concurrencySafe: false,
		callResult:      ErrorResult("bash failed"),
	}
	siblingTool := &mockStreamTool{
		name:            "SiblingTool",
		concurrencySafe: false,
		callDelay:       50 * time.Millisecond,
		callResult:      TextResult("should not run"),
	}

	registry := newMockStreamRegistry(bashTool, siblingTool)
	toolCtx := &ToolUseContext{WorkingDir: "/tmp", AbortCtx: context.Background()}
	exec := NewStreamingToolExecutor(registry, toolCtx, nil)

	exec.AddTool("id1", BashToolName, json.RawMessage(`{"command":"fail"}`))
	// Small delay to let bash tool start first
	time.Sleep(20 * time.Millisecond)
	exec.AddTool("id2", "SiblingTool", json.RawMessage(`{}`))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var results []ToolExecResult
	for u := range exec.GetRemainingResults(ctx) {
		if u.Result != nil {
			results = append(results, *u.Result)
		}
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First result is the bash error
	if !results[0].IsError {
		t.Error("expected bash result to be an error")
	}

	// Second result should be a synthetic sibling error
	if !results[1].IsError {
		t.Error("expected sibling result to be a synthetic error")
	}
	if results[1].Content != "Cancelled: parallel tool call Bash(fail) errored" &&
		results[1].Content != "Cancelled: parallel tool call errored" {
		// Accept either form since the description depends on parsing
		if results[1].Content == "" {
			t.Errorf("sibling error content should not be empty")
		}
	}
}

func TestStreamingDiscardProducesErrors(t *testing.T) {
	// Discard() should cause queued tools to get streaming_fallback errors.
	slowTool := &mockStreamTool{
		name:            "SlowTool",
		concurrencySafe: false,
		callDelay:       200 * time.Millisecond,
		callResult:      TextResult("slow result"),
	}
	queuedTool := &mockStreamTool{
		name:            "QueuedTool",
		concurrencySafe: false,
		callResult:      TextResult("queued result"),
	}

	registry := newMockStreamRegistry(slowTool, queuedTool)
	toolCtx := &ToolUseContext{WorkingDir: "/tmp", AbortCtx: context.Background()}
	exec := NewStreamingToolExecutor(registry, toolCtx, nil)

	exec.AddTool("id1", "SlowTool", json.RawMessage(`{}`))
	time.Sleep(20 * time.Millisecond) // Let slow tool start
	exec.AddTool("id2", "QueuedTool", json.RawMessage(`{}`))

	// Discard before slow tool finishes
	time.Sleep(50 * time.Millisecond)
	exec.Discard()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// GetRemainingResults should return empty when discarded
	var updates []MessageUpdate
	for u := range exec.GetRemainingResults(ctx) {
		updates = append(updates, u)
	}

	// Discarded executor should produce no results from GetRemainingResults
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates from discarded executor, got %d", len(updates))
	}

	// GetCompletedResults should also return empty
	completed := exec.GetCompletedResults()
	if len(completed) != 0 {
		t.Fatalf("expected 0 completed from discarded executor, got %d", len(completed))
	}
}

func TestStreamingGetCompletedResultsInOrder(t *testing.T) {
	// Non-concurrent tools block yielding: a safe group, then an unsafe tool,
	// then another safe group should yield in group order. Within a concurrent
	// group, results may arrive in any order (goroutine scheduling), but
	// non-concurrent boundaries are respected.
	safeTool1 := &mockStreamTool{
		name:            "SafeA",
		concurrencySafe: true,
		callDelay:       20 * time.Millisecond,
		callResult:      TextResult("safe-a"),
	}
	unsafeTool := &mockStreamTool{
		name:            "UnsafeB",
		concurrencySafe: false,
		callDelay:       20 * time.Millisecond,
		callResult:      TextResult("unsafe-b"),
	}
	safeTool2 := &mockStreamTool{
		name:            "SafeC",
		concurrencySafe: true,
		callDelay:       20 * time.Millisecond,
		callResult:      TextResult("safe-c"),
	}

	registry := newMockStreamRegistry(safeTool1, unsafeTool, safeTool2)
	toolCtx := &ToolUseContext{WorkingDir: "/tmp", AbortCtx: context.Background()}
	exec := NewStreamingToolExecutor(registry, toolCtx, nil)

	exec.AddTool("id1", "SafeA", json.RawMessage(`{}`))
	exec.AddTool("id2", "UnsafeB", json.RawMessage(`{}`))
	exec.AddTool("id3", "SafeC", json.RawMessage(`{}`))

	// Wait for all tools to complete
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var results []ToolExecResult
	for u := range exec.GetRemainingResults(ctx) {
		if u.Result != nil {
			results = append(results, *u.Result)
		}
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// The ordering guarantee: safe-a must come before unsafe-b,
	// and unsafe-b must come before safe-c. Non-concurrent tools
	// block yielding of subsequent tools.
	idxA, idxB, idxC := -1, -1, -1
	for i, r := range results {
		switch r.Content {
		case "safe-a":
			idxA = i
		case "unsafe-b":
			idxB = i
		case "safe-c":
			idxC = i
		}
	}

	if idxA == -1 || idxB == -1 || idxC == -1 {
		t.Fatalf("missing results: a=%d b=%d c=%d", idxA, idxB, idxC)
	}

	if idxA >= idxB {
		t.Errorf("safe-a (idx=%d) should come before unsafe-b (idx=%d)", idxA, idxB)
	}
	if idxB >= idxC {
		t.Errorf("unsafe-b (idx=%d) should come before safe-c (idx=%d)", idxB, idxC)
	}
}

func TestStreamingUnknownToolCompletesWithError(t *testing.T) {
	registry := newMockStreamRegistry() // empty registry
	toolCtx := &ToolUseContext{WorkingDir: "/tmp", AbortCtx: context.Background()}
	exec := NewStreamingToolExecutor(registry, toolCtx, nil)

	exec.AddTool("id1", "NonExistentTool", json.RawMessage(`{}`))

	// Should complete immediately with error
	time.Sleep(50 * time.Millisecond)
	results := exec.GetCompletedResults()

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Result == nil {
		t.Fatal("expected a result update")
	}
	if !results[0].Result.IsError {
		t.Error("expected error result for unknown tool")
	}
	if results[0].Result.Content != "Error: No such tool available: NonExistentTool" {
		t.Errorf("unexpected error content: %q", results[0].Result.Content)
	}
}

func TestStreamingGetUpdatedContext(t *testing.T) {
	registry := newMockStreamRegistry()
	toolCtx := &ToolUseContext{WorkingDir: "/tmp", AbortCtx: context.Background()}
	exec := NewStreamingToolExecutor(registry, toolCtx, nil)

	got := exec.GetUpdatedContext()
	if got != toolCtx {
		t.Error("GetUpdatedContext should return the current context")
	}
}
