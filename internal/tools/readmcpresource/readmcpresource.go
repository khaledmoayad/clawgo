// Package readmcpresource implements the ReadMcpResource tool.
// This is a stub that will be fully implemented in Phase 5 when the MCP client is available.
package readmcpresource

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Server string `json:"server"`
	URI    string `json:"uri"`
}

// ReadMcpResourceTool reads a specific MCP resource by URI.
type ReadMcpResourceTool struct{}

// New creates a new ReadMcpResourceTool.
func New() *ReadMcpResourceTool { return &ReadMcpResourceTool{} }

func (t *ReadMcpResourceTool) Name() string                { return "ReadMcpResourceTool" }
func (t *ReadMcpResourceTool) Description() string          { return toolDescription }
func (t *ReadMcpResourceTool) IsReadOnly() bool             { return true }
func (t *ReadMcpResourceTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because reading a resource is a read-only operation.
func (t *ReadMcpResourceTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *ReadMcpResourceTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("ReadMcpResourceTool", true, permCtx), nil
}

func (t *ReadMcpResourceTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Server) == "" {
		return tools.ErrorResult("required field \"server\" is missing or empty"), nil
	}
	if strings.TrimSpace(in.URI) == "" {
		return tools.ErrorResult("required field \"uri\" is missing or empty"), nil
	}

	return tools.TextResult("No MCP servers connected."), nil
}
