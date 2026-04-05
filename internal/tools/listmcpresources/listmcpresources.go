// Package listmcpresources implements the ListMcpResources tool.
// It lists available resources from connected MCP servers.
package listmcpresources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/mcp"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// ListMcpResourcesTool lists available MCP resources from connected servers.
type ListMcpResourcesTool struct{}

// New creates a new ListMcpResourcesTool.
func New() *ListMcpResourcesTool { return &ListMcpResourcesTool{} }

func (t *ListMcpResourcesTool) Name() string                { return "ListMcpResourcesTool" }
func (t *ListMcpResourcesTool) Description() string          { return toolDescription }
func (t *ListMcpResourcesTool) IsReadOnly() bool             { return true }
func (t *ListMcpResourcesTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because listing resources is a read-only operation.
func (t *ListMcpResourcesTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *ListMcpResourcesTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("ListMcpResourcesTool", true, permCtx), nil
}

type input struct {
	Server string `json:"server"`
}

func (t *ListMcpResourcesTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	mgr, ok := toolCtx.MCPManager.(*mcp.Manager)
	if !ok || mgr == nil {
		return tools.ErrorResult("No MCP manager available. MCP servers may not be configured."), nil
	}

	resources, err := mgr.ListResources(ctx, in.Server)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("Error listing MCP resources: %s", err)), nil
	}

	if len(resources) == 0 {
		if in.Server != "" {
			return tools.TextResult(fmt.Sprintf("No resources found for server %q. MCP servers may still provide tools even if they have no resources.", in.Server)), nil
		}
		return tools.TextResult("No resources found. MCP servers may still provide tools even if they have no resources."), nil
	}

	// Format resources as readable text rows
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d resource(s):\n\n", len(resources)))
	for _, r := range resources {
		sb.WriteString(fmt.Sprintf("Server: %s\n", r.ServerName))
		sb.WriteString(fmt.Sprintf("  URI:   %s\n", r.URI))
		if r.Title != "" {
			sb.WriteString(fmt.Sprintf("  Title: %s\n", r.Title))
		}
		sb.WriteString(fmt.Sprintf("  Name:  %s\n", r.Name))
		if r.Description != "" {
			sb.WriteString(fmt.Sprintf("  Desc:  %s\n", r.Description))
		}
		if r.MIMEType != "" {
			sb.WriteString(fmt.Sprintf("  Type:  %s\n", r.MIMEType))
		}
		sb.WriteString("\n")
	}

	return tools.TextResult(sb.String()), nil
}
