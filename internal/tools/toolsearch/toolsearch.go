// Package toolsearch implements the ToolSearch tool for searching available tools by name or description.
package toolsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Query string `json:"query"`
}

// ToolSearchTool searches the tool registry by name and description.
type ToolSearchTool struct {
	registry *tools.Registry
}

// New creates a new ToolSearchTool with the given tool registry.
func New(registry *tools.Registry) *ToolSearchTool {
	return &ToolSearchTool{registry: registry}
}

func (t *ToolSearchTool) Name() string                { return "ToolSearch" }
func (t *ToolSearchTool) Description() string          { return toolDescription }
func (t *ToolSearchTool) IsReadOnly() bool             { return true }
func (t *ToolSearchTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because searching tools is a read-only operation.
func (t *ToolSearchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *ToolSearchTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("ToolSearch", true, permCtx), nil
}

func (t *ToolSearchTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Query) == "" {
		return tools.ErrorResult("required field \"query\" is missing or empty"), nil
	}

	query := strings.ToLower(in.Query)
	var matches []string

	for _, tool := range t.registry.All() {
		name := strings.ToLower(tool.Name())
		desc := strings.ToLower(tool.Description())

		if strings.Contains(name, query) || strings.Contains(desc, query) {
			matches = append(matches, fmt.Sprintf("  - %s: %s", tool.Name(), firstLine(tool.Description())))
		}
	}

	if len(matches) == 0 {
		return tools.TextResult(fmt.Sprintf("No tools found matching %q.", in.Query)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d tool(s) matching %q:\n", len(matches), in.Query))
	for _, m := range matches {
		sb.WriteString(m)
		sb.WriteString("\n")
	}

	return tools.TextResult(sb.String()), nil
}

// firstLine returns the first line of a multi-line string.
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx != -1 {
		return s[:idx]
	}
	return s
}
