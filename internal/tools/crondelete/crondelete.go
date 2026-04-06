// Package crondelete implements the CronDelete tool.
// Deletes scheduled cron tasks by ID from both file-backed and session-scoped stores.
package crondelete

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/daemon"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	ID string `json:"id"`
}

type output struct {
	ID string `json:"id"`
}

// CronDeleteTool deletes a scheduled cron task by ID.
type CronDeleteTool struct{}

// New creates a new CronDeleteTool.
func New() *CronDeleteTool { return &CronDeleteTool{} }

func (t *CronDeleteTool) Name() string                { return "CronDelete" }
func (t *CronDeleteTool) Description() string          { return toolDescription }
func (t *CronDeleteTool) IsReadOnly() bool             { return false }
func (t *CronDeleteTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because deleting cron jobs modifies shared state.
func (t *CronDeleteTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *CronDeleteTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("CronDelete", false, permCtx), nil
}

func (t *CronDeleteTool) Call(_ context.Context, inp json.RawMessage, tuc *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.ID) == "" {
		return tools.ErrorResult("required field \"id\" is missing or empty"), nil
	}

	dir := tuc.ProjectRoot

	// Check if the task exists in file-backed tasks
	fileTasks, err := daemon.ReadCronTasks(dir)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to read tasks: %v", err)), nil
	}

	found := false
	for _, task := range fileTasks {
		if task.ID == in.ID {
			found = true
			break
		}
	}

	if found {
		if err := daemon.RemoveCronTasks([]string{in.ID}, dir); err != nil {
			return tools.ErrorResult(fmt.Sprintf("failed to delete task: %v", err)), nil
		}
	} else {
		// Check session-scoped tasks
		sessionTasks := daemon.GetSessionTasks()
		for _, task := range sessionTasks {
			if task.ID == in.ID {
				found = true
				break
			}
		}
		if found {
			daemon.RemoveSessionTask(in.ID)
		}
	}

	if !found {
		return tools.ErrorResult(fmt.Sprintf("no cron task found with id %q", in.ID)), nil
	}

	out := output{ID: in.ID}
	data, err := json.Marshal(out)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to marshal output: %v", err)), nil
	}

	return tools.TextResult(string(data)), nil
}
