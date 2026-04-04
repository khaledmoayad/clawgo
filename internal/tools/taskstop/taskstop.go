// Package taskstop implements the TaskStop tool for stopping background tasks.
package taskstop

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

// TaskStopTool stops a running background task.
type TaskStopTool struct {
	store *tasks.Store
}

// New creates a new TaskStopTool with the given shared task store.
func New(store *tasks.Store) *TaskStopTool {
	return &TaskStopTool{store: store}
}

func (t *TaskStopTool) Name() string                { return "TaskStop" }
func (t *TaskStopTool) Description() string          { return toolDescription }
func (t *TaskStopTool) IsReadOnly() bool             { return false }
func (t *TaskStopTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because stopping a task modifies shared state.
func (t *TaskStopTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *TaskStopTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("TaskStop", false, permCtx), nil
}

func (t *TaskStopTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.TaskID) == "" {
		return tools.ErrorResult("required field \"task_id\" is missing or empty"), nil
	}

	if err := t.store.Stop(in.TaskID); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.TextResult(fmt.Sprintf("Task %s has been stopped.", in.TaskID)), nil
}
