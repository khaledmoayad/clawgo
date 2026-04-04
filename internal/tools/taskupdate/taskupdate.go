// Package taskupdate implements the TaskUpdate tool for modifying background task state.
package taskupdate

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
	TaskID  string `json:"task_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// TaskUpdateTool updates the status or output of a background task.
type TaskUpdateTool struct {
	store *tasks.Store
}

// New creates a new TaskUpdateTool with the given shared task store.
func New(store *tasks.Store) *TaskUpdateTool {
	return &TaskUpdateTool{store: store}
}

func (t *TaskUpdateTool) Name() string                { return "TaskUpdate" }
func (t *TaskUpdateTool) Description() string          { return toolDescription }
func (t *TaskUpdateTool) IsReadOnly() bool             { return false }
func (t *TaskUpdateTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because task updates modify shared state.
func (t *TaskUpdateTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *TaskUpdateTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("TaskUpdate", false, permCtx), nil
}

func (t *TaskUpdateTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.TaskID) == "" {
		return tools.ErrorResult("required field \"task_id\" is missing or empty"), nil
	}

	if err := t.store.Update(in.TaskID, in.Status, in.Message); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.TextResult(fmt.Sprintf("Updated task %s", in.TaskID)), nil
}
