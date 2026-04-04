// Package askuser implements the AskUserQuestionTool for interactive user prompts.
// It returns a special ToolResult that signals the REPL to pause and prompt
// the user for input, matching the TypeScript AskUserQuestion tool behavior.
package askuser

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Question string `json:"question"`
}

// AskUserTool pauses execution to prompt the user for input.
type AskUserTool struct{}

// New creates a new AskUserTool.
func New() *AskUserTool { return &AskUserTool{} }

func (t *AskUserTool) Name() string                { return "AskUserQuestion" }
func (t *AskUserTool) Description() string          { return toolDescription }
func (t *AskUserTool) IsReadOnly() bool             { return true }
func (t *AskUserTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because it blocks on user input.
func (t *AskUserTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

// CheckPermissions returns Allow -- asking the user is always permitted.
func (t *AskUserTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

func (t *AskUserTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Question) == "" {
		return tools.ErrorResult("required field \"question\" is missing or empty"), nil
	}

	// Return the question as the tool result text with metadata signaling
	// that user input is required. The REPL will detect AskUserQuestion
	// tool results and handle user prompting in the integration plan.
	return &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: in.Question}},
		Metadata: map[string]any{
			"requires_user_input": true,
			"question":            in.Question,
		},
	}, nil
}
