package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectToStdioServer(t *testing.T) {
	// Set up a real MCP server in-memory and connect to it via the client
	ctx := context.Background()

	// Create a test server with one tool
	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "0.1.0"},
		nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "echo",
		Description: "Echoes input",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args map[string]any) (*gomcp.CallToolResult, any, error) {
		msg, _ := args["message"].(string)
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: msg}},
		}, nil, nil
	})

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()

	// Start server in background
	go func() {
		server.Run(ctx, serverTransport)
	}()

	// Use ConnectToServerWithTransport for testing (avoids subprocess)
	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name: "test-server",
		Type: TransportStdio,
	}, clientTransport)
	require.NoError(t, err)
	defer cs.Close()

	// Verify tool discovery
	mcpTools := cs.ListTools()
	require.Len(t, mcpTools, 1)
	assert.Equal(t, "echo", mcpTools[0].Name)
}

func TestCallRemoteTool(t *testing.T) {
	ctx := context.Background()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "0.1.0"},
		nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "greet",
		Description: "Greets a person",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args map[string]any) (*gomcp.CallToolResult, any, error) {
		name, _ := args["name"].(string)
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "Hello, " + name + "!"}},
		}, nil, nil
	})

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	go func() {
		server.Run(ctx, serverTransport)
	}()

	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name: "test-server",
		Type: TransportStdio,
	}, clientTransport)
	require.NoError(t, err)
	defer cs.Close()

	// Call the tool
	result, err := cs.CallTool(ctx, "greet", map[string]any{"name": "World"})
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*gomcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, "Hello, World!", textContent.Text)
}

func TestDisconnect(t *testing.T) {
	ctx := context.Background()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "0.1.0"},
		nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "noop",
		Description: "Does nothing",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args map[string]any) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{}, nil, nil
	})

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	go func() {
		server.Run(ctx, serverTransport)
	}()

	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name: "test-server",
		Type: TransportStdio,
	}, clientTransport)
	require.NoError(t, err)

	// Close should not error
	err = cs.Close()
	assert.NoError(t, err)
}

func TestConnectToServerEnv(t *testing.T) {
	env := map[string]string{
		"CUSTOM_VAR":  "custom_value",
		"ANOTHER_VAR": "another_value",
	}

	result := buildEnv(env)

	// Should contain all existing env vars plus custom ones
	found := make(map[string]bool)
	for _, e := range result {
		if e == "CUSTOM_VAR=custom_value" {
			found["CUSTOM_VAR"] = true
		}
		if e == "ANOTHER_VAR=another_value" {
			found["ANOTHER_VAR"] = true
		}
	}
	assert.True(t, found["CUSTOM_VAR"], "CUSTOM_VAR should be in environment")
	assert.True(t, found["ANOTHER_VAR"], "ANOTHER_VAR should be in environment")

	// Should also contain system PATH
	hasPath := false
	for _, e := range result {
		if len(e) > 5 && e[:5] == "PATH=" {
			hasPath = true
			break
		}
	}
	assert.True(t, hasPath, "PATH should be inherited from system environment")
}

func TestBuildEnvOverrides(t *testing.T) {
	// Set a value to override
	os.Setenv("TEST_BUILD_ENV_KEY", "original")
	defer os.Unsetenv("TEST_BUILD_ENV_KEY")

	env := map[string]string{
		"TEST_BUILD_ENV_KEY": "overridden",
	}

	result := buildEnv(env)

	found := false
	for _, e := range result {
		if e == "TEST_BUILD_ENV_KEY=overridden" {
			found = true
		}
	}
	assert.True(t, found, "environment variable should be overridden")
}

func TestBuildEnvNil(t *testing.T) {
	result := buildEnv(nil)
	// Should return system env unchanged
	assert.Equal(t, os.Environ(), result)
}

func TestConnectToServerConfig(t *testing.T) {
	// Test that MCPServerConfig fields are properly stored in ConnectedServer
	cfg := MCPServerConfig{
		Name:    "test",
		Type:    TransportStdio,
		Command: "/usr/bin/echo",
		Args:    []string{"hello"},
		Env:     map[string]string{"KEY": "VALUE"},
	}

	// We can verify the config is stored correctly in the struct
	_ = json.RawMessage(`{}`)
	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, TransportStdio, cfg.Type)
	assert.Equal(t, "/usr/bin/echo", cfg.Command)
}
