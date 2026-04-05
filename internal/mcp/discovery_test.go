package mcp

import (
	"context"
	"strings"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeServerName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "myserver", "myserver"},
		{"uppercase", "MyServer", "myserver"},
		{"dots", "my.server.com", "my_server_com"},
		{"spaces", "My Server", "my_server"},
		{"special chars", "my@server/v2", "my_server_v2"},
		{"leading trailing underscores", ".server.", "server"},
		{"consecutive underscores collapsed", "my..server", "my_server"},
		{"hyphens preserved", "my-server", "my-server"},
		{"unicode replaced", "my\u00e9server", "my_server"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeServerName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNormalizeToolName(t *testing.T) {
	tests := []struct {
		name       string
		serverName string
		toolName   string
		expected   string
	}{
		{
			"basic",
			"github", "list_repos",
			"mcp__github__list_repos",
		},
		{
			"server with special chars",
			"My.Server", "doSomething",
			"mcp__my_server__dosomething",
		},
		{
			"tool with special chars",
			"server", "do.something/now",
			"mcp__server__do_something_now",
		},
		{
			"both need normalization",
			"My Server!", "Get Items!!",
			"mcp__my_server__get_items",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeToolName(tc.serverName, tc.toolName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRefreshDiscoveryCachesToolsResourcesPrompts(t *testing.T) {
	ctx := context.Background()

	// Create a test server with tools, resources, and prompts
	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "discovery-test", Version: "0.1.0"},
		&gomcp.ServerOptions{},
	)

	// Add a tool
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "list_files",
		Description: "Lists files in a directory",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args map[string]any) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{}, nil, nil
	})

	// Add a resource
	server.AddResource(
		&gomcp.Resource{
			URI:         "file:///test/readme.md",
			Name:        "readme",
			Title:       "README",
			Description: "Project readme file",
			MIMEType:    "text/markdown",
		},
		func(_ context.Context, req *gomcp.ReadResourceRequest) (*gomcp.ReadResourceResult, error) {
			return &gomcp.ReadResourceResult{
				Contents: []*gomcp.ResourceContents{
					{URI: req.Params.URI, MIMEType: "text/markdown", Text: "# Hello"},
				},
			}, nil
		},
	)

	// Add a prompt
	server.AddPrompt(
		&gomcp.Prompt{
			Name:        "summarize",
			Description: "Summarizes text content",
			Arguments: []*gomcp.PromptArgument{
				{Name: "text", Description: "The text to summarize", Required: true},
			},
		},
		func(_ context.Context, _ *gomcp.GetPromptRequest) (*gomcp.GetPromptResult, error) {
			return &gomcp.GetPromptResult{}, nil
		},
	)

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name: "Discovery Test",
		Type: TransportStdio,
	}, clientTransport)
	require.NoError(t, err)
	defer cs.Close()

	// Refresh discovery
	err = cs.RefreshDiscovery(ctx)
	require.NoError(t, err)

	// Verify tools were discovered and normalized
	tools := cs.DiscoveredTools()
	require.Len(t, tools, 1)
	assert.Equal(t, "list_files", tools[0].OriginalName)
	assert.Equal(t, "mcp__discovery_test__list_files", tools[0].NormalizedName)
	assert.Equal(t, "Lists files in a directory", tools[0].Description)

	// Verify resources were discovered
	resources, err := cs.ListResources(ctx)
	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Equal(t, "file:///test/readme.md", resources[0].URI)
	assert.Equal(t, "readme", resources[0].Name)
	assert.Equal(t, "README", resources[0].Title)
	assert.Equal(t, "text/markdown", resources[0].MIMEType)
	assert.Equal(t, "discovery_test", resources[0].ServerName)

	// Verify prompts were discovered
	prompts, err := cs.ListPrompts(ctx)
	require.NoError(t, err)
	require.Len(t, prompts, 1)
	assert.Equal(t, "summarize", prompts[0].OriginalName)
	assert.Equal(t, "mcp__discovery_test__summarize", prompts[0].NormalizedName)
	assert.Equal(t, "Summarizes text content", prompts[0].Description)
	require.Len(t, prompts[0].Arguments, 1)
	assert.Equal(t, "text", prompts[0].Arguments[0].Name)

	// Verify ReadResource works
	readResult, err := cs.ReadResource(ctx, "file:///test/readme.md")
	require.NoError(t, err)
	require.Len(t, readResult.Contents, 1)
	assert.Equal(t, "# Hello", readResult.Contents[0].Text)
}

func TestRefreshDiscoveryTruncatesDescriptions(t *testing.T) {
	ctx := context.Background()

	longDesc := strings.Repeat("x", MaxDiscoveryDescriptionLength+500)

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "truncate-test", Version: "0.1.0"},
		&gomcp.ServerOptions{},
	)

	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "verbose_tool",
		Description: longDesc,
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args map[string]any) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{}, nil, nil
	})

	server.AddPrompt(
		&gomcp.Prompt{
			Name:        "verbose_prompt",
			Description: longDesc,
		},
		func(_ context.Context, _ *gomcp.GetPromptRequest) (*gomcp.GetPromptResult, error) {
			return &gomcp.GetPromptResult{}, nil
		},
	)

	server.AddResource(
		&gomcp.Resource{
			URI:         "file:///big.txt",
			Name:        "big",
			Description: longDesc,
		},
		func(_ context.Context, _ *gomcp.ReadResourceRequest) (*gomcp.ReadResourceResult, error) {
			return &gomcp.ReadResourceResult{}, nil
		},
	)

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	go func() {
		_ = server.Run(ctx, serverTransport)
	}()

	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name: "truncate-test",
		Type: TransportStdio,
	}, clientTransport)
	require.NoError(t, err)
	defer cs.Close()

	err = cs.RefreshDiscovery(ctx)
	require.NoError(t, err)

	// Tool description should be truncated
	tools := cs.DiscoveredTools()
	require.Len(t, tools, 1)
	assert.Len(t, tools[0].Description, MaxDiscoveryDescriptionLength)

	// Prompt description should be truncated
	prompts, err := cs.ListPrompts(ctx)
	require.NoError(t, err)
	require.Len(t, prompts, 1)
	assert.Len(t, prompts[0].Description, MaxDiscoveryDescriptionLength)

	// Resource description should be truncated
	resources, err := cs.ListResources(ctx)
	require.NoError(t, err)
	require.Len(t, resources, 1)
	assert.Len(t, resources[0].Description, MaxDiscoveryDescriptionLength)
}
