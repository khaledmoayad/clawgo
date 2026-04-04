package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/khaledmoayad/clawgo/internal/tools"
)

// StartServer creates an MCP server that exposes all tools from the given
// registry and runs it on stdio. This is invoked by `clawgo mcp serve`.
//
// CRITICAL: All logging goes to stderr to avoid corrupting the JSON-RPC
// stream on stdout.
func StartServer(ctx context.Context, registry *tools.Registry, version string) error {
	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "claude/tengu", Version: version},
		nil,
	)

	registerTools(server, registry)

	// Run on stdio transport
	return server.Run(ctx, &gomcp.StdioTransport{})
}

// registerTools registers all tools from a ClawGo tools.Registry into an MCP
// server. Each tool is wrapped with a handler that delegates to the underlying
// Tool.Call method.
func registerTools(server *gomcp.Server, registry *tools.Registry) {
	for _, t := range registry.All() {
		mcpTool, handler := convertToolToMCP(t)
		server.AddTool(mcpTool, handler)
	}
}

// convertRegistryToMCPTools converts all tools in a registry to MCP tool
// definitions. Used for testing tool registration logic in isolation.
func convertRegistryToMCPTools(registry *tools.Registry) []*gomcp.Tool {
	result := make([]*gomcp.Tool, 0, len(registry.All()))
	for _, t := range registry.All() {
		mcpTool, _ := convertToolToMCP(t)
		result = append(result, mcpTool)
	}
	return result
}

// convertToolToMCP converts a single ClawGo tool to an MCP tool definition
// and its corresponding handler function.
func convertToolToMCP(t tools.Tool) (*gomcp.Tool, gomcp.ToolHandler) {
	// Parse the JSON Schema from the tool's InputSchema
	var schemaMap map[string]any
	if err := json.Unmarshal(t.InputSchema(), &schemaMap); err != nil {
		// Fallback to empty object schema if parsing fails
		schemaMap = map[string]any{"type": "object"}
	}

	// Ensure the schema has type "object" (required by MCP SDK)
	if _, ok := schemaMap["type"]; !ok {
		schemaMap["type"] = "object"
	}

	mcpTool := &gomcp.Tool{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: json.RawMessage(t.InputSchema()),
		Annotations: &gomcp.ToolAnnotations{
			ReadOnlyHint: t.IsReadOnly(),
		},
	}

	// The handler receives the raw CallToolRequest and delegates to Tool.Call
	handler := func(ctx context.Context, req *gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		// Extract arguments as json.RawMessage
		argsJSON, err := json.Marshal(req.Params.Arguments)
		if err != nil {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: fmt.Sprintf("failed to marshal arguments: %v", err)}},
				IsError: true,
			}, nil
		}

		// Build a minimal ToolUseContext for MCP server mode
		wd, _ := os.Getwd()
		toolCtx := &tools.ToolUseContext{
			WorkingDir:  wd,
			ProjectRoot: wd,
		}

		// Call the underlying tool
		result, err := t.Call(ctx, argsJSON, toolCtx)
		if err != nil {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: fmt.Sprintf("tool error: %v", err)}},
				IsError: true,
			}, nil
		}

		return toolResultToMCP(result), nil
	}

	return mcpTool, handler
}

// toolResultToMCP converts a ClawGo ToolResult to an MCP CallToolResult.
func toolResultToMCP(result *tools.ToolResult) *gomcp.CallToolResult {
	content := make([]gomcp.Content, 0, len(result.Content))
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			content = append(content, &gomcp.TextContent{Text: block.Text})
		default:
			// For unsupported types, fall back to text
			content = append(content, &gomcp.TextContent{Text: block.Text})
		}
	}
	return &gomcp.CallToolResult{
		Content: content,
		IsError: result.IsError,
	}
}

