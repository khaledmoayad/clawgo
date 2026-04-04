package api

import (
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/google/uuid"
)

// RequestHeaders holds the custom headers to inject into API requests.
// Mirrors the TypeScript client.ts default headers construction.
type RequestHeaders struct {
	// AppName identifies the application (always "cli" for ClawGo).
	AppName string
	// UserAgent is the User-Agent string (e.g., "ClawGo/1.0.0").
	UserAgent string
	// SessionID is the unique session identifier (UUID).
	SessionID string
	// ContainerID is the remote container ID (from CLAUDE_CODE_CONTAINER_ID env var).
	// Only included if non-empty.
	ContainerID string
	// RemoteSessionID is the remote session ID (from CLAUDE_CODE_REMOTE_SESSION_ID env var).
	// Only included if non-empty.
	RemoteSessionID string
	// ClientApp identifies the SDK consumer's app/library.
	// Only included if non-empty.
	ClientApp string
	// ClientRequestID is a per-request UUID for server-side log correlation.
	// Generated automatically if empty.
	ClientRequestID string
}

// ClientRequestIDHeader is the header key for client-generated request IDs.
// Used for server-log lookup when debugging API errors.
const ClientRequestIDHeader = "x-client-request-id"

// InjectCustomHeaders returns a map of custom header key-value pairs to inject
// into API requests. Mirrors the TypeScript client.ts defaultHeaders construction.
func InjectCustomHeaders(headers RequestHeaders) map[string]string {
	result := make(map[string]string)

	// Always set x-app
	appName := headers.AppName
	if appName == "" {
		appName = "cli"
	}
	result["x-app"] = appName

	// User-Agent
	if headers.UserAgent != "" {
		result["User-Agent"] = headers.UserAgent
	}

	// Session ID
	if headers.SessionID != "" {
		result["X-Claude-Code-Session-Id"] = headers.SessionID
	}

	// Remote container ID (conditional)
	if headers.ContainerID != "" {
		result["x-claude-remote-container-id"] = headers.ContainerID
	}

	// Remote session ID (conditional)
	if headers.RemoteSessionID != "" {
		result["x-claude-remote-session-id"] = headers.RemoteSessionID
	}

	// Client app (conditional)
	if headers.ClientApp != "" {
		result["x-client-app"] = headers.ClientApp
	}

	// Client request ID -- generate if not provided
	clientRequestID := headers.ClientRequestID
	if clientRequestID == "" {
		clientRequestID = uuid.New().String()
	}
	result[ClientRequestIDHeader] = clientRequestID

	return result
}

// InjectCustomHeadersAsOptions converts custom headers into SDK RequestOption
// values that can be passed to the streaming API call. Each header becomes a
// WithHeader option.
func InjectCustomHeadersAsOptions(headers RequestHeaders) []option.RequestOption {
	headerMap := InjectCustomHeaders(headers)
	opts := make([]option.RequestOption, 0, len(headerMap))
	for k, v := range headerMap {
		opts = append(opts, option.WithHeader(k, v))
	}
	return opts
}
