// Package mcp implements the Model Context Protocol server and client for ClawGo.
// It wraps the official MCP Go SDK to expose ClawGo's tool registry over stdio
// and to connect to external MCP servers.
package mcp

// MCPTransportType identifies the transport protocol for an MCP server connection.
type MCPTransportType string

const (
	// TransportStdio uses stdin/stdout for JSON-RPC communication.
	TransportStdio MCPTransportType = "stdio"
	// TransportSSE uses Server-Sent Events over HTTP (2024-11-05 spec).
	TransportSSE MCPTransportType = "sse"
	// TransportHTTP uses Streamable HTTP transport (2025-03-26 spec).
	TransportHTTP MCPTransportType = "http"
	// TransportWebSocket uses WebSocket transport.
	TransportWebSocket MCPTransportType = "ws"
)

// ConfigScope identifies where an MCP server config was defined.
type ConfigScope string

const (
	ScopeLocal      ConfigScope = "local"
	ScopeUser       ConfigScope = "user"
	ScopeProject    ConfigScope = "project"
	ScopeDynamic    ConfigScope = "dynamic"
	ScopeEnterprise ConfigScope = "enterprise"
	ScopeClaudeAI   ConfigScope = "claudeai"
	ScopeManaged    ConfigScope = "managed"
)

// ConnectionStatus tracks the state of an MCP server connection.
type ConnectionStatus string

const (
	StatusConnected ConnectionStatus = "connected"
	StatusFailed    ConnectionStatus = "failed"
	StatusNeedsAuth ConnectionStatus = "needs-auth"
	StatusPending   ConnectionStatus = "pending"
	StatusDisabled  ConnectionStatus = "disabled"
)

// MCPServerConfig describes an external MCP server to connect to.
// Matches the TypeScript mcpServers settings format.
type MCPServerConfig struct {
	Name    string            `json:"name"`              // server identifier
	Type    MCPTransportType  `json:"type"`              // "stdio", "sse", "http", "ws"
	Command string            `json:"command,omitempty"`  // for stdio: executable path
	Args    []string          `json:"args,omitempty"`     // for stdio: command arguments
	Env     map[string]string `json:"env,omitempty"`      // environment variable overrides
	URL     string            `json:"url,omitempty"`      // for sse/http/ws: server URL
	Headers map[string]string `json:"headers,omitempty"`  // custom headers for remote transports

	// Lifecycle configuration
	Disabled             bool        `json:"disabled,omitempty"`              // whether this server is disabled
	Scope                ConfigScope `json:"scope,omitempty"`                 // where config was defined
	MaxReconnectAttempts int         `json:"maxReconnectAttempts,omitempty"`  // max reconnection attempts (0 = default 5)
	TimeoutMS            int         `json:"timeoutMs,omitempty"`             // tool call timeout in ms (0 = default ~27.8 hours)

	// OAuth configuration (for SSE/HTTP servers)
	OAuth *MCPOAuthConfig `json:"oauth,omitempty"`

	// HeadersHelper is a command to run to get dynamic headers.
	HeadersHelper string `json:"headersHelper,omitempty"`
}

// MCPOAuthConfig holds OAuth configuration for an MCP server.
type MCPOAuthConfig struct {
	ClientID              string `json:"clientId,omitempty"`
	CallbackPort          int    `json:"callbackPort,omitempty"`
	AuthServerMetadataURL string `json:"authServerMetadataUrl,omitempty"`
}
