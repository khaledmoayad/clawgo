// Package tasklist implements the TaskList tool for listing background tasks.
package tasklist

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
	Status string `json:"status"`
}

// TaskListTool lists all background tasks with optional status filtering.
type TaskListTool struct {
	store *tasks.Store
}

// New creates a new TaskListTool with the given shared task store.
func New(store *tasks.Store) *TaskListTool {
	return &TaskListTool{store: store}
}

func (t *TaskListTool) Name() string                { return "TaskList" }
func (t *TaskListTool) Description() string          { return toolDescription }
func (t *TaskListTool) IsReadOnly() bool             { return true }
func (t *TaskListTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because listing tasks is a read-only operation.
func (t *TaskListTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *TaskListTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("TaskList", true, permCtx), nil
}

func (t *TaskListTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	taskList := t.store.List(in.Status)

	if len(taskList) == 0 {
		if in.Status != "" {
			return tools.TextResult(fmt.Sprintf("No tasks with status %q found.", in.Status)), nil
		}
		return tools.TextResult("No tasks found."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tasks (%d):\n", len(taskList)))
	for _, task := range taskList {
		sb.WriteString(fmt.Sprintf("  - %s [%s] (%s): %s\n", task.ID, task.Status, task.Type, task.Description))
	}

	return tools.TextResult(sb.String()), nil
}
