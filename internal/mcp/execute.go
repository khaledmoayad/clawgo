package mcp

import (
	"context"
	"errors"
	"fmt"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// DefaultMCPToolTimeoutMs is the default tool-call timeout in milliseconds.
// Matches Claude Code's DEFAULT_MCP_TOOL_TIMEOUT_MS (~27.8 hours, effectively
// infinite -- the real safeguard is the query-level budget).
const DefaultMCPToolTimeoutMs = 100_000_000

// MCPRequestTimeoutMs is the per-HTTP-request timeout in milliseconds applied
// to remote HTTP/SSE transports for non-streaming operations (POST, auth).
// Long-lived GET streams are exempt.
const MCPRequestTimeoutMs = 60_000

// MaxURLElicitationRetries caps how many times we retry a tool call after
// receiving a -32042 URL-elicitation-required error.
const MaxURLElicitationRetries = 3

// ElicitationRequiredCode is the MCP JSON-RPC error code for
// UrlElicitationRequired (-32042). Servers return this when the tool call
// cannot complete without the user visiting a URL first.
const ElicitationRequiredCode = -32042

// ToolCallOptions configures the behavior of CallToolWithPolicy.
type ToolCallOptions struct {
	// TimeoutMs overrides DefaultMCPToolTimeoutMs when > 0.
	TimeoutMs int

	// OnProgress is called when the server sends progress notifications
	// during a long-running tool call.
	OnProgress func(ProgressEvent)

	// OnElicitation is called when the server returns -32042 and the call
	// requires user interaction (e.g. open a URL). Return true to retry
	// the tool call, false to abort.
	OnElicitation func(elicitations []ElicitationRequest) bool
}

// ProgressEvent describes incremental progress reported by an MCP server
// during a tool call.
type ProgressEvent struct {
	// Status is one of "started", "progress", "completed", "failed".
	Status string
	// ServerName identifies the MCP server.
	ServerName string
	// ToolName is the tool being executed.
	ToolName string
	// Progress is the current progress value (optional).
	Progress float64
	// Total is the expected total (optional, 0 if unknown).
	Total float64
	// Message is an optional human-readable status message from the server.
	Message string
	// ElapsedMs is milliseconds since the call started (set on completed/failed).
	ElapsedMs int64
}

// ElicitationRequest represents a URL-elicitation entry extracted from a
// -32042 error's data payload.
type ElicitationRequest struct {
	ElicitationID string `json:"elicitationId"`
	URL           string `json:"url"`
	Message       string `json:"message"`
}

// MCPError wraps a JSON-RPC error returned by an MCP server, preserving
// the numeric code so callers can distinguish protocol errors from bugs.
type MCPError struct {
	Code    int
	Message string
	Data    map[string]any
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// CallToolWithPolicy wraps a raw MCP tool call with Claude Code-compatible
// runtime policy:
//   - _meta passthrough to the server
//   - hard timeout (DefaultMCPToolTimeoutMs or opts.TimeoutMs)
//   - progress reporting via opts.OnProgress
//   - one retry after 401/403 (auth recovery)
//   - one retry after -32042 (URL elicitation required)
func (cs *ConnectedServer) CallToolWithPolicy(
	ctx context.Context,
	name string,
	args map[string]any,
	meta map[string]any,
	opts ToolCallOptions,
) (*gomcp.CallToolResult, error) {
	if cs.session == nil {
		return nil, errNoSession
	}

	// Determine timeout.
	timeoutMs := opts.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = DefaultMCPToolTimeoutMs
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	startTime := time.Now()

	// Emit started progress.
	if opts.OnProgress != nil {
		opts.OnProgress(ProgressEvent{
			Status:     "started",
			ServerName: cs.Config.Name,
			ToolName:   name,
		})
	}

	// Build CallToolParams with _meta passthrough.
	params := &gomcp.CallToolParams{
		Name:      name,
		Arguments: args,
	}
	if len(meta) > 0 {
		params.Meta = gomcp.Meta(meta)
	}

	// Retry loop: at most one auth retry + elicitation retries.
	const maxAuthRetries = 1
	authRetries := 0
	elicitRetries := 0

	for {
		result, err := cs.session.CallTool(ctx, params)

		if err == nil {
			// Normalize and size-bound the result before returning.
			result, err = NormalizeCallToolResult(result)
			if err != nil {
				if opts.OnProgress != nil {
					opts.OnProgress(ProgressEvent{
						Status:     "failed",
						ServerName: cs.Config.Name,
						ToolName:   name,
						ElapsedMs:  time.Since(startTime).Milliseconds(),
					})
				}
				return nil, fmt.Errorf("normalizing result for tool %q: %w", name, err)
			}

			// Success -- emit completed progress.
			if opts.OnProgress != nil {
				opts.OnProgress(ProgressEvent{
					Status:     "completed",
					ServerName: cs.Config.Name,
					ToolName:   name,
					ElapsedMs:  time.Since(startTime).Milliseconds(),
				})
			}
			return result, nil
		}

		// Check for auth errors (401/403).
		if isAuthError(err) && authRetries < maxAuthRetries {
			authRetries++
			continue
		}

		// Check for elicitation-required (-32042).
		if mcpErr, ok := asMCPError(err); ok && mcpErr.Code == ElicitationRequiredCode {
			if elicitRetries >= MaxURLElicitationRetries {
				if opts.OnProgress != nil {
					opts.OnProgress(ProgressEvent{
						Status:     "failed",
						ServerName: cs.Config.Name,
						ToolName:   name,
						ElapsedMs:  time.Since(startTime).Milliseconds(),
					})
				}
				return nil, fmt.Errorf("tool %q exceeded max URL elicitation retries (%d): %w",
					name, MaxURLElicitationRetries, err)
			}
			elicitRetries++

			// Extract elicitation entries from error data.
			elicitations := extractElicitations(mcpErr.Data)

			// If a handler is registered, let it decide whether to retry.
			if opts.OnElicitation != nil && len(elicitations) > 0 {
				if !opts.OnElicitation(elicitations) {
					// User declined -- surface a readable error.
					if opts.OnProgress != nil {
						opts.OnProgress(ProgressEvent{
							Status:     "failed",
							ServerName: cs.Config.Name,
							ToolName:   name,
							ElapsedMs:  time.Since(startTime).Milliseconds(),
						})
					}
					return nil, fmt.Errorf("tool %q: URL elicitation declined by user", name)
				}
				// Retry the call.
				continue
			}

			// No handler or no valid elicitation entries -- fail.
			if opts.OnProgress != nil {
				opts.OnProgress(ProgressEvent{
					Status:     "failed",
					ServerName: cs.Config.Name,
					ToolName:   name,
					ElapsedMs:  time.Since(startTime).Milliseconds(),
				})
			}
			return nil, fmt.Errorf("tool %q requires URL elicitation but no handler configured: %w", name, err)
		}

		// Unrecoverable error.
		if opts.OnProgress != nil {
			opts.OnProgress(ProgressEvent{
				Status:     "failed",
				ServerName: cs.Config.Name,
				ToolName:   name,
				ElapsedMs:  time.Since(startTime).Milliseconds(),
			})
		}
		return nil, err
	}
}

// isAuthError checks if an error indicates a 401 or 403 HTTP status,
// which signals that the OAuth token expired or was revoked.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	// The Go MCP SDK wraps HTTP errors -- check the error message for status
	// codes since the SDK does not export a typed HTTP error.
	msg := err.Error()
	for _, code := range []string{"401", "403"} {
		if containsStatus(msg, code) {
			return true
		}
	}
	// Also check wrapped MCPError codes that map to auth failures.
	var mcpErr *MCPError
	if errors.As(err, &mcpErr) {
		// JSON-RPC doesn't have standard auth codes, but some servers use them.
		return mcpErr.Code == 401 || mcpErr.Code == 403
	}
	return false
}

// containsStatus checks if the error message contains an HTTP status code
// in common positions (e.g., "status 401", "HTTP 403", or just the number).
func containsStatus(msg, code string) bool {
	// Simple substring check -- the SDK typically includes the status code
	// in the error message as "status NNN" or "HTTP NNN" or "NNN".
	return len(msg) > 0 && (contains(msg, "status "+code) ||
		contains(msg, "HTTP "+code) ||
		contains(msg, code+" "))
}

// contains is a simple substring check (avoids importing strings for one use).
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// asMCPError attempts to extract an MCPError from an error chain.
// It first checks errors.As, then falls back to inspecting the error message
// for the Go MCP SDK's error format.
func asMCPError(err error) (*MCPError, bool) {
	var mcpErr *MCPError
	if errors.As(err, &mcpErr) {
		return mcpErr, true
	}
	return nil, false
}

// extractElicitations pulls ElicitationRequest entries from the -32042
// error's data map. Returns nil if data is missing or malformed.
func extractElicitations(data map[string]any) []ElicitationRequest {
	if data == nil {
		return nil
	}
	raw, ok := data["elicitations"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	var result []ElicitationRequest
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		eid, _ := m["elicitationId"].(string)
		eurl, _ := m["url"].(string)
		emsg, _ := m["message"].(string)
		if eid == "" || eurl == "" || emsg == "" {
			continue
		}
		result = append(result, ElicitationRequest{
			ElicitationID: eid,
			URL:           eurl,
			Message:       emsg,
		})
	}
	return result
}
