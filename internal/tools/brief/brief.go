// Package brief implements the BriefTool for enabling concise response mode.
// When invoked, it sets a context modifier that flags brief mode on the
// ToolUseContext, matching the TypeScript Brief tool behavior.
package brief

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Message string `json:"message"`
}

// BriefTool enables brief/concise response mode.
type BriefTool struct{}

// New creates a new BriefTool.
func New() *BriefTool { return &BriefTool{} }

func (t *BriefTool) Name() string                { return "Brief" }
func (t *BriefTool) Description() string          { return toolDescription }
func (t *BriefTool) IsReadOnly() bool             { return true }
func (t *BriefTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true -- setting brief mode is a safe metadata operation.
func (t *BriefTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// CheckPermissions returns Allow -- enabling brief mode is always permitted.
func (t *BriefTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

func (t *BriefTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Message) == "" {
		return tools.ErrorResult("required field \"message\" is missing or empty"), nil
	}

	return &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: in.Message}},
		Metadata: map[string]any{
			"brief_mode": true,
		},
		ContextModifier: func(ctx *tools.ToolUseContext) {
			// The ToolUseContext doesn't have a BriefMode field yet;
			// the query loop will check ToolResult.Metadata for "brief_mode"
			// to adjust system prompt behavior. This ContextModifier is a
			// placeholder that will be wired when the REPL handles brief mode.
		},
	}, nil
}
