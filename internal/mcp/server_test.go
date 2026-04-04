package mcp

import (
	"context"
	"encoding/json"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/khaledmoayad/clawgo/internal/tools"
)

// mockTool implements tools.Tool for testing.
type mockTool struct {
	name        string
	description string
	schema      json.RawMessage
	readOnly    bool
	callResult  *tools.ToolResult
	callErr     error
}

func (m *mockTool) Name() string                    { return m.name }
func (m *mockTool) Description() string              { return m.description }
func (m *mockTool) InputSchema() json.RawMessage     { return m.schema }
func (m *mockTool) IsReadOnly() bool                 { return m.readOnly }
func (m *mockTool) IsConcurrencySafe(json.RawMessage) bool { return m.readOnly }
func (m *mockTool) Call(ctx context.Context, input json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	return m.callResult, m.callErr
}
func (m *mockTool) CheckPermissions(ctx context.Context, input json.RawMessage, permCtx *tools.PermissionContext) (tools.PermissionResult, error) {
	return tools.PermissionAllow, nil
}

func TestMCPServerRegistersTools(t *testing.T) {
	// Create mock tools
	schema1 := json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`)
	schema2 := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)

	tool1 := &mockTool{name: "bash", description: "Execute bash commands", schema: schema1}
	tool2 := &mockTool{name: "file_read", description: "Read a file", schema: schema2, readOnly: true}

	registry := tools.NewRegistry(tool1, tool2)

	// Convert tools and verify
	mcpTools := convertRegistryToMCPTools(registry)

	assert.Len(t, mcpTools, 2)

	// Verify first tool
	assert.Equal(t, "bash", mcpTools[0].Name)
	assert.Equal(t, "Execute bash commands", mcpTools[0].Description)
	// Verify the input schema was properly set
	schemaJSON, err := json.Marshal(mcpTools[0].InputSchema)
	require.NoError(t, err)
	assert.Contains(t, string(schemaJSON), `"type":"object"`)

	// Verify second tool
	assert.Equal(t, "file_read", mcpTools[1].Name)
	assert.Equal(t, "Read a file", mcpTools[1].Description)
}

func TestMCPServerToolCall(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`)
	result := &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: "hello world"}},
		IsError: false,
	}
	tool := &mockTool{
		name:        "test_tool",
		description: "A test tool",
		schema:      schema,
		callResult:  result,
	}
	registry := tools.NewRegistry(tool)

	// Test using in-memory transport (client-server pair)
	ctx := context.Background()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "0.1.0"},
		nil,
	)
	registerTools(server, registry)

	// Use in-memory transport to test
	serverTransport, clientTransport := gomcp.NewInMemoryTransports()

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, serverTransport)
	}()

	// Connect client
	client := gomcp.NewClient(
		&gomcp.Implementation{Name: "test-client", Version: "0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer session.Close()

	// List tools and verify
	listResult, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, listResult.Tools, 1)
	assert.Equal(t, "test_tool", listResult.Tools[0].Name)

	// Call the tool
	callResult, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      "test_tool",
		Arguments: map[string]any{"command": "echo hello"},
	})
	require.NoError(t, err)
	require.Len(t, callResult.Content, 1)
	textContent, ok := callResult.Content[0].(*gomcp.TextContent)
	require.True(t, ok, "expected TextContent, got %T", callResult.Content[0])
	assert.Equal(t, "hello world", textContent.Text)
	assert.False(t, callResult.IsError)
}

func TestMCPServerToolCallError(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`)
	result := &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: "command not found"}},
		IsError: true,
	}
	tool := &mockTool{
		name:        "error_tool",
		description: "A tool that returns errors",
		schema:      schema,
		callResult:  result,
	}
	registry := tools.NewRegistry(tool)

	ctx := context.Background()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "0.1.0"},
		nil,
	)
	registerTools(server, registry)

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()

	go func() {
		server.Run(ctx, serverTransport)
	}()

	client := gomcp.NewClient(
		&gomcp.Implementation{Name: "test-client", Version: "0.1.0"},
		nil,
	)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer session.Close()

	callResult, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      "error_tool",
		Arguments: map[string]any{"command": "nonexistent"},
	})
	require.NoError(t, err)
	assert.True(t, callResult.IsError)
	require.Len(t, callResult.Content, 1)
	textContent, ok := callResult.Content[0].(*gomcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "command not found", textContent.Text)
}

func TestLoadMCPConfig(t *testing.T) {
	t.Run("parses map format", func(t *testing.T) {
		configJSON := json.RawMessage(`{
			"mcpServers": {
				"filesystem": {
					"type": "stdio",
					"command": "npx",
					"args": ["-y", "@modelcontextprotocol/server-filesystem"],
					"env": {"HOME": "/tmp"}
				},
				"web": {
					"type": "sse",
					"url": "https://example.com/mcp",
					"headers": {"Authorization": "Bearer token123"}
				}
			}
		}`)

		configs, err := LoadMCPConfig(configJSON)
		require.NoError(t, err)
		require.Len(t, configs, 2)

		// Find the filesystem config
		var fsCfg, webCfg MCPServerConfig
		for _, c := range configs {
			if c.Name == "filesystem" {
				fsCfg = c
			}
			if c.Name == "web" {
				webCfg = c
			}
		}

		assert.Equal(t, "filesystem", fsCfg.Name)
		assert.Equal(t, MCPTransportType("stdio"), fsCfg.Type)
		assert.Equal(t, "npx", fsCfg.Command)
		assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem"}, fsCfg.Args)
		assert.Equal(t, map[string]string{"HOME": "/tmp"}, fsCfg.Env)

		assert.Equal(t, "web", webCfg.Name)
		assert.Equal(t, MCPTransportType("sse"), webCfg.Type)
		assert.Equal(t, "https://example.com/mcp", webCfg.URL)
		assert.Equal(t, map[string]string{"Authorization": "Bearer token123"}, webCfg.Headers)
	})

	t.Run("returns empty slice when no config", func(t *testing.T) {
		configs, err := LoadMCPConfig(json.RawMessage(`{}`))
		require.NoError(t, err)
		assert.Empty(t, configs)
	})

	t.Run("returns empty slice for null", func(t *testing.T) {
		configs, err := LoadMCPConfig(json.RawMessage(`{"mcpServers": null}`))
		require.NoError(t, err)
		assert.Empty(t, configs)
	})
}
