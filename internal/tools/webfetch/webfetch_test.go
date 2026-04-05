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
	// Generate content larger than MAX_MARKDOWN_LENGTH to test content-level truncation
	bigContent := strings.Repeat("x", MAX_MARKDOWN_LENGTH+1000)
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
	assert.Contains(t, text, "[Content truncated due to length...]")
	// The body portion should be bounded by MAX_MARKDOWN_LENGTH plus header/footer text
	assert.Less(t, len(text), MAX_MARKDOWN_LENGTH+2048, "response should be truncated")
}

func TestWebFetchToolRedirect(t *testing.T) {
	// Same-host redirect: path redirect within the same server (same host:port)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "Final destination")
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL+"/redirect", "test")

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

// ---- New tests for caching, preapproved hosts, and extraction ----

func TestURLCacheTTL(t *testing.T) {
	cache := NewURLCache()

	entry := &cacheEntry{
		content:     "cached content",
		contentType: "text/plain",
		statusCode:  200,
		finalURL:    "https://example.com",
		fetchedAt:   time.Now(),
	}
	cache.Set("https://example.com", entry)

	// Should be retrievable immediately
	got, ok := cache.Get("https://example.com")
	assert.True(t, ok)
	assert.Equal(t, "cached content", got.content)

	// Simulate expired entry by setting fetchedAt far in the past
	entry.fetchedAt = time.Now().Add(-CACHE_TTL - time.Second)
	cache.mu.Lock()
	cache.entries["https://example.com"] = entry
	cache.mu.Unlock()

	// Should be a miss after TTL expires
	got, ok = cache.Get("https://example.com")
	assert.False(t, ok)
	assert.Nil(t, got)

	// Entry should have been removed (lazy eviction)
	assert.Equal(t, 0, cache.Len())
}

func TestURLCacheEviction(t *testing.T) {
	cache := NewURLCache()

	// Fill cache past MAX_CACHE_ENTRIES
	for i := 0; i < MAX_CACHE_ENTRIES+5; i++ {
		url := fmt.Sprintf("https://example.com/%d", i)
		cache.Set(url, &cacheEntry{
			content:   fmt.Sprintf("content-%d", i),
			fetchedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
		})
	}

	// Cache should not exceed MAX_CACHE_ENTRIES
	assert.LessOrEqual(t, cache.Len(), MAX_CACHE_ENTRIES)

	// The oldest entries (0-4) should have been evicted
	for i := 0; i < 5; i++ {
		url := fmt.Sprintf("https://example.com/%d", i)
		_, ok := cache.Get(url)
		assert.False(t, ok, "entry %d should have been evicted", i)
	}

	// The newest entries should still be present
	url := fmt.Sprintf("https://example.com/%d", MAX_CACHE_ENTRIES+4)
	got, ok := cache.Get(url)
	assert.True(t, ok)
	assert.Contains(t, got.content, "content-")
}

func TestURLCacheClear(t *testing.T) {
	cache := NewURLCache()

	// Add some entries
	for i := 0; i < 10; i++ {
		cache.Set(fmt.Sprintf("https://example.com/%d", i), &cacheEntry{
			content:   "content",
			fetchedAt: time.Now(),
		})
	}
	assert.Equal(t, 10, cache.Len())

	// Clear should remove all
	cache.Clear()
	assert.Equal(t, 0, cache.Len())

	// Confirm a specific entry is gone
	_, ok := cache.Get("https://example.com/0")
	assert.False(t, ok)
}

func TestPreapprovedHosts(t *testing.T) {
	// Known preapproved hosts should return true
	assert.True(t, isPreapprovedHost("docs.python.org", "/"))
	assert.True(t, isPreapprovedHost("developer.mozilla.org", "/"))
	assert.True(t, isPreapprovedHost("go.dev", "/"))
	assert.True(t, isPreapprovedHost("pkg.go.dev", "/docs"))
	assert.True(t, isPreapprovedHost("react.dev", "/"))
	assert.True(t, isPreapprovedHost("kubernetes.io", "/"))
	assert.True(t, isPreapprovedHost("redis.io", "/commands"))

	// Unknown hosts should return false
	assert.False(t, isPreapprovedHost("evil.example.com", "/"))
	assert.False(t, isPreapprovedHost("malware.io", "/"))
	assert.False(t, isPreapprovedHost("random-domain.net", "/"))
}

func TestPreapprovedHostPathBased(t *testing.T) {
	// github.com is only approved with the /anthropics path prefix
	assert.True(t, isPreapprovedHost("github.com", "/anthropics"))
	assert.True(t, isPreapprovedHost("github.com", "/anthropics/claude-code"))

	// github.com without the approved path prefix should NOT be approved
	assert.False(t, isPreapprovedHost("github.com", "/"))
	assert.False(t, isPreapprovedHost("github.com", "/other-org"))

	// Path segment boundary: /anthropics-evil should NOT match /anthropics
	assert.False(t, isPreapprovedHost("github.com", "/anthropics-evil"))
	assert.False(t, isPreapprovedHost("github.com", "/anthropics-evil/malware"))
}

func TestWebFetchCacheHit(t *testing.T) {
	// Clear global cache to avoid interference from other tests
	globalCache.Clear()

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "cached response body")
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "test prompt")

	// First fetch: should hit the server
	result1, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result1.IsError)
	assert.Equal(t, 1, requestCount)
	assert.Contains(t, result1.Content[0].Text, "cached response body")

	// Second fetch: should use cache (no additional server request)
	result2, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result2.IsError)
	assert.Equal(t, 1, requestCount, "second request should be served from cache")
	assert.Contains(t, result2.Content[0].Text, "cached response body")

	// Verify cached metadata
	meta, ok := result2.Metadata["cached"].(bool)
	assert.True(t, ok)
	assert.True(t, meta, "second result should be marked as cached")
}

func TestWebFetchHTTPUpgrade(t *testing.T) {
	globalCache.Clear()

	// Verify http:// loopback URLs are NOT upgraded (test server compatibility).
	// Start an HTTP server and verify it receives the request (no upgrade).
	var receivedScheme string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedScheme = "http" // we reached plain HTTP server, no upgrade
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}

	// Loopback http:// should NOT be upgraded (server will receive the request)
	input := makeInput(t, server.URL+"/test", "test")
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError, "loopback http:// should not be upgraded")
	assert.Equal(t, "http", receivedScheme)

	// Verify the upgrade logic: non-loopback http:// URLs get upgraded.
	// We test by checking the error message contains "https://" for a
	// non-routable address. Use a very short context to avoid waiting.
	ctx2, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	input2 := makeInput(t, "http://example.invalid/test", "test")
	result2, err := tool.Call(ctx2, input2, toolCtx)
	require.NoError(t, err)
	// The fetch will fail but the error should reference https:// showing upgrade
	assert.True(t, result2.IsError)
	assert.Contains(t, result2.Content[0].Text, "https://example.invalid")
}

func TestWebFetchContentTruncation(t *testing.T) {
	globalCache.Clear()

	// Generate content larger than MAX_MARKDOWN_LENGTH
	bigContent := strings.Repeat("A", MAX_MARKDOWN_LENGTH+5000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, bigContent)
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "extract info")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := result.Content[0].Text
	// Should contain the truncation marker
	assert.Contains(t, text, "[Content truncated due to length...]")
	// Total text (header + content + footer) should not exceed MAX_MARKDOWN_LENGTH + overhead
	assert.Less(t, len(text), MAX_MARKDOWN_LENGTH+2048)

	// Verify truncated metadata flag
	trunc, ok := result.Metadata["truncated"].(bool)
	assert.True(t, ok)
	assert.True(t, trunc)
}

func TestWebFetchCrossHostRedirect(t *testing.T) {
	globalCache.Clear()

	// Create two servers on different ports to simulate cross-host redirect
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "final content")
	}))
	defer finalServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, finalServer.URL, http.StatusFound)
	}))
	defer redirectServer.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, redirectServer.URL, "test prompt")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := result.Content[0].Text
	// Should detect cross-host redirect and inform the model
	assert.Contains(t, text, "REDIRECT DETECTED")
	assert.Contains(t, text, finalServer.URL)
	assert.Contains(t, text, "Please use WebFetch again")

	// Verify metadata
	crossRedirect, ok := result.Metadata["cross_redirect"].(bool)
	assert.True(t, ok)
	assert.True(t, crossRedirect)
}

func TestWebFetchPromptInOutput(t *testing.T) {
	globalCache.Clear()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "Some web content here")
	}))
	defer server.Close()

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}
	input := makeInput(t, server.URL, "Extract the main heading")

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := result.Content[0].Text
	// Output should contain the prompt for context framing
	assert.Contains(t, text, "processed with prompt: Extract the main heading")
	// And the actual content
	assert.Contains(t, text, "Some web content here")
}

func TestWebFetchPreapprovedHostPermission(t *testing.T) {
	tool := New()

	// Preapproved host should get Allow
	input := makeInput(t, "https://docs.python.org/3/library/json.html", "test")
	result, err := tool.CheckPermissions(context.Background(), input, nil)
	require.NoError(t, err)
	assert.Equal(t, tools.PermissionAllow, result)

	// Non-preapproved host should also get Allow (current behavior -- TODO for full parity)
	input2 := makeInput(t, "https://unknown-domain.example.com/page", "test")
	result2, err := tool.CheckPermissions(context.Background(), input2, nil)
	require.NoError(t, err)
	assert.Equal(t, tools.PermissionAllow, result2)

	// Path-based preapproved entry
	input3 := makeInput(t, "https://github.com/anthropics/claude-code", "test")
	result3, err := tool.CheckPermissions(context.Background(), input3, nil)
	require.NoError(t, err)
	assert.Equal(t, tools.PermissionAllow, result3)
}
