package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/api"
	"golang.org/x/sync/errgroup"
)

// MaxConcurrency is the maximum number of tools that can execute
// concurrently in a single batch. Matches the TypeScript
// CLAUDE_CODE_MAX_TOOL_USE_CONCURRENCY constant.
const MaxConcurrency = 10

// ToolCallEntry represents a single tool_use block from the assistant response.
type ToolCallEntry struct {
	ID    string          // tool_use ID from the API
	Name  string          // tool name
	Input json.RawMessage // raw JSON input for the tool
}

// Batch is a group of tool calls that share the same concurrency classification.
// Concurrent-safe batches run in parallel; non-safe batches run sequentially.
type Batch struct {
	ConcurrencySafe bool
	Entries         []ToolCallEntry
}

// PermissionFn is the signature for permission checking during batch execution.
type PermissionFn func(name string, input json.RawMessage, tool Tool) (PermissionResult, error)

// PartitionToolCalls groups consecutive tool calls into batches based on
// their concurrency safety classification. Consecutive safe tools are grouped
// into a single concurrent batch; each unsafe tool gets its own serial batch.
// Unknown tools (not in registry) are treated as non-safe.
func PartitionToolCalls(entries []ToolCallEntry, registry *Registry) []Batch {
	if len(entries) == 0 {
		return nil
	}

	var batches []Batch
	var currentBatch *Batch

	for _, entry := range entries {
		tool, ok := registry.Get(entry.Name)
		safe := false
		if ok {
			safe = tool.IsConcurrencySafe(entry.Input)
		}

		if safe {
			// Try to add to current concurrent batch
			if currentBatch != nil && currentBatch.ConcurrencySafe {
				currentBatch.Entries = append(currentBatch.Entries, entry)
			} else {
				// Start new concurrent batch
				if currentBatch != nil {
					batches = append(batches, *currentBatch)
				}
				currentBatch = &Batch{
					ConcurrencySafe: true,
					Entries:         []ToolCallEntry{entry},
				}
			}
		} else {
			// Non-safe: flush current batch, then create single-item batch
			if currentBatch != nil {
				batches = append(batches, *currentBatch)
			}
			currentBatch = &Batch{
				ConcurrencySafe: false,
				Entries:         []ToolCallEntry{entry},
			}
		}
	}

	// Flush the last batch
	if currentBatch != nil {
		batches = append(batches, *currentBatch)
	}

	return batches
}

// ExecuteBatches processes all batches in sequence. Concurrent-safe batches
// are executed in parallel via RunConcurrentBatch; non-safe batches are
// executed sequentially via RunSerialBatch. ContextModifiers from tool
// results are applied after each tool execution.
func ExecuteBatches(
	ctx context.Context,
	batches []Batch,
	registry *Registry,
	toolCtx *ToolUseContext,
	permissionFn PermissionFn,
) ([]api.ToolResultEntry, error) {
	var allResults []api.ToolResultEntry

	for _, batch := range batches {
		var results []api.ToolResultEntry
		var err error

		if batch.ConcurrencySafe {
			results, err = RunConcurrentBatch(ctx, batch, registry, toolCtx, permissionFn)
		} else {
			results, err = RunSerialBatch(ctx, batch, registry, toolCtx, permissionFn)
		}

		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// RunConcurrentBatch executes all tools in a batch concurrently using
// errgroup with a bounded concurrency of maxConcurrency (10).
// Results are returned in the same order as the input entries.
func RunConcurrentBatch(
	ctx context.Context,
	batch Batch,
	registry *Registry,
	toolCtx *ToolUseContext,
	permissionFn PermissionFn,
) ([]api.ToolResultEntry, error) {
	results := make([]api.ToolResultEntry, len(batch.Entries))

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(MaxConcurrency) // SetLimit(10) -- matches TS CLAUDE_CODE_MAX_TOOL_USE_CONCURRENCY

	for i, entry := range batch.Entries {
		i, entry := i, entry // capture loop variables
		g.Go(func() error {
			result := executeSingleTool(gCtx, entry, registry, toolCtx, permissionFn)
			results[i] = result
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, err
	}

	return results, nil
}

// RunSerialBatch executes tools one at a time in order.
// ContextModifiers from tool results are applied after each execution
// so subsequent tools see the updated context.
func RunSerialBatch(
	ctx context.Context,
	batch Batch,
	registry *Registry,
	toolCtx *ToolUseContext,
	permissionFn PermissionFn,
) ([]api.ToolResultEntry, error) {
	results := make([]api.ToolResultEntry, 0, len(batch.Entries))

	for _, entry := range batch.Entries {
		result := executeSingleTool(ctx, entry, registry, toolCtx, permissionFn)
		results = append(results, result)
	}

	return results, nil
}

// executeSingleTool looks up a tool, checks permissions, executes it,
// applies any ContextModifier, and returns a ToolResultEntry.
func executeSingleTool(
	ctx context.Context,
	entry ToolCallEntry,
	registry *Registry,
	toolCtx *ToolUseContext,
	permissionFn PermissionFn,
) api.ToolResultEntry {
	tool, ok := registry.Get(entry.Name)
	if !ok {
		return api.ToolResultEntry{
			ToolUseID: entry.ID,
			Content:   fmt.Sprintf("Unknown tool: %s", entry.Name),
			IsError:   true,
		}
	}

	// Check permissions
	if permissionFn != nil {
		permResult, err := permissionFn(entry.Name, entry.Input, tool)
		if err != nil {
			return api.ToolResultEntry{
				ToolUseID: entry.ID,
				Content:   fmt.Sprintf("Permission check error: %s", err),
				IsError:   true,
			}
		}
		if permResult == PermissionDeny {
			return api.ToolResultEntry{
				ToolUseID: entry.ID,
				Content:   fmt.Sprintf("Tool %s is disallowed by settings", entry.Name),
				IsError:   true,
			}
		}
	}

	// Execute tool
	result, err := tool.Call(ctx, entry.Input, toolCtx)
	if err != nil {
		return api.ToolResultEntry{
			ToolUseID: entry.ID,
			Content:   err.Error(),
			IsError:   true,
		}
	}

	// Apply context modifier if present
	if result.ContextModifier != nil {
		result.ContextModifier(toolCtx)
	}

	// Convert to API result entry
	content := ""
	if len(result.Content) > 0 {
		content = result.Content[0].Text
	}

	return api.ToolResultEntry{
		ToolUseID: entry.ID,
		Content:   content,
		IsError:   result.IsError,
	}
}

// StreamingExecutor delivers tool execution results incrementally via channels.
type StreamingExecutor struct{}

// Execute runs a single tool call and returns a channel of StreamEvents.
// The channel is closed when execution completes.
func (se *StreamingExecutor) Execute(
	ctx context.Context,
	entry ToolCallEntry,
	registry *Registry,
	toolCtx *ToolUseContext,
) <-chan StreamEvent {
	ch := make(chan StreamEvent, 8)

	go func() {
		defer close(ch)

		tool, ok := registry.Get(entry.Name)
		if !ok {
			ch <- StreamEvent{
				Type: "error",
				Text: fmt.Sprintf("Unknown tool: %s", entry.Name),
				Done: true,
			}
			return
		}

		// Send progress event
		ch <- StreamEvent{
			Type: "progress",
			Text: fmt.Sprintf("Executing %s...", entry.Name),
		}

		// Execute
		result, err := tool.Call(ctx, entry.Input, toolCtx)
		if err != nil {
			ch <- StreamEvent{
				Type: "error",
				Text: err.Error(),
				Done: true,
			}
			return
		}

		// Send result
		content := ""
		if len(result.Content) > 0 {
			content = result.Content[0].Text
		}

		if result.IsError {
			ch <- StreamEvent{
				Type: "error",
				Text: content,
				Done: true,
			}
			return
		}

		ch <- StreamEvent{
			Type: "complete",
			Text: content,
			Done: true,
		}
	}()

	return ch
}
