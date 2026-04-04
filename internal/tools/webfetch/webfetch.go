// Package webfetch implements the WebFetchTool for fetching web content.
// It performs HTTP GET requests and converts HTML responses to markdown
// using html-to-markdown. Non-HTML responses (text, JSON) are returned raw.
package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// MaxResponseSize is the maximum number of bytes to read from a response body.
// Content exceeding this limit is truncated.
const MaxResponseSize = 100 * 1024 // 100KB

const (
	defaultTimeout = 30 * time.Second
	maxRedirects   = 10
	userAgent      = "ClawGo/1.0 (compatible; Claude Code)"
)

type input struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

func (in *input) Validate() error {
	if strings.TrimSpace(in.URL) == "" {
		return fmt.Errorf("required field \"url\" is missing or empty")
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return fmt.Errorf("required field \"prompt\" is missing or empty")
	}
	return nil
}

// WebFetchTool fetches content from a URL and converts HTML to markdown.
type WebFetchTool struct{}

// New creates a new WebFetchTool.
func New() *WebFetchTool { return &WebFetchTool{} }

func (t *WebFetchTool) Name() string                { return "WebFetch" }
func (t *WebFetchTool) Description() string          { return toolDescription }
func (t *WebFetchTool) IsReadOnly() bool             { return true }
func (t *WebFetchTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true — HTTP fetch requests are independent and stateless.
func (t *WebFetchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// CheckPermissions always allows — this is a read-only network tool.
func (t *WebFetchTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

func (t *WebFetchTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	// Validate URL scheme
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		return tools.ErrorResult("invalid URL: must start with http:// or https://"), nil
	}

	// Create HTTP client with timeout and redirect limit
	client := &http.Client{
		Timeout: defaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects (max %d)", maxRedirects)
			}
			return nil
		},
	}

	// Build request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to create request: %s", err)), nil
	}
	req.Header.Set("User-Agent", userAgent)

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		// Provide friendly error messages
		if ctx.Err() != nil {
			return tools.ErrorResult(fmt.Sprintf("request timeout or context cancelled: %s", ctx.Err())), nil
		}
		return tools.ErrorResult(fmt.Sprintf("fetch failed: %s", err)), nil
	}
	defer resp.Body.Close()

	// Check for non-2xx status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tools.ErrorResult(fmt.Sprintf("HTTP error %d: %s", resp.StatusCode, resp.Status)), nil
	}

	// Read body up to MaxResponseSize + 1 to detect truncation
	limitReader := io.LimitReader(resp.Body, int64(MaxResponseSize+1))
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to read response body: %s", err)), nil
	}

	truncated := false
	if len(body) > MaxResponseSize {
		body = body[:MaxResponseSize]
		truncated = true
	}

	// Determine content type
	contentType := resp.Header.Get("Content-Type")
	content := string(body)

	// Convert HTML to markdown
	if isHTML(contentType) {
		md, convErr := htmltomarkdown.ConvertString(content)
		if convErr == nil {
			content = md
		}
		// On conversion error, fall through and return the raw HTML
	}

	// Build output
	var out strings.Builder
	fmt.Fprintf(&out, "Prompt: %s\n\n", in.Prompt)
	fmt.Fprintf(&out, "URL: %s\n", resp.Request.URL.String())
	fmt.Fprintf(&out, "Status: %d\n", resp.StatusCode)
	fmt.Fprintf(&out, "Content-Type: %s\n\n", contentType)
	out.WriteString(content)

	if truncated {
		out.WriteString("\n\n[truncated — response exceeded 100KB limit]")
	}

	return &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: out.String()}},
		Metadata: map[string]any{
			"status_code":  resp.StatusCode,
			"content_type": contentType,
			"final_url":    resp.Request.URL.String(),
			"truncated":    truncated,
		},
	}, nil
}

// isHTML checks if the Content-Type header indicates HTML content.
func isHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}
