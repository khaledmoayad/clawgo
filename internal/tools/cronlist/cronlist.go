// Package cronlist implements the CronList tool.
// Lists all scheduled cron tasks (both durable and session-scoped).
package cronlist

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/khaledmoayad/clawgo/internal/daemon"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type jobEntry struct {
	ID            string `json:"id"`
	Cron          string `json:"cron"`
	HumanSchedule string `json:"humanSchedule"`
	Prompt        string `json:"prompt"`
	Recurring     bool   `json:"recurring,omitempty"`
	Durable       bool   `json:"durable,omitempty"`
}

type output struct {
	Jobs []jobEntry `json:"jobs"`
}

// CronListTool lists all configured cron tasks and their schedules.
type CronListTool struct{}

// New creates a new CronListTool.
func New() *CronListTool { return &CronListTool{} }

func (t *CronListTool) Name() string                { return "CronList" }
func (t *CronListTool) Description() string          { return toolDescription }
func (t *CronListTool) IsReadOnly() bool             { return true }
func (t *CronListTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because listing cron tasks is a read-only operation.
func (t *CronListTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *CronListTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("CronList", true, permCtx), nil
}

func (t *CronListTool) Call(_ context.Context, _ json.RawMessage, tuc *tools.ToolUseContext) (*tools.ToolResult, error) {
	dir := tuc.ProjectRoot

	// Read file-backed tasks
	fileTasks, err := daemon.ReadCronTasks(dir)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to read tasks: %v", err)), nil
	}

	// Read session-scoped tasks
	sessionTasks := daemon.GetSessionTasks()

	jobs := make([]jobEntry, 0, len(fileTasks)+len(sessionTasks))

	for _, t := range fileTasks {
		isRecurring := t.Recurring != nil && *t.Recurring
		jobs = append(jobs, jobEntry{
			ID:            t.ID,
			Cron:          t.Cron,
			HumanSchedule: daemon.HumanReadableSchedule(t.Cron),
			Prompt:        t.Prompt,
			Recurring:     isRecurring,
			Durable:       true,
		})
	}

	for _, t := range sessionTasks {
		isRecurring := t.Recurring != nil && *t.Recurring
		jobs = append(jobs, jobEntry{
			ID:            t.ID,
			Cron:          t.Cron,
			HumanSchedule: daemon.HumanReadableSchedule(t.Cron),
			Prompt:        t.Prompt,
			Recurring:     isRecurring,
			Durable:       false,
		})
	}

	out := output{Jobs: jobs}
	data, err := json.Marshal(out)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to marshal output: %v", err)), nil
	}

	return tools.TextResult(string(data)), nil
}
