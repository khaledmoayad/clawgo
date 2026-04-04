// Package listmcpresources implements the ListMcpResources tool.
// This is a stub that will be fully implemented in Phase 5 when the MCP client is available.
package listmcpresources

import (
	"context"
	"encoding/json"

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

func (t *ListMcpResourcesTool) Call(_ context.Context, _ json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	return tools.TextResult("No MCP servers connected."), nil
}
