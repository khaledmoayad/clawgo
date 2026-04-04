package tools

import (
	"context"
	"encoding/json"
)

// Tool defines the contract for all tools Claude can invoke.
// Mirrors the TypeScript Tool.ts buildTool() pattern.
type Tool interface {
	// Name returns the tool's unique identifier (e.g., "bash", "file_read").
	Name() string

	// Description returns a human-readable description for the API.
	Description() string

	// InputSchema returns the JSON Schema for tool input, sent to the Anthropic API.
	InputSchema() json.RawMessage

	// IsReadOnly indicates whether the tool only reads data (no side effects).
	// Read-only tools are auto-approved in default permission mode.
	IsReadOnly() bool

	// Call executes the tool with the given input and context.
	Call(ctx context.Context, input json.RawMessage, toolCtx *ToolUseContext) (*ToolResult, error)

	// IsConcurrencySafe indicates whether the tool can safely execute concurrently
	// with other tools in the same batch. Read-only tools typically return true;
	// tools that modify state (filesystem, processes) return false.
	// The input parameter allows input-dependent classification (e.g., bash
	// command analysis in future plans).
	IsConcurrencySafe(input json.RawMessage) bool

	// CheckPermissions determines whether the tool should be allowed, denied, or require user prompt.
	CheckPermissions(ctx context.Context, input json.RawMessage, permCtx *PermissionContext) (PermissionResult, error)
}

// ToolDefinition is the API-facing representation sent to the Anthropic API.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// Registry maps tool names to implementations.
type Registry struct {
	tools map[string]Tool
	order []string // preserves registration order
}

// NewRegistry creates a registry and registers the given tools.
func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{
		tools: make(map[string]Tool, len(tools)),
		order: make([]string, 0, len(tools)),
	}
	for _, t := range tools {
		r.tools[t.Name()] = t
		r.order = append(r.order, t.Name())
	}
	return r
}

// Get returns a tool by name, or nil and false if not found.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// All returns all registered tools in registration order.
func (r *Registry) All() []Tool {
	result := make([]Tool, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.tools[name])
	}
	return result
}

// ToolDefinitions returns API-compatible definitions for all registered tools.
func (r *Registry) ToolDefinitions() []ToolDefinition {
	defs := make([]ToolDefinition, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		defs = append(defs, ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return defs
}

// Register adds a tool to the registry after construction.
// This is useful for tools that need a registry reference (e.g., AgentTool, ToolSearch).
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
	r.order = append(r.order, t.Name())
}

// Names returns all registered tool names in registration order.
func (r *Registry) Names() []string {
	return append([]string{}, r.order...)
}
