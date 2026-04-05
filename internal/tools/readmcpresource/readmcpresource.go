// Package readmcpresource implements the ReadMcpResource tool.
// It reads a specific resource from a connected MCP server by URI.
package readmcpresource

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/mcp"
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

func (t *ReadMcpResourceTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
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

	mgr, ok := toolCtx.MCPManager.(*mcp.Manager)
	if !ok || mgr == nil {
		return tools.ErrorResult("No MCP manager available. MCP servers may not be configured."), nil
	}

	result, err := mgr.ReadResource(ctx, in.Server, in.URI)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("Error reading resource from server %q: %s", in.Server, err)), nil
	}

	if result == nil || len(result.Contents) == 0 {
		return tools.TextResult(fmt.Sprintf("Resource %q from server %q returned no content.", in.URI, in.Server)), nil
	}

	// Render content entries into readable text
	var sb strings.Builder
	for i, c := range result.Contents {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("URI: %s\n", c.URI))
		if c.MIMEType != "" {
			sb.WriteString(fmt.Sprintf("Type: %s\n", c.MIMEType))
		}
		if c.Text != "" {
			sb.WriteString(fmt.Sprintf("\n%s\n", c.Text))
		} else if len(c.Blob) > 0 {
			sb.WriteString(fmt.Sprintf("\n[Binary content: %d bytes]\n", len(c.Blob)))
		}
	}

	return tools.TextResult(sb.String()), nil
}
