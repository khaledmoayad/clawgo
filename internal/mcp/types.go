// Package mcp implements the Model Context Protocol server and client for ClawGo.
// It wraps the official MCP Go SDK to expose ClawGo's tool registry over stdio
// and to connect to external MCP servers.
package mcp

// MCPTransportType identifies the transport protocol for an MCP server connection.
type MCPTransportType string

const (
	// TransportStdio uses stdin/stdout for JSON-RPC communication.
	TransportStdio MCPTransportType = "stdio"
	// TransportSSE uses Server-Sent Events over HTTP.
	TransportSSE MCPTransportType = "sse"
	// TransportHTTP uses Streamable HTTP transport.
	TransportHTTP MCPTransportType = "http"
)

// MCPServerConfig describes an external MCP server to connect to.
// Matches the TypeScript mcpServers settings format.
type MCPServerConfig struct {
	Name    string            `json:"name"`              // server identifier
	Type    MCPTransportType  `json:"type"`              // "stdio", "sse", "http"
	Command string            `json:"command,omitempty"`  // for stdio: executable path
	Args    []string          `json:"args,omitempty"`     // for stdio: command arguments
	Env     map[string]string `json:"env,omitempty"`      // environment variable overrides
	URL     string            `json:"url,omitempty"`      // for sse/http: server URL
	Headers map[string]string `json:"headers,omitempty"`  // custom headers for remote transports
}
