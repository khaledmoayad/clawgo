// Package syntheticoutput implements the SyntheticOutput tool for injecting
// system-generated tool outputs into the conversation.
package syntheticoutput

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Content string `json:"content"`
	Format  string `json:"format"`
}

// SyntheticOutputTool returns content directly as a tool result.
type SyntheticOutputTool struct{}

// New creates a new SyntheticOutputTool.
func New() *SyntheticOutputTool { return &SyntheticOutputTool{} }

func (t *SyntheticOutputTool) Name() string                { return "StructuredOutput" }
func (t *SyntheticOutputTool) Description() string          { return toolDescription }
func (t *SyntheticOutputTool) IsReadOnly() bool             { return true }
func (t *SyntheticOutputTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because synthetic output is a pure pass-through.
func (t *SyntheticOutputTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *SyntheticOutputTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("StructuredOutput", true, permCtx), nil
}

func (t *SyntheticOutputTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Content) == "" {
		return tools.ErrorResult("required field \"content\" is missing or empty"), nil
	}

	// Return the content directly as a tool result
	return tools.TextResult(in.Content), nil
}
