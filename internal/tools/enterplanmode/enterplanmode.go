// Package enterplanmode implements the EnterPlanModeTool that switches
// the permission mode to plan mode, where all mutations require explicit
// user approval, matching the TypeScript EnterPlanMode tool behavior.
package enterplanmode

import (
	"context"
	"encoding/json"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// EnterPlanModeTool switches to plan mode where mutations require approval.
type EnterPlanModeTool struct{}

// New creates a new EnterPlanModeTool.
func New() *EnterPlanModeTool { return &EnterPlanModeTool{} }

func (t *EnterPlanModeTool) Name() string                { return "EnterPlanMode" }
func (t *EnterPlanModeTool) Description() string          { return toolDescription }
func (t *EnterPlanModeTool) IsReadOnly() bool             { return true }
func (t *EnterPlanModeTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because it modifies the permission state.
func (t *EnterPlanModeTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

// CheckPermissions returns Allow -- entering plan mode is always permitted.
func (t *EnterPlanModeTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

func (t *EnterPlanModeTool) Call(_ context.Context, _ json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	return &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: "Entered plan mode -- all mutations will require approval."}},
		Metadata: map[string]any{
			"plan_mode": true,
		},
		ContextModifier: func(ctx *tools.ToolUseContext) {
			if ctx.PermCtx != nil {
				ctx.PermCtx.Mode = permissions.ModePlan
			}
		},
	}, nil
}
