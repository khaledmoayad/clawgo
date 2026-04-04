// Package crondelete implements the CronDelete tool.
// This is a stub that will be fully implemented in Phase 6 with the daemon worker system.
package crondelete

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Name string `json:"name"`
}

// CronDeleteTool deletes a scheduled cron task by name.
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

func (t *CronDeleteTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Name) == "" {
		return tools.ErrorResult("required field \"name\" is missing or empty"), nil
	}

	return tools.TextResult("Cron scheduling not yet available."), nil
}
