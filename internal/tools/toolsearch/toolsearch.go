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

const defaultMaxResults = 5

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
	data, err := tools.ParseRawInput(inp)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	query, err := tools.RequireString(data, "query")
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(query) == "" {
		return tools.ErrorResult("required field \"query\" is missing or empty"), nil
	}

	// Parse max_results with semantic number coercion
	maxResultsF, err := tools.OptionalSemanticNumber(data, "max_results", float64(defaultMaxResults))
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("invalid \"max_results\" parameter: %s", err.Error())), nil
	}
	maxResults := int(maxResultsF)
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	// Check for "select:" prefix -- direct tool activation
	if strings.HasPrefix(strings.ToLower(query), "select:") {
		selectPart := query[len("select:"):]
		requested := strings.Split(selectPart, ",")

		var found []string
		var missing []string
		for _, name := range requested {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			tool := t.findTool(name)
			if tool != nil {
				found = append(found, tool.Name())
			} else {
				missing = append(missing, name)
			}
		}

		if len(found) == 0 {
			return tools.TextResult(fmt.Sprintf("No matching tools found for select query: %s", selectPart)), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Selected %d tool(s):\n", len(found)))
		for _, name := range found {
			sb.WriteString(fmt.Sprintf("  - %s\n", name))
		}
		if len(missing) > 0 {
			sb.WriteString(fmt.Sprintf("Not found: %s\n", strings.Join(missing, ", ")))
		}
		return tools.TextResult(sb.String()), nil
	}

	// Keyword search
	queryLower := strings.ToLower(query)
	var matches []string

	for _, tool := range t.registry.All() {
		name := strings.ToLower(tool.Name())
		desc := strings.ToLower(tool.Description())

		if strings.Contains(name, queryLower) || strings.Contains(desc, queryLower) {
			matches = append(matches, fmt.Sprintf("  - %s: %s", tool.Name(), firstLine(tool.Description())))
		}

		if len(matches) >= maxResults {
			break
		}
	}

	if len(matches) == 0 {
		return tools.TextResult(fmt.Sprintf("No tools found matching %q.", query)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d tool(s) matching %q:\n", len(matches), query))
	for _, m := range matches {
		sb.WriteString(m)
		sb.WriteString("\n")
	}

	return tools.TextResult(sb.String()), nil
}

// findTool looks up a tool by exact name (case-insensitive).
func (t *ToolSearchTool) findTool(name string) tools.Tool {
	nameLower := strings.ToLower(name)
	for _, tool := range t.registry.All() {
		if strings.ToLower(tool.Name()) == nameLower {
			return tool
		}
	}
	return nil
}

// firstLine returns the first line of a multi-line string.
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx != -1 {
		return s[:idx]
	}
	return s
}
