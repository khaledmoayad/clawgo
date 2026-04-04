// Package taskcreate implements the TaskCreate tool for creating background tasks.
package taskcreate

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
	Description string `json:"description"`
	Type        string `json:"type"`
	Command     string `json:"command"`
}

// TaskCreateTool creates background tasks in the shared task store.
type TaskCreateTool struct {
	store *tasks.Store
}

// New creates a new TaskCreateTool with the given shared task store.
func New(store *tasks.Store) *TaskCreateTool {
	return &TaskCreateTool{store: store}
}

func (t *TaskCreateTool) Name() string                { return "TaskCreate" }
func (t *TaskCreateTool) Description() string          { return toolDescription }
func (t *TaskCreateTool) IsReadOnly() bool             { return false }
func (t *TaskCreateTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because task creation modifies shared state.
func (t *TaskCreateTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *TaskCreateTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("TaskCreate", false, permCtx), nil
}

func (t *TaskCreateTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Description) == "" {
		return tools.ErrorResult("required field \"description\" is missing or empty"), nil
	}

	task := t.store.Create(in.Description, in.Type)

	return tools.TextResult(fmt.Sprintf("Created task %s (type: %s, status: %s): %s",
		task.ID, task.Type, task.Status, task.Description)), nil
}
