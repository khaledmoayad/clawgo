// Package websearch implements the WebSearchTool which uses Anthropic's
// server-side web search capability. The actual search is performed by the
// Anthropic API when this tool is included as a server_tool in the tools array.
// The local Call() method returns an informational message.
package websearch

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

func (in *input) Validate() error {
	if strings.TrimSpace(in.Query) == "" {
		return fmt.Errorf("required field \"query\" is missing or empty")
	}
	return nil
}

// WebSearchTool provides web search via Anthropic's server-side search API.
// In the TS original this is BetaWebSearchTool20250305, sent as a "server_tool"
// type in the API tools array. The API performs the search and returns results
// directly — no local execution is needed.
type WebSearchTool struct {
	// IsServerSide indicates this tool is handled server-side by the Anthropic API.
	// When building the tools array for the API request, tools with this flag
	// are sent as type "server_tool" rather than "custom" tool definitions.
	IsServerSide bool
}

// New creates a new WebSearchTool.
func New() *WebSearchTool {
	return &WebSearchTool{IsServerSide: true}
}

func (t *WebSearchTool) Name() string                { return "WebSearch" }
func (t *WebSearchTool) Description() string          { return ToolDescription() }
func (t *WebSearchTool) IsReadOnly() bool             { return true }
func (t *WebSearchTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true — search requests are independent and stateless.
func (t *WebSearchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// CheckPermissions always allows — search is a read-only operation.
func (t *WebSearchTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

// Call returns an informational message. In production, this tool is handled
// server-side by the Anthropic API. The local Call() exists to satisfy the
// Tool interface and is invoked only if the tool is called outside of the
// normal API flow.
func (t *WebSearchTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return &tools.ToolResult{
		Content: []tools.ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Web search is handled server-side by the Anthropic API. Query: %q", in.Query),
		}},
		Metadata: map[string]any{
			"server_tool": true,
			"query":       in.Query,
		},
	}, nil
}
