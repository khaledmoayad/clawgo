package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallToolWithPolicyTimeout verifies that CallToolWithPolicy respects the
// configured timeout and cancels the call when it expires.
func TestCallToolWithPolicyTimeout(t *testing.T) {
	ctx := context.Background()

	// Create a server with a tool that sleeps forever (simulates a hanging tool).
	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "slow-server", Version: "0.1.0"},
		nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "hang",
		Description: "Hangs forever",
	}, func(ctx context.Context, _ *gomcp.CallToolRequest, _ map[string]any) (*gomcp.CallToolResult, any, error) {
		// Block until context is cancelled (simulates timeout).
		<-ctx.Done()
		return nil, nil, ctx.Err()
	})

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	go func() {
		server.Run(ctx, serverTransport)
	}()

	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name: "slow-server",
		Type: TransportStdio,
	}, clientTransport)
	require.NoError(t, err)
	defer cs.Close()

	// Use a very short timeout to trigger the timeout path.
	start := time.Now()
	_, err = cs.CallToolWithPolicy(ctx, "hang", nil, nil, ToolCallOptions{
		TimeoutMs: 100, // 100ms
	})
	elapsed := time.Since(start)

	require.Error(t, err)
	// The call should have timed out well under 2 seconds.
	assert.Less(t, elapsed, 2*time.Second, "call should timeout quickly")
}

// TestCallToolWithPolicyMetaPassthrough verifies that _meta is passed through
// to the MCP server and back in the result.
func TestCallToolWithPolicyMetaPassthrough(t *testing.T) {
	ctx := context.Background()

	// Create a server that echoes the _meta from the request into the result.
	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "meta-server", Version: "0.1.0"},
		nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "echo_meta",
		Description: "Echoes _meta back",
	}, func(_ context.Context, req *gomcp.CallToolRequest, _ map[string]any) (*gomcp.CallToolResult, any, error) {
		result := &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "ok"}},
		}
		// Copy request meta to result meta.
		if req.Params.Meta != nil {
			result.Meta = req.Params.Meta
		}
		return result, nil, nil
	})

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	go func() {
		server.Run(ctx, serverTransport)
	}()

	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name: "meta-server",
		Type: TransportStdio,
	}, clientTransport)
	require.NoError(t, err)
	defer cs.Close()

	meta := map[string]any{
		"requestId": "test-123",
		"custom":    "value",
	}

	result, err := cs.CallToolWithPolicy(ctx, "echo_meta", nil, meta, ToolCallOptions{})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify _meta is preserved in the result.
	require.NotNil(t, result.Meta, "_meta should be present in result")
	assert.Equal(t, "test-123", result.Meta["requestId"])
	assert.Equal(t, "value", result.Meta["custom"])
}

// TestCallToolWithPolicyProgress verifies that progress callbacks are invoked
// for started and completed states.
func TestCallToolWithPolicyProgress(t *testing.T) {
	ctx := context.Background()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "progress-server", Version: "0.1.0"},
		nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "fast_tool",
		Description: "Returns immediately",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, _ map[string]any) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "done"}},
		}, nil, nil
	})

	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	go func() {
		server.Run(ctx, serverTransport)
	}()

	cs, err := ConnectToServerWithTransport(ctx, MCPServerConfig{
		Name: "progress-server",
		Type: TransportStdio,
	}, clientTransport)
	require.NoError(t, err)
	defer cs.Close()

	var mu sync.Mutex
	var events []ProgressEvent

	result, err := cs.CallToolWithPolicy(ctx, "fast_tool", nil, nil, ToolCallOptions{
		OnProgress: func(e ProgressEvent) {
			mu.Lock()
			events = append(events, e)
			mu.Unlock()
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	mu.Lock()
	defer mu.Unlock()

	// Should have at least "started" and "completed".
	require.GreaterOrEqual(t, len(events), 2, "should have started + completed events")
	assert.Equal(t, "started", events[0].Status)
	assert.Equal(t, "progress-server", events[0].ServerName)
	assert.Equal(t, "fast_tool", events[0].ToolName)

	last := events[len(events)-1]
	assert.Equal(t, "completed", last.Status)
	assert.Greater(t, last.ElapsedMs, int64(-1), "elapsed should be non-negative")
}

// TestCallToolWithPolicyRetriesAfterElicitation verifies the retry loop when
// the MCP server returns -32042 (URL elicitation required).
func TestCallToolWithPolicyRetriesAfterElicitation(t *testing.T) {
	// This test uses the extract/handler path directly since we can't easily
	// make the Go MCP SDK server return a -32042 error. Instead, we verify
	// the helper functions and the overall flow contract.

	t.Run("extractElicitations parses valid entries", func(t *testing.T) {
		data := map[string]any{
			"elicitations": []any{
				map[string]any{
					"elicitationId": "e1",
					"url":           "https://example.com/auth",
					"message":       "Please authenticate",
				},
			},
		}
		elicitations := extractElicitations(data)
		require.Len(t, elicitations, 1)
		assert.Equal(t, "e1", elicitations[0].ElicitationID)
		assert.Equal(t, "https://example.com/auth", elicitations[0].URL)
		assert.Equal(t, "Please authenticate", elicitations[0].Message)
	})

	t.Run("extractElicitations ignores malformed entries", func(t *testing.T) {
		data := map[string]any{
			"elicitations": []any{
				map[string]any{
					"elicitationId": "e1",
					// missing url and message
				},
				"not-a-map",
				map[string]any{
					"elicitationId": "e2",
					"url":           "https://example.com",
					"message":       "Valid",
				},
			},
		}
		elicitations := extractElicitations(data)
		require.Len(t, elicitations, 1)
		assert.Equal(t, "e2", elicitations[0].ElicitationID)
	})

	t.Run("extractElicitations returns nil for missing data", func(t *testing.T) {
		assert.Nil(t, extractElicitations(nil))
		assert.Nil(t, extractElicitations(map[string]any{}))
	})

	t.Run("isAuthError detects 401", func(t *testing.T) {
		err := fmt.Errorf("request failed: status 401 Unauthorized")
		assert.True(t, isAuthError(err))
	})

	t.Run("isAuthError detects 403", func(t *testing.T) {
		err := fmt.Errorf("HTTP 403 Forbidden")
		assert.True(t, isAuthError(err))
	})

	t.Run("isAuthError returns false for other errors", func(t *testing.T) {
		err := fmt.Errorf("connection refused")
		assert.False(t, isAuthError(err))
	})

	t.Run("MCPError code ElicitationRequiredCode", func(t *testing.T) {
		mcpErr := &MCPError{
			Code:    ElicitationRequiredCode,
			Message: "URL elicitation required",
			Data: map[string]any{
				"elicitations": []any{
					map[string]any{
						"elicitationId": "e1",
						"url":           "https://example.com",
						"message":       "Auth needed",
					},
				},
			},
		}
		assert.Equal(t, -32042, mcpErr.Code)
		extracted, ok := asMCPError(mcpErr)
		require.True(t, ok)
		assert.Equal(t, ElicitationRequiredCode, extracted.Code)
	})
}

// TestNormalizeCallToolResultTruncatesLargeText verifies that text content
// exceeding MaxMCPResultChars is truncated with a suffix.
func TestNormalizeCallToolResultTruncatesLargeText(t *testing.T) {
	// Build a text content block that exceeds the limit.
	bigText := strings.Repeat("x", MaxMCPResultChars+1000)

	result := &gomcp.CallToolResult{
		Content: []gomcp.Content{
			&gomcp.TextContent{Text: bigText},
		},
	}

	normalized, err := NormalizeCallToolResult(result)
	require.NoError(t, err)
	require.Len(t, normalized.Content, 1)

	tc, ok := normalized.Content[0].(*gomcp.TextContent)
	require.True(t, ok)

	// The text should be truncated to MaxMCPResultChars + the suffix.
	assert.True(t, len(tc.Text) < len(bigText), "text should be shorter than original")
	assert.True(t, len(tc.Text) > MaxMCPResultChars, "text should include truncation suffix")
	assert.Contains(t, tc.Text, "[OUTPUT TRUNCATED")

	// A text under the limit should pass through unchanged.
	smallText := "hello world"
	result2 := &gomcp.CallToolResult{
		Content: []gomcp.Content{&gomcp.TextContent{Text: smallText}},
	}
	normalized2, err := NormalizeCallToolResult(result2)
	require.NoError(t, err)
	tc2, ok := normalized2.Content[0].(*gomcp.TextContent)
	require.True(t, ok)
	assert.Equal(t, smallText, tc2.Text)
}

// TestNormalizeCallToolResultPreservesMeta verifies that _meta on the result
// survives normalization.
func TestNormalizeCallToolResultPreservesMeta(t *testing.T) {
	result := &gomcp.CallToolResult{
		Meta: gomcp.Meta{"key": "value", "nested": map[string]any{"a": 1}},
		Content: []gomcp.Content{
			&gomcp.TextContent{Text: "ok"},
		},
	}

	normalized, err := NormalizeCallToolResult(result)
	require.NoError(t, err)
	require.NotNil(t, normalized.Meta)
	assert.Equal(t, "value", normalized.Meta["key"])

	nested, ok := normalized.Meta["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, nested["a"])
}

// TestNormalizeCallToolResultConvertsUnsupportedContent verifies that
// unsupported content types produce human-readable text summaries.
func TestNormalizeCallToolResultConvertsUnsupportedContent(t *testing.T) {
	t.Run("audio content becomes text summary", func(t *testing.T) {
		result := &gomcp.CallToolResult{
			Content: []gomcp.Content{
				&gomcp.AudioContent{
					Data:     []byte("fake-audio-data"),
					MIMEType: "audio/mp3",
				},
			},
		}

		normalized, err := NormalizeCallToolResult(result)
		require.NoError(t, err)
		require.Len(t, normalized.Content, 1)

		tc, ok := normalized.Content[0].(*gomcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, tc.Text, "Audio content")
		assert.Contains(t, tc.Text, "audio/mp3")
	})

	t.Run("nil result passes through", func(t *testing.T) {
		result, err := NormalizeCallToolResult(nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("isError flag preserved", func(t *testing.T) {
		result := &gomcp.CallToolResult{
			IsError: true,
			Content: []gomcp.Content{&gomcp.TextContent{Text: "error details"}},
		}
		normalized, err := NormalizeCallToolResult(result)
		require.NoError(t, err)
		assert.True(t, normalized.IsError)
	})

	t.Run("structuredContent preserved", func(t *testing.T) {
		sc := map[string]any{"key": "value"}
		result := &gomcp.CallToolResult{
			Content:           []gomcp.Content{&gomcp.TextContent{Text: "ok"}},
			StructuredContent: sc,
		}
		normalized, err := NormalizeCallToolResult(result)
		require.NoError(t, err)
		assert.Equal(t, sc, normalized.StructuredContent)
	})

	t.Run("mixed content types", func(t *testing.T) {
		result := &gomcp.CallToolResult{
			Content: []gomcp.Content{
				&gomcp.TextContent{Text: "text part"},
				&gomcp.AudioContent{Data: []byte("audio"), MIMEType: "audio/wav"},
			},
		}
		normalized, err := NormalizeCallToolResult(result)
		require.NoError(t, err)
		require.Len(t, normalized.Content, 2)

		tc1, ok := normalized.Content[0].(*gomcp.TextContent)
		require.True(t, ok)
		assert.Equal(t, "text part", tc1.Text)

		tc2, ok := normalized.Content[1].(*gomcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, tc2.Text, "Audio content")
	})
}
