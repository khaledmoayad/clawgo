// Package taskget implements the TaskGet tool for retrieving background task status.
package taskget

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
	TaskID string `json:"taskId"`
}

// TaskGetTool retrieves the status and details of a background task.
type TaskGetTool struct {
	store *tasks.Store
}

// New creates a new TaskGetTool with the given shared task store.
func New(store *tasks.Store) *TaskGetTool {
	return &TaskGetTool{store: store}
}

func (t *TaskGetTool) Name() string                { return "TaskGet" }
func (t *TaskGetTool) Description() string          { return toolDescription }
func (t *TaskGetTool) IsReadOnly() bool             { return true }
func (t *TaskGetTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because reading task status is safe for concurrent execution.
func (t *TaskGetTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *TaskGetTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("TaskGet", true, permCtx), nil
}

func (t *TaskGetTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.TaskID) == "" {
		return tools.ErrorResult("required field \"taskId\" is missing or empty"), nil
	}

	task, ok := t.store.Get(in.TaskID)
	if !ok {
		return tools.ErrorResult(fmt.Sprintf("task %q not found", in.TaskID)), nil
	}

	output := task.Output
	if output == "" {
		output = "(no output yet)"
	}

	return tools.TextResult(fmt.Sprintf("Task: %s\nType: %s\nStatus: %s\nDescription: %s\nOutput: %s",
		task.ID, task.Type, task.Status, task.Description, output)), nil
}
