package mcp

import (
	"context"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisabledServer(t *testing.T) {
	ctx := context.Background()
	cs, err := ConnectToServer(ctx, MCPServerConfig{
		Name:     "disabled-server",
		Type:     TransportStdio,
		Command:  "echo",
		Disabled: true,
	})
	require.NoError(t, err)
	assert.Equal(t, StatusDisabled, cs.Status)
	assert.Nil(t, cs.session)
	assert.NoError(t, cs.Close())
}

func TestConnectionStatus(t *testing.T) {
	ctx := context.Background()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "0.1.0"},
		nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "ping",
		Description: "Returns pong",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args map[string]any) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "pong"}},
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

	// Should be connected
	assert.Equal(t, StatusConnected, cs.Status)
}

func TestNormalizedToolNames(t *testing.T) {
	ctx := context.Background()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "0.1.0"},
		nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "create_issue",
		Description: "Creates an issue",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args map[string]any) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{}, nil, nil
	})
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "list_repos",
		Description: "Lists repos",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args map[string]any) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{}, nil, nil
	})

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	go func() {
		server.Run(ctx, serverTransport)
	}()

	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name: "github",
		Type: TransportStdio,
	}, clientTransport)
	require.NoError(t, err)
	defer cs.Close()

	// Check normalized names exist
	names := cs.NormalizedToolNames()
	assert.Equal(t, "create_issue", names["mcp__github__create_issue"])
	assert.Equal(t, "list_repos", names["mcp__github__list_repos"])

	// Check OriginalToolName lookup
	assert.Equal(t, "create_issue", cs.OriginalToolName("mcp__github__create_issue"))
	assert.Equal(t, "", cs.OriginalToolName("mcp__github__nonexistent"))
}

func TestToolCallTimeout(t *testing.T) {
	ctx := context.Background()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "0.1.0"},
		nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "fast",
		Description: "Returns quickly",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args map[string]any) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "done"}},
		}, nil, nil
	})

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	go func() {
		server.Run(ctx, serverTransport)
	}()

	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name:      "test-server",
		Type:      TransportStdio,
		TimeoutMS: 5000, // 5 second timeout
	}, clientTransport)
	require.NoError(t, err)
	defer cs.Close()

	// Fast call should succeed
	result, err := cs.CallTool(ctx, "fast", nil)
	require.NoError(t, err)
	require.Len(t, result.Content, 1)
}

func TestStderrCapture(t *testing.T) {
	// ConnectedServer with stderrBuf should return captured output
	cs := &ConnectedServer{
		Config: MCPServerConfig{Name: "test"},
		Status: StatusConnected,
	}
	assert.Equal(t, "", cs.Stderr())
}

func TestReconnectMaxAttempts(t *testing.T) {
	cs := &ConnectedServer{
		Config: MCPServerConfig{
			Name:                 "test-server",
			Type:                 TransportStdio,
			Command:              "nonexistent-command",
			MaxReconnectAttempts: 2,
		},
		Status:           StatusConnected,
		reconnectAttempt: 2, // already at max
	}

	err := cs.Reconnect(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "max reconnect attempts")
	assert.Equal(t, StatusFailed, cs.Status)
}

func TestConfigScopeTypes(t *testing.T) {
	// Verify all scope constants exist
	scopes := []ConfigScope{
		ScopeLocal, ScopeUser, ScopeProject,
		ScopeDynamic, ScopeEnterprise, ScopeClaudeAI, ScopeManaged,
	}
	for _, s := range scopes {
		assert.NotEmpty(t, string(s))
	}
}

func TestConnectionStatusTypes(t *testing.T) {
	// Verify all status constants exist
	statuses := []ConnectionStatus{
		StatusConnected, StatusFailed, StatusNeedsAuth,
		StatusPending, StatusDisabled,
	}
	for _, s := range statuses {
		assert.NotEmpty(t, string(s))
	}
}

func TestTransportTypes(t *testing.T) {
	// Verify all transport type constants
	assert.Equal(t, MCPTransportType("stdio"), TransportStdio)
	assert.Equal(t, MCPTransportType("sse"), TransportSSE)
	assert.Equal(t, MCPTransportType("http"), TransportHTTP)
	assert.Equal(t, MCPTransportType("ws"), TransportWebSocket)
}

func TestConnectSSERequiresURL(t *testing.T) {
	ctx := context.Background()
	_, err := ConnectToServer(ctx, MCPServerConfig{
		Name: "test",
		Type: TransportSSE,
		URL:  "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a URL")
}

func TestConnectHTTPRequiresURL(t *testing.T) {
	ctx := context.Background()
	_, err := ConnectToServer(ctx, MCPServerConfig{
		Name: "test",
		Type: TransportHTTP,
		URL:  "",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a URL")
}

func TestConnectWebSocketNotSupported(t *testing.T) {
	ctx := context.Background()
	_, err := ConnectToServer(ctx, MCPServerConfig{
		Name: "test",
		Type: TransportWebSocket,
		URL:  "ws://localhost:8080",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet supported")
}

func TestConnectUnknownTransport(t *testing.T) {
	ctx := context.Background()
	_, err := ConnectToServer(ctx, MCPServerConfig{
		Name: "test",
		Type: MCPTransportType("unknown"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown transport type")
}

func TestConstants(t *testing.T) {
	// Verify constants match Claude Code values
	assert.Equal(t, 100_000_000, DefaultMCPToolTimeoutMS)
	assert.Equal(t, 2048, MaxMCPDescriptionLength)
	assert.Equal(t, 5, DefaultMaxReconnectAttempts)
}
