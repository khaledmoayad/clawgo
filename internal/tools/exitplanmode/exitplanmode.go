// Package exitplanmode implements the ExitPlanModeTool that switches
// the permission mode back from plan mode to the default mode,
// matching the TypeScript ExitPlanMode tool behavior.
package exitplanmode

import (
	"context"
	"encoding/json"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// allowedPrompt represents a prompt-based permission request.
type allowedPrompt struct {
	Tool   string `json:"tool"`
	Prompt string `json:"prompt"`
}

type input struct {
	AllowedPrompts []allowedPrompt `json:"allowedPrompts,omitempty"`
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

	return &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: "Exited plan mode."}},
		Metadata: map[string]any{
			"plan_mode":       false,
			"allowedPrompts":  in.AllowedPrompts,
		},
		ContextModifier: func(ctx *tools.ToolUseContext) {
			if ctx.PermCtx != nil {
				ctx.PermCtx.Mode = permissions.ModeDefault
			}
		},
	}, nil
}
