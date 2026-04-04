// Package lsp implements the LSP tool for language server protocol queries.
// This is a stub that will be fully implemented in Phase 7 with the LSP client.
package lsp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Action    string `json:"action"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Character int    `json:"character"`
}

// LSPTool queries a Language Server Protocol server for code intelligence.
type LSPTool struct{}

// New creates a new LSPTool.
func New() *LSPTool { return &LSPTool{} }

func (t *LSPTool) Name() string                { return "LSP" }
func (t *LSPTool) Description() string          { return toolDescription }
func (t *LSPTool) IsReadOnly() bool             { return true }
func (t *LSPTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because LSP queries are read-only.
func (t *LSPTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *LSPTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("LSP", true, permCtx), nil
}

func (t *LSPTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Action) == "" {
		return tools.ErrorResult("required field \"action\" is missing or empty"), nil
	}
	if strings.TrimSpace(in.File) == "" {
		return tools.ErrorResult("required field \"file\" is missing or empty"), nil
	}

	return tools.TextResult("LSP integration not yet available. Use Grep and Read tools for code navigation."), nil
}
