// Package cronlist implements the CronList tool.
// This is a stub that will be fully implemented in Phase 6 with the daemon worker system.
package cronlist

import (
	"context"
	"encoding/json"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

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

func (t *CronListTool) Call(_ context.Context, _ json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	return tools.TextResult("Cron scheduling not yet available."), nil
}
