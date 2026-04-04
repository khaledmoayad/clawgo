// Package exitplanmode implements the ExitPlanModeTool that switches
// the permission mode back from plan mode to the default mode,
// matching the TypeScript ExitPlanMode tool behavior.
package exitplanmode

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	PlanSummary string `json:"plan_summary"`
}

// ExitPlanModeTool switches back from plan mode to default permission mode.
type ExitPlanModeTool struct{}

// New creates a new ExitPlanModeTool.
func New() *ExitPlanModeTool { return &ExitPlanModeTool{} }

func (t *ExitPlanModeTool) Name() string                { return "ExitPlanMode" }
func (t *ExitPlanModeTool) Description() string          { return toolDescription }
func (t *ExitPlanModeTool) IsReadOnly() bool             { return false }
func (t *ExitPlanModeTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because it modifies the permission state.
func (t *ExitPlanModeTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

// CheckPermissions returns Allow -- exiting plan mode is always permitted.
func (t *ExitPlanModeTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

func (t *ExitPlanModeTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.PlanSummary) == "" {
		return tools.ErrorResult("required field \"plan_summary\" is missing or empty"), nil
	}

	return &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: in.PlanSummary}},
		Metadata: map[string]any{
			"plan_mode": false,
		},
		ContextModifier: func(ctx *tools.ToolUseContext) {
			if ctx.PermCtx != nil {
				ctx.PermCtx.Mode = permissions.ModeDefault
			}
		},
	}, nil
}
