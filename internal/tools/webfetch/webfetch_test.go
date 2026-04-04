package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeInput(t *testing.T, url, prompt string) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(map[string]string{"url": url, "prompt": prompt})
	require.NoError(t, err)
	return b
}

func TestWebFetchToolMetadata(t *testing.T) {
	tool := New()

	t.Run("Name returns WebFetch", func(t *testing.T) {
		assert.Equal(t, "WebFetch", tool.Name())
	})

	t.Run("IsReadOnly returns true", func(t *testing.T) {
		assert.True(t, tool.IsReadOnly())
	})

	t.Run("IsConcurrencySafe returns true", func(t *testing.T) {
		assert.True(t, tool.IsConcurrencySafe(nil))
	})

	t.Run("Description is non-empty", func(t *testing.T) {
		assert.NotEmpty(t, tool.Description())
	})

	t.Run("InputSchema is valid JSON", func(t *testing.T) {
		var schema map[string]any
		err := json.Unmarshal(tool.InputSchema(), &schema)
		assert.NoError(t, err)
		props, ok := schema["properties"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, props, "url")
		assert.Contains(t, props, "prompt")
	})
}

func TestWebFetchToolCallValid(t *testing.T) {
	// Create a test server returning HTML
	htmlContent := `<!DOCTYPE html>
<html><head><title>Test Page</title></head>
<body>
<h1>Hello World</h1>
<p>This is a test paragraph.</p>
<ul>
<li>Item 1</li>
<li>Item 2</li>
</ul>
<a href="https://example.com">Example Link</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlContent)
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}

	input := makeInput(t, server.URL, "Extract all content")
	result, err := tool.Call(ctx, input, toolCtx)

	require.NoError(t, err)
	assert.False(t, result.IsError)

	// The result text should contain the markdown-converted content
	text := result.Content[0].Text
	assert.Contains(t, text, "Hello World")
	assert.Contains(t, text, "test paragraph")
	assert.Contains(t, text, "Item 1")
	assert.Contains(t, text, "Item 2")
	assert.Contains(t, text, "Example Link")
}

func TestWebFetchToolHTMLToMarkdown(t *testing.T) {
	htmlContent := `<html><body>
<h1>Title</h1>
<p>Paragraph with <code>inline code</code>.</p>
<a href="https://example.com">A link</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, htmlContent)
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "Get content")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := result.Content[0].Text
	// Should contain markdown formatted output
	assert.Contains(t, text, "Title")
	assert.Contains(t, text, "inline code")
	assert.Contains(t, text, "A link")
}

func TestWebFetchToolInvalidURL(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}

	t.Run("no scheme", func(t *testing.T) {
		input := makeInput(t, "example.com", "test")
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "http://")
	})

	t.Run("ftp scheme", func(t *testing.T) {
		input := makeInput(t, "ftp://example.com", "test")
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestWebFetchToolTimeout(t *testing.T) {
	// Create a server that delays longer than our context timeout.
	// Use a channel to unblock the handler promptly when the test finishes.
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
		}
	}))
	defer func() {
		close(done)
		server.Close()
	}()

	tool := New()
	// Use a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "test")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	// Should mention timeout or context
	text := strings.ToLower(result.Content[0].Text)
	assert.True(t, strings.Contains(text, "timeout") || strings.Contains(text, "deadline") || strings.Contains(text, "context") || strings.Contains(text, "cancel"))
}

func TestWebFetchToolHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Not Found")
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "test")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "404")
}

func TestWebFetchToolTruncation(t *testing.T) {
	// Generate content larger than MaxResponseSize
	bigContent := strings.Repeat("x", MaxResponseSize+1000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, bigContent)
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "test")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := result.Content[0].Text
	// The truncation message marker must appear
	assert.Contains(t, text, "[truncated")
	// The body portion should be no larger than MaxResponseSize; total text includes headers
	assert.Less(t, len(text), MaxResponseSize+2048, "response should be truncated")
}

func TestWebFetchToolRedirect(t *testing.T) {
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "Final destination")
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusFound)
	}))
	defer redirectServer.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, redirectServer.URL, "test")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Final destination")
}

func TestWebFetchToolUserAgent(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "test")

	_, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.Contains(t, receivedUA, "ClawGo")
}

func TestWebFetchToolPlainText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "Plain text content")
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "test")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "Plain text content")
}

func TestWebFetchToolJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"key": "value"}`)
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "test")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, `"key"`)
	assert.Contains(t, result.Content[0].Text, `"value"`)
}

func TestWebFetchToolMissingFields(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}

	t.Run("missing url", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"prompt": "test"})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})

	t.Run("missing prompt", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"url": "https://example.com"})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestWebFetchToolCheckPermissions(t *testing.T) {
	tool := New()
	result, err := tool.CheckPermissions(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, tools.PermissionAllow, result)
}
