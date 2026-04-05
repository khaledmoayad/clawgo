// Package syntheticoutput implements the StructuredOutput tool for returning
// structured JSON responses. The tool accepts any JSON object as input and
// validates it against a user-provided JSON schema at runtime.
package syntheticoutput

import (
	"context"
	"encoding/json"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// SyntheticOutputTool returns structured JSON output validated against a dynamic schema.
type SyntheticOutputTool struct {
	// JSONSchema is the optional user-provided JSON schema for runtime validation.
	// When nil, any JSON object is accepted.
	JSONSchema json.RawMessage
}

// New creates a new SyntheticOutputTool without a validation schema.
func New() *SyntheticOutputTool { return &SyntheticOutputTool{} }

// NewWithSchema creates a SyntheticOutputTool that validates input against the given JSON schema.
func NewWithSchema(schema json.RawMessage) *SyntheticOutputTool {
	return &SyntheticOutputTool{JSONSchema: schema}
}

func (t *SyntheticOutputTool) Name() string       { return "StructuredOutput" }
func (t *SyntheticOutputTool) Description() string { return toolDescription }
func (t *SyntheticOutputTool) IsReadOnly() bool    { return true }

// InputSchema returns the dynamic JSON schema if set, otherwise the default permissive schema.
func (t *SyntheticOutputTool) InputSchema() json.RawMessage {
	if len(t.JSONSchema) > 0 {
		return t.JSONSchema
	}
	return json.RawMessage(inputSchemaJSON)
}

// IsConcurrencySafe returns true because structured output is a pure pass-through.
func (t *SyntheticOutputTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *SyntheticOutputTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	// Always allow -- this tool just returns data.
	return permissions.Allow, nil
}

func (t *SyntheticOutputTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	// Validate that input is valid JSON (the raw message is the structured output itself).
	var parsed map[string]interface{}
	if err := json.Unmarshal(inp, &parsed); err != nil {
		return tools.ErrorResult("structured output must be a valid JSON object: " + err.Error()), nil
	}

	return &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: "Structured output provided successfully"}},
		Metadata: map[string]any{
			"structured_output": parsed,
		},
	}, nil
}
