// Package taskoutput implements the TaskOutput tool for retrieving task output.
package taskoutput

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/khaledmoayad/clawgo/internal/tools/tasks"
)

type input struct {
	TaskID string `json:"task_id"`
}

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

func (t *TaskOutputTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.TaskID) == "" {
		return tools.ErrorResult("required field \"task_id\" is missing or empty"), nil
	}

	task, ok := t.store.Get(in.TaskID)
	if !ok {
		return tools.ErrorResult(fmt.Sprintf("task %q not found", in.TaskID)), nil
	}

	output := task.Output
	if output == "" {
		return tools.TextResult(fmt.Sprintf("Task %s (%s): no output available yet.", task.ID, task.Status)), nil
	}

	return tools.TextResult(fmt.Sprintf("Task %s (%s) output:\n%s", task.ID, task.Status, output)), nil
}
