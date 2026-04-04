// Package croncreate implements the CronCreate tool.
// This is a stub that will be fully implemented in Phase 6 with the daemon worker system.
package croncreate

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
}

// CronCreateTool creates a new scheduled task with a cron expression.
type CronCreateTool struct{}

// New creates a new CronCreateTool.
func New() *CronCreateTool { return &CronCreateTool{} }

func (t *CronCreateTool) Name() string                { return "CronCreate" }
func (t *CronCreateTool) Description() string          { return toolDescription }
func (t *CronCreateTool) IsReadOnly() bool             { return false }
func (t *CronCreateTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because creating cron jobs modifies shared state.
func (t *CronCreateTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *CronCreateTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("CronCreate", false, permCtx), nil
}

func (t *CronCreateTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Name) == "" {
		return tools.ErrorResult("required field \"name\" is missing or empty"), nil
	}
	if strings.TrimSpace(in.Schedule) == "" {
		return tools.ErrorResult("required field \"schedule\" is missing or empty"), nil
	}
	if strings.TrimSpace(in.Command) == "" {
		return tools.ErrorResult("required field \"command\" is missing or empty"), nil
	}

	return tools.TextResult("Cron scheduling not yet available."), nil
}
