package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// BashToolName is the name of the Bash tool. Only Bash tool errors cancel
// sibling tools -- Bash commands often have implicit dependency chains
// (e.g., mkdir fails -> subsequent commands are pointless). Read/WebFetch/etc.
// are independent; one failure shouldn't nuke the rest.
const BashToolName = "Bash"

// ToolStatus tracks the lifecycle state of a tool within the StreamingToolExecutor.
type ToolStatus string

const (
	StatusQueued    ToolStatus = "queued"
	StatusExecuting ToolStatus = "executing"
	StatusCompleted ToolStatus = "completed"
	StatusYielded   ToolStatus = "yielded"
)

// ToolExecResult holds the outcome of a single tool execution.
type ToolExecResult struct {
	ToolUseID string
	Content   string
	IsError   bool
}

// ProgressEvent represents an incremental progress update from a tool.
type ProgressEvent struct {
	ToolName string
	Text     string
}

// MessageUpdate is the unit of output from the StreamingToolExecutor.
// Exactly one of Result, Progress, or NewContext is non-nil per update.
type MessageUpdate struct {
	Result     *ToolExecResult
	Progress   *ProgressEvent
	NewContext *ToolUseContext
}

// TrackedTool holds the state for a single tool queued in the StreamingToolExecutor.
type TrackedTool struct {
	ID                string
	Name              string
	Input             json.RawMessage
	Status            ToolStatus
	IsConcurrencySafe bool
	Results           []ToolExecResult
	PendingProgress   []ProgressEvent
	contextModifiers  []func(*ToolUseContext)

	// done is closed when tool execution completes.
	done chan struct{}
}

// StreamingToolExecutor executes tools as they stream in with concurrency control.
// Concurrent-safe tools can execute in parallel with other concurrent-safe tools.
// Non-concurrent tools must execute alone (exclusive access).
// Results are buffered and emitted in the order tools were received.
//
// This matches Claude Code's StreamingToolExecutor class behavior.
type StreamingToolExecutor struct {
	tools            []*TrackedTool
	toolCtx          *ToolUseContext
	hasErrored       bool
	erroredToolDesc  string
	discarded        bool
	registry         *Registry
	permissionFn     PermissionFn
	mu               sync.Mutex
	progressCh       chan struct{} // signaled when new progress is available
}

// NewStreamingToolExecutor creates a new StreamingToolExecutor.
func NewStreamingToolExecutor(registry *Registry, toolCtx *ToolUseContext, permFn PermissionFn) *StreamingToolExecutor {
	return &StreamingToolExecutor{
		registry:     registry,
		toolCtx:      toolCtx,
		permissionFn: permFn,
		progressCh:   make(chan struct{}, 1),
	}
}

// AddTool queues a tool for execution. It will start executing immediately
// if concurrency conditions allow (e.g., no non-concurrent tool is running).
func (s *StreamingToolExecutor) AddTool(id, name string, input json.RawMessage) {
	s.mu.Lock()

	// Determine concurrency safety from registry
	isSafe := false
	if tool, ok := s.registry.Get(name); ok {
		isSafe = tool.IsConcurrencySafe(input)
	}

	tracked := &TrackedTool{
		ID:                id,
		Name:              name,
		Input:             input,
		Status:            StatusQueued,
		IsConcurrencySafe: isSafe,
		done:              make(chan struct{}),
	}

	// If tool not found in registry, mark it as completed with error immediately
	if _, ok := s.registry.Get(name); !ok {
		tracked.Status = StatusCompleted
		tracked.Results = []ToolExecResult{{
			ToolUseID: id,
			Content:   fmt.Sprintf("Error: No such tool available: %s", name),
			IsError:   true,
		}}
		close(tracked.done)
		s.tools = append(s.tools, tracked)
		s.mu.Unlock()
		return
	}

	s.tools = append(s.tools, tracked)
	s.mu.Unlock()

	go s.processQueue()
}

// Discard marks the executor as discarded (e.g., on streaming fallback).
// Queued tools won't start, and in-progress tools will receive synthetic errors.
func (s *StreamingToolExecutor) Discard() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discarded = true
}

// canExecuteTool checks if a tool with the given concurrency safety can start
// executing based on current executor state. Returns true if no tools are
// executing, or if both the candidate and all executing tools are concurrent-safe.
func (s *StreamingToolExecutor) canExecuteTool(isConcurrencySafe bool) bool {
	executingCount := 0
	allExecutingSafe := true
	for _, t := range s.tools {
		if t.Status == StatusExecuting {
			executingCount++
			if !t.IsConcurrencySafe {
				allExecutingSafe = false
			}
		}
	}
	return executingCount == 0 ||
		(isConcurrencySafe && allExecutingSafe)
}

// getAbortReason determines why a tool should be cancelled, if at all.
func (s *StreamingToolExecutor) getAbortReason() string {
	if s.discarded {
		return "streaming_fallback"
	}
	if s.hasErrored {
		return "sibling_error"
	}
	return ""
}

// createSyntheticError produces an error result for a tool that was cancelled.
func (s *StreamingToolExecutor) createSyntheticError(toolUseID, reason string) ToolExecResult {
	var msg string
	switch reason {
	case "streaming_fallback":
		msg = "Error: Streaming fallback - tool execution discarded"
	case "sibling_error":
		desc := s.erroredToolDesc
		if desc != "" {
			msg = fmt.Sprintf("Cancelled: parallel tool call %s errored", desc)
		} else {
			msg = "Cancelled: parallel tool call errored"
		}
	default:
		msg = "Cancelled: unknown reason"
	}
	return ToolExecResult{
		ToolUseID: toolUseID,
		Content:   msg,
		IsError:   true,
	}
}

// processQueue iterates through queued tools and starts execution when
// concurrency conditions allow. Non-concurrent tools block the queue.
func (s *StreamingToolExecutor) processQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, tool := range s.tools {
		if tool.Status != StatusQueued {
			continue
		}

		if s.canExecuteTool(tool.IsConcurrencySafe) {
			tool.Status = StatusExecuting
			go s.executeTool(tool)
		} else {
			// Can't execute this tool yet. If it's non-concurrent-safe,
			// we must stop processing to maintain order.
			if !tool.IsConcurrencySafe {
				break
			}
		}
	}
}

// executeTool runs a single tool, collecting results and handling errors.
func (s *StreamingToolExecutor) executeTool(tool *TrackedTool) {
	defer func() {
		close(tool.done)
		// Re-process queue -- completion may unblock waiting tools
		s.processQueue()
	}()

	// Check for abort before executing
	s.mu.Lock()
	reason := s.getAbortReason()
	s.mu.Unlock()

	if reason != "" {
		s.mu.Lock()
		tool.Results = []ToolExecResult{s.createSyntheticError(tool.ID, reason)}
		tool.Status = StatusCompleted
		s.mu.Unlock()
		return
	}

	// Build a ToolCallEntry and use executeSingleTool from orchestration
	entry := ToolCallEntry{
		ID:    tool.ID,
		Name:  tool.Name,
		Input: tool.Input,
	}

	// Use the tool context's AbortCtx for cancellation
	ctx := context.Background()
	if s.toolCtx != nil && s.toolCtx.AbortCtx != nil {
		ctx = s.toolCtx.AbortCtx
	}

	result := executeSingleTool(ctx, entry, s.registry, s.toolCtx, s.permissionFn)

	s.mu.Lock()
	defer s.mu.Unlock()

	execResult := ToolExecResult{
		ToolUseID: result.ToolUseID,
		Content:   result.Content,
		IsError:   result.IsError,
	}

	// If this is a Bash tool error, set hasErrored to cancel siblings
	if result.IsError && tool.Name == BashToolName {
		s.hasErrored = true
		s.erroredToolDesc = toolDescription(tool)
	}

	tool.Results = []ToolExecResult{execResult}
	tool.Status = StatusCompleted

	// Signal progress channel to wake up any waiting consumers
	select {
	case s.progressCh <- struct{}{}:
	default:
	}
}

// toolDescription produces a concise description for error messages.
func toolDescription(tool *TrackedTool) string {
	// Try to extract a summary from the input
	var inputMap map[string]interface{}
	if err := json.Unmarshal(tool.Input, &inputMap); err == nil {
		for _, key := range []string{"command", "file_path", "pattern"} {
			if v, ok := inputMap[key]; ok {
				if s, ok := v.(string); ok && s != "" {
					if len(s) > 40 {
						s = s[:40] + "\u2026"
					}
					return fmt.Sprintf("%s(%s)", tool.Name, s)
				}
			}
		}
	}
	return tool.Name
}

// GetCompletedResults returns completed results that haven't been yielded yet
// (non-blocking). Maintains order: non-concurrent tools block yielding until
// complete. Also yields any pending progress messages immediately.
func (s *StreamingToolExecutor) GetCompletedResults() []MessageUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.discarded {
		return nil
	}

	var updates []MessageUpdate

	for _, tool := range s.tools {
		// Always yield pending progress messages immediately
		for _, p := range tool.PendingProgress {
			updates = append(updates, MessageUpdate{Progress: &p})
		}
		tool.PendingProgress = nil

		if tool.Status == StatusYielded {
			continue
		}

		if tool.Status == StatusCompleted && tool.Results != nil {
			tool.Status = StatusYielded
			for _, r := range tool.Results {
				r := r // capture
				updates = append(updates, MessageUpdate{
					Result:     &r,
					NewContext: s.toolCtx,
				})
			}
		} else if tool.Status == StatusExecuting && !tool.IsConcurrencySafe {
			// Non-concurrent tool is still executing -- stop yielding
			// to maintain ordering guarantees.
			break
		}
	}

	return updates
}

// GetRemainingResults returns a channel that yields results as tools complete.
// The channel is closed when all tools have finished and yielded.
func (s *StreamingToolExecutor) GetRemainingResults(ctx context.Context) <-chan MessageUpdate {
	ch := make(chan MessageUpdate, 16)

	go func() {
		defer close(ch)

		for {
			if ctx.Err() != nil {
				return
			}

			// Yield any completed results
			updates := s.GetCompletedResults()
			for _, u := range updates {
				select {
				case ch <- u:
				case <-ctx.Done():
					return
				}
			}

			// Check if all tools are done
			s.mu.Lock()
			allDone := true
			var waitChans []<-chan struct{}
			for _, t := range s.tools {
				if t.Status != StatusYielded {
					allDone = false
				}
				if t.Status == StatusExecuting || t.Status == StatusQueued {
					waitChans = append(waitChans, t.done)
				}
			}
			discarded := s.discarded
			s.mu.Unlock()

			if allDone || discarded {
				// Yield any final results
				finals := s.GetCompletedResults()
				for _, u := range finals {
					select {
					case ch <- u:
					case <-ctx.Done():
						return
					}
				}
				return
			}

			// Wait for any tool to complete or progress to arrive
			if len(waitChans) > 0 {
				select {
				case <-waitChans[0]:
					// A tool completed, loop again to yield
				case <-s.progressCh:
					// Progress available
				case <-ctx.Done():
					return
				}
			} else {
				// No executing tools, but not all yielded -- try yielding again
				// This handles the case where tools completed between our check
				// and the wait.
				continue
			}
		}
	}()

	return ch
}

// GetUpdatedContext returns the current tool use context, which may have been
// modified by context modifiers from completed non-concurrent tools.
func (s *StreamingToolExecutor) GetUpdatedContext() *ToolUseContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.toolCtx
}

// HasUnfinishedTools returns true if any tools have not yet been yielded.
func (s *StreamingToolExecutor) HasUnfinishedTools() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tools {
		if t.Status != StatusYielded {
			return true
		}
	}
	return false
}
