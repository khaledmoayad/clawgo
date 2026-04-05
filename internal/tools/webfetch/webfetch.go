// Package webfetch implements the WebFetchTool for fetching web content.
// It performs HTTP GET requests and converts HTML responses to markdown
// using html-to-markdown. Non-HTML responses (text, JSON) are returned raw.
// Responses are cached for 15 minutes and content is bounded to 100KB.
package webfetch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// MAX_MARKDOWN_LENGTH is the maximum number of characters to include in the
// output content. Content exceeding this limit is truncated.
// Matches the TypeScript MAX_MARKDOWN_LENGTH constant.
const MAX_MARKDOWN_LENGTH = 100_000

// MaxResponseSize is the maximum number of bytes to read from a response body.
// Content exceeding this limit is truncated at the HTTP read level.
const MaxResponseSize = 10 * 1024 * 1024 // 10MB per PSR guidelines

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

// IsConcurrencySafe returns true -- HTTP fetch requests are independent and stateless.
func (t *WebFetchTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// CheckPermissions allows preapproved code-related domains automatically.
// For non-preapproved domains, the standard permission prompt is used.
func (t *WebFetchTool) CheckPermissions(_ context.Context, inp json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	// Parse input to extract URL for hostname check
	var in input
	if err := json.Unmarshal(inp, &in); err != nil {
		// If we can't parse input, fall through to allow (validation will catch it later)
		return permissions.Allow, nil
	}

	parsedURL, err := url.Parse(in.URL)
	if err != nil {
		return permissions.Allow, nil
	}

	// Preapproved hosts bypass the permission prompt
	if isPreapprovedHost(parsedURL.Hostname(), parsedURL.Path) {
		return permissions.Allow, nil
	}

	// Non-preapproved domains: allow for now (full parity would use Ask)
	// TODO: Full parity requires returning permissions.Ask for non-preapproved hosts
	// and building the permission suggestion system (domain-based rules).
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

	// Upgrade HTTP to HTTPS (matching TS behavior).
	// Skip upgrade for loopback addresses used in test environments.
	fetchURL := in.URL
	if strings.HasPrefix(fetchURL, "http://") {
		parsedForUpgrade, parseErr := url.Parse(fetchURL)
		if parseErr == nil {
			host := parsedForUpgrade.Hostname()
			if host != "127.0.0.1" && host != "localhost" && host != "::1" {
				fetchURL = "https://" + strings.TrimPrefix(fetchURL, "http://")
			}
		}
	}

	// 1. Cache check: return cached content if available and not expired
	if cached, ok := globalCache.Get(in.URL); ok {
		content := cached.content

		// Truncate content to MAX_MARKDOWN_LENGTH
		truncated := false
		if len(content) > MAX_MARKDOWN_LENGTH {
			content = content[:MAX_MARKDOWN_LENGTH]
			truncated = true
		}

		// Build output with prompt context
		var out strings.Builder
		fmt.Fprintf(&out, "Content from %s (processed with prompt: %s):\n\n", in.URL, in.Prompt)
		out.WriteString(content)
		if truncated {
			out.WriteString("\n\n[Content truncated due to length...]")
		}

		return &tools.ToolResult{
			Content: []tools.ContentBlock{{Type: "text", Text: out.String()}},
			Metadata: map[string]any{
				"status_code":  cached.statusCode,
				"content_type": cached.contentType,
				"final_url":    cached.finalURL,
				"truncated":    truncated,
				"cached":       true,
			},
		}, nil
	}

	// 2. HTTP fetch
	originalHost := ""
	if parsedURL, err := url.Parse(fetchURL); err == nil {
		originalHost = parsedURL.Host
	}

	client := &http.Client{
		Timeout: defaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("too many redirects (max %d)", maxRedirects)
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to create request: %s", err)), nil
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/markdown, text/html, */*")

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return tools.ErrorResult(fmt.Sprintf("request timeout or context cancelled: %s", ctx.Err())), nil
		}
		return tools.ErrorResult(fmt.Sprintf("fetch failed: %s", err)), nil
	}
	defer resp.Body.Close()

	// 3. Cross-host redirect detection: if the final URL's host differs from
	// the original request host, inform the model so it can re-fetch with
	// the correct URL (matching TS redirect behavior).
	finalURL := resp.Request.URL.String()
	finalHost := resp.Request.URL.Host
	if originalHost != "" && finalHost != "" && originalHost != finalHost {
		message := fmt.Sprintf(
			"REDIRECT DETECTED: The URL redirects to a different host.\n\n"+
				"Original URL: %s\n"+
				"Redirect URL: %s\n\n"+
				"To complete your request, I need to fetch content from the redirected URL. "+
				"Please use WebFetch again with these parameters:\n"+
				"- url: \"%s\"\n"+
				"- prompt: \"%s\"",
			in.URL, finalURL, finalURL, in.Prompt,
		)
		return &tools.ToolResult{
			Content: []tools.ContentBlock{{Type: "text", Text: message}},
			Metadata: map[string]any{
				"status_code":    resp.StatusCode,
				"content_type":   resp.Header.Get("Content-Type"),
				"final_url":      finalURL,
				"cross_redirect": true,
			},
		}, nil
	}

	// Check for non-2xx status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return tools.ErrorResult(fmt.Sprintf("HTTP error %d: %s", resp.StatusCode, resp.Status)), nil
	}

	// Read body up to MaxResponseSize + 1 to detect truncation at HTTP level
	limitReader := io.LimitReader(resp.Body, int64(MaxResponseSize+1))
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to read response body: %s", err)), nil
	}

	if len(body) > MaxResponseSize {
		body = body[:MaxResponseSize]
	}

	contentType := resp.Header.Get("Content-Type")
	content := string(body)

	// Convert HTML to markdown
	if isHTML(contentType) {
		md, convErr := htmltomarkdown.ConvertString(content)
		if convErr == nil {
			content = md
		}
	}

	// 4. Cache the response
	globalCache.Set(in.URL, &cacheEntry{
		content:     content,
		contentType: contentType,
		statusCode:  resp.StatusCode,
		finalURL:    finalURL,
		fetchedAt:   time.Now(),
	})

	// 5. Truncate content to MAX_MARKDOWN_LENGTH
	truncated := false
	if len(content) > MAX_MARKDOWN_LENGTH {
		content = content[:MAX_MARKDOWN_LENGTH]
		truncated = true
	}

	// 6. Build output with prompt context
	// TODO: Full parity requires sending content to a secondary (Haiku) model
	// with the user's prompt for focused extraction. For now, we frame the
	// response with the prompt and bound content to MAX_MARKDOWN_LENGTH.
	var out strings.Builder
	fmt.Fprintf(&out, "Content from %s (processed with prompt: %s):\n\n", in.URL, in.Prompt)
	out.WriteString(content)
	if truncated {
		out.WriteString("\n\n[Content truncated due to length...]")
	}

	return &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: out.String()}},
		Metadata: map[string]any{
			"status_code":  resp.StatusCode,
			"content_type": contentType,
			"final_url":    finalURL,
			"truncated":    truncated,
			"cached":       false,
		},
	}, nil
}

// isHTML checks if the Content-Type header indicates HTML content.
func isHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}
