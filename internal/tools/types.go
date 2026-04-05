// Package tools defines the tool system infrastructure for ClawGo.
// It provides the Tool interface, Registry, input validation helpers,
// and shared types used across all tool implementations.
package tools

import (
	"context"

	"github.com/khaledmoayad/clawgo/internal/filestate"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools/tasks"
)

// PermissionResult is an alias for permissions.PermissionResult,
// re-exported here for convenience so tool implementations don't
// need to import the permissions package directly.
type PermissionResult = permissions.PermissionResult

// PermissionContext is an alias for permissions.PermissionContext,
// re-exported here for convenience.
type PermissionContext = permissions.PermissionContext

// Permission result constants re-exported for convenience.
var (
	PermissionAllow = permissions.Allow
	PermissionDeny  = permissions.Deny
	PermissionAsk   = permissions.Ask
)

// ToolUseContext carries runtime state into tool execution.
type ToolUseContext struct {
	WorkingDir     string
	ProjectRoot    string
	SessionID      string
	AbortCtx       context.Context
	PermCtx        *permissions.PermissionContext
	TaskStore      *tasks.Store
	FileStateCache *filestate.FileStateCache

	// MCPManager holds a reference to the MCP Manager (typed as any to
	// avoid circular imports between tools and mcp packages). Tool
	// implementations that need MCP access type-assert this to
	// *mcp.Manager.
	MCPManager any
}

// ContentBlock for tool results (text or image content).
type ContentBlock struct {
	Type string `json:"type"` // "text", "image"
	Text string `json:"text,omitempty"`
}

// ToolResult is returned by Tool.Call().
type ToolResult struct {
	Content  []ContentBlock
	IsError  bool
	Metadata map[string]any

	// ContextModifier, if non-nil, is applied after tool execution to modify
	// the ToolUseContext for subsequent loop iterations.
	// This matches the TypeScript pattern where tool results carry context modifiers.
	ContextModifier func(*ToolUseContext)
}

// StreamEvent represents an incremental output event from a streaming tool execution.
// Used by the StreamingExecutor to deliver partial results via channels.
type StreamEvent struct {
	Type string // "text", "progress", "complete", "error"
	Text string
	Done bool
}

// TextResult creates a simple text ToolResult.
func TextResult(text string) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

// ErrorResult creates an error ToolResult.
func ErrorResult(msg string) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{{Type: "text", Text: msg}},
		IsError: true,
	}
}
