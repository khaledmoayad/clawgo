// Package taskoutput implements the TaskOutput tool for retrieving task output.
package taskoutput

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/khaledmoayad/clawgo/internal/tools/tasks"
)

const (
	defaultBlockTimeout = 30000.0  // 30 seconds in ms
	maxBlockTimeout     = 600000.0 // 10 minutes in ms
	pollInterval        = 250 * time.Millisecond
)

// TaskOutputTool retrieves the output log of a background task.
type TaskOutputTool struct {
	store *tasks.Store
}

// New creates a new TaskOutputTool with the given shared task store.
func New(store *tasks.Store) *TaskOutputTool {
	return &TaskOutputTool{store: store}
}

func (t *TaskOutputTool) Name() string                { return "TaskOutput" }
func (t *TaskOutputTool) Description() string          { return toolDescription }
func (t *TaskOutputTool) IsReadOnly() bool             { return true }
func (t *TaskOutputTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because reading task output is safe for concurrent execution.
func (t *TaskOutputTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *TaskOutputTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("TaskOutput", true, permCtx), nil
}

func (t *TaskOutputTool) Call(ctx context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	data, err := tools.ParseRawInput(inp)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	taskID, err := tools.RequireString(data, "task_id")
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(taskID) == "" {
		return tools.ErrorResult("required field \"task_id\" is missing or empty"), nil
	}

	// Parse block parameter with semantic coercion (handles string "true"/"false")
	block, err := tools.OptionalSemanticBool(data, "block", true)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("invalid \"block\" parameter: %s", err.Error())), nil
	}

	// Parse timeout parameter with semantic coercion (handles string numbers)
	timeoutMs, err := tools.OptionalSemanticNumber(data, "timeout", defaultBlockTimeout)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("invalid \"timeout\" parameter: %s", err.Error())), nil
	}
	if timeoutMs < 0 {
		timeoutMs = 0
	}
	if timeoutMs > maxBlockTimeout {
		timeoutMs = maxBlockTimeout
	}

	task, ok := t.store.Get(taskID)
	if !ok {
		return tools.ErrorResult(fmt.Sprintf("task %q not found", taskID)), nil
	}

	// If block=true and task is not yet finished, wait for completion
	if block && !isTerminalStatus(task.Status) {
		timeout := time.Duration(timeoutMs) * time.Millisecond
		deadline := time.Now().Add(timeout)

		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return tools.ErrorResult("request cancelled while waiting for task"), nil
			default:
			}

			// Re-fetch task status
			task, ok = t.store.Get(taskID)
			if !ok {
				return tools.ErrorResult(fmt.Sprintf("task %q not found", taskID)), nil
			}
			if isTerminalStatus(task.Status) {
				break
			}

			// Sleep briefly before polling again
			time.Sleep(pollInterval)
		}

		// Check if we timed out
		if !isTerminalStatus(task.Status) {
			output := task.Output
			if output == "" {
				output = "no output available yet"
			}
			return tools.TextResult(fmt.Sprintf(
				"Task %s (%s): timed out after %dms waiting for completion.\n%s",
				task.ID, task.Status, int(timeoutMs), output,
			)), nil
		}
	}

	output := task.Output
	if output == "" {
		return tools.TextResult(fmt.Sprintf("Task %s (%s): no output available yet.", task.ID, task.Status)), nil
	}

	return tools.TextResult(fmt.Sprintf("Task %s (%s) output:\n%s", task.ID, task.Status, output)), nil
}

// isTerminalStatus returns true if the task status indicates completion.
func isTerminalStatus(status string) bool {
	switch status {
	case "completed", "stopped", "failed":
		return true
	default:
		return false
	}
}
