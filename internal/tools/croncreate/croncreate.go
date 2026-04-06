// Package croncreate implements the CronCreate tool.
// Creates scheduled cron tasks that fire prompts into the query loop.
package croncreate

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
	Cron      string `json:"cron"`
	Prompt    string `json:"prompt"`
	Recurring *bool  `json:"recurring,omitempty"`
	Durable   *bool  `json:"durable,omitempty"`
}

type output struct {
	ID            string `json:"id"`
	HumanSchedule string `json:"humanSchedule"`
	Recurring     bool   `json:"recurring"`
	Durable       bool   `json:"durable,omitempty"`
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

func (t *CronCreateTool) Call(_ context.Context, inp json.RawMessage, tuc *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Cron) == "" {
		return tools.ErrorResult("required field \"cron\" is missing or empty"), nil
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return tools.ErrorResult("required field \"prompt\" is missing or empty"), nil
	}

	// Validate cron expression by attempting to compute next run
	nextMs := daemon.NextCronRunMs(in.Cron, daemon.NowMs())
	if nextMs == nil {
		return tools.ErrorResult(fmt.Sprintf("invalid cron expression %q or no match within the next year", in.Cron)), nil
	}

	// Defaults: recurring=true, durable=false
	recurring := true
	if in.Recurring != nil {
		recurring = *in.Recurring
	}
	durable := false
	if in.Durable != nil {
		durable = *in.Durable
	}

	// Check job limit
	dir := tuc.ProjectRoot
	count, err := daemon.CountAllTasks(dir)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to count existing tasks: %v", err)), nil
	}
	if count >= daemon.MaxCronJobs {
		return tools.ErrorResult(fmt.Sprintf("maximum of %d concurrent scheduled jobs reached", daemon.MaxCronJobs)), nil
	}

	id, err := daemon.AddCronTask(in.Cron, in.Prompt, recurring, durable, dir)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to create cron task: %v", err)), nil
	}

	humanSchedule := daemon.HumanReadableSchedule(in.Cron)
	out := output{
		ID:            id,
		HumanSchedule: humanSchedule,
		Recurring:     recurring,
		Durable:       durable,
	}

	data, err := json.Marshal(out)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to marshal output: %v", err)), nil
	}

	return tools.TextResult(string(data)), nil
}
