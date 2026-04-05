package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// errNoSession is returned when an operation requires a live MCP session
// but the ConnectedServer has none (e.g., disabled or not yet connected).
var errNoSession = errors.New("no active MCP session")

// DefaultMaxReconnectAttempts is the default number of reconnection attempts.
const DefaultMaxReconnectAttempts = 5

// ConnectedServer represents a live connection to an external MCP server.
// It caches the server's tool list after the initial ListTools discovery.
type ConnectedServer struct {
	Config  MCPServerConfig
	Status  ConnectionStatus
	session *gomcp.ClientSession
	tools   []*gomcp.Tool // cached from ListTools
	cancel  context.CancelFunc

	// normalizedTools maps normalized tool names (mcp__server__tool) to original names
	normalizedTools map[string]string

	// discovery caches the normalized tools, resources, and prompts from
	// the last RefreshDiscovery call.
	discovery discoveryCache

	// stderrBuf captures stderr output from stdio subprocess for debugging
	stderrBuf *bytes.Buffer

	// mu guards status changes and reconnection
	mu sync.Mutex

	// reconnectAttempt tracks the current reconnection attempt count
	reconnectAttempt int
}

// ConnectToServer connects to an external MCP server based on the given config.
// For stdio transport, it launches the server as a subprocess.
// For SSE transport, it connects via Server-Sent Events.
// For HTTP transport, it connects via Streamable HTTP.
func ConnectToServer(ctx context.Context, cfg MCPServerConfig) (*ConnectedServer, error) {
	if cfg.Disabled {
		return &ConnectedServer{
			Config: cfg,
			Status: StatusDisabled,
		}, nil
	}

	client := gomcp.NewClient(
		&gomcp.Implementation{Name: "clawgo", Version: "1.0.0"},
		nil,
	)

	var transport gomcp.Transport
	switch cfg.Type {
	case TransportStdio, "":
		if cfg.Command == "" {
			return nil, fmt.Errorf("stdio transport requires a command")
		}
		cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
		cmd.Env = buildEnv(cfg.Env)

		// Capture stderr for debugging instead of forwarding to os.Stderr
		stderrBuf := &bytes.Buffer{}
		cmd.Stderr = stderrBuf

		cmdTransport := &gomcp.CommandTransport{Command: cmd}
		return connectWithTransportAndStderr(ctx, client, cfg, cmdTransport, stderrBuf)

	case TransportSSE:
		if cfg.URL == "" {
			return nil, fmt.Errorf("SSE transport requires a URL")
		}
		httpClient := buildHTTPClient(cfg)
		transport = &gomcp.SSEClientTransport{
			Endpoint:   cfg.URL,
			HTTPClient: httpClient,
		}

	case TransportHTTP:
		if cfg.URL == "" {
			return nil, fmt.Errorf("HTTP transport requires a URL")
		}
		httpClient := buildHTTPClient(cfg)
		transport = &gomcp.StreamableClientTransport{
			Endpoint:   cfg.URL,
			HTTPClient: httpClient,
		}

	case TransportWebSocket:
		return nil, fmt.Errorf("transport %q not yet supported", cfg.Type)

	default:
		return nil, fmt.Errorf("unknown transport type: %q", cfg.Type)
	}

	return connectWithTransport(ctx, client, cfg, transport)
}

// ConnectToServerWithTransport connects to an MCP server using a provided
// transport. This is used for testing with in-memory transports.
func ConnectToServerWithTransport(ctx context.Context, cfg MCPServerConfig, transport gomcp.Transport) (*ConnectedServer, error) {
	client := gomcp.NewClient(
		&gomcp.Implementation{Name: "clawgo", Version: "1.0.0"},
		nil,
	)
	return connectWithTransport(ctx, client, cfg, transport)
}

// connectWithTransport performs the actual connection using the provided
// client and transport.
func connectWithTransport(ctx context.Context, client *gomcp.Client, cfg MCPServerConfig, transport gomcp.Transport) (*ConnectedServer, error) {
	return connectWithTransportAndStderr(ctx, client, cfg, transport, nil)
}

// connectWithTransportAndStderr performs the actual connection and optionally
// captures stderr from the subprocess.
func connectWithTransportAndStderr(ctx context.Context, client *gomcp.Client, cfg MCPServerConfig, transport gomcp.Transport, stderrBuf *bytes.Buffer) (*ConnectedServer, error) {
	connCtx, cancel := context.WithCancel(ctx)

	session, err := client.Connect(connCtx, transport, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("connecting to MCP server %q: %w", cfg.Name, err)
	}

	// Discover tools
	listResult, err := session.ListTools(connCtx, nil)
	if err != nil {
		cancel()
		session.Close()
		return nil, fmt.Errorf("listing tools from %q: %w", cfg.Name, err)
	}

	cs := &ConnectedServer{
		Config:          cfg,
		Status:          StatusConnected,
		session:         session,
		tools:           listResult.Tools,
		cancel:          cancel,
		stderrBuf:       stderrBuf,
		normalizedTools: make(map[string]string),
	}

	// Build normalized tool name mapping
	for _, tool := range cs.tools {
		normalized := BuildMCPToolName(cfg.Name, tool.Name)
		cs.normalizedTools[normalized] = tool.Name
	}

	return cs, nil
}

// ListTools returns the cached list of tools from the connected server.
func (cs *ConnectedServer) ListTools() []*gomcp.Tool {
	return cs.tools
}

// NormalizedToolNames returns a map of normalized tool names (mcp__server__tool)
// to original tool names as reported by the server.
func (cs *ConnectedServer) NormalizedToolNames() map[string]string {
	return cs.normalizedTools
}

// OriginalToolName returns the original tool name for a normalized name,
// or empty string if not found.
func (cs *ConnectedServer) OriginalToolName(normalized string) string {
	return cs.normalizedTools[normalized]
}

// CallTool calls a tool on the connected server with a default timeout.
// For full policy behavior (progress, _meta, retries), use CallToolWithPolicy.
func (cs *ConnectedServer) CallTool(ctx context.Context, name string, args map[string]any) (*gomcp.CallToolResult, error) {
	// Apply timeout
	timeoutMS := cs.Config.TimeoutMS
	if timeoutMS <= 0 {
		timeoutMS = DefaultMCPToolTimeoutMs
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	return cs.session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

// Stderr returns captured stderr output from a stdio transport subprocess.
// Returns empty string for non-stdio transports.
func (cs *ConnectedServer) Stderr() string {
	if cs.stderrBuf == nil {
		return ""
	}
	return cs.stderrBuf.String()
}

// Reconnect attempts to reconnect to the server. It returns an error if
// the maximum reconnect attempts have been exceeded or if reconnection fails.
func (cs *ConnectedServer) Reconnect(ctx context.Context) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	maxAttempts := cs.Config.MaxReconnectAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultMaxReconnectAttempts
	}

	if cs.reconnectAttempt >= maxAttempts {
		cs.Status = StatusFailed
		return fmt.Errorf("max reconnect attempts (%d) exceeded for server %q", maxAttempts, cs.Config.Name)
	}

	cs.reconnectAttempt++
	cs.Status = StatusPending

	// Close existing connection if any
	if cs.session != nil {
		cs.session.Close()
	}
	if cs.cancel != nil {
		cs.cancel()
	}

	// Exponential backoff: 500ms * 2^attempt, capped at 30s
	backoff := time.Duration(500*time.Millisecond) * (1 << uint(cs.reconnectAttempt-1))
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}

	select {
	case <-time.After(backoff):
	case <-ctx.Done():
		cs.Status = StatusFailed
		return ctx.Err()
	}

	newCS, err := ConnectToServer(ctx, cs.Config)
	if err != nil {
		cs.Status = StatusFailed
		return fmt.Errorf("reconnecting to %q (attempt %d/%d): %w",
			cs.Config.Name, cs.reconnectAttempt, maxAttempts, err)
	}

	// Transfer state from new connection
	cs.session = newCS.session
	cs.tools = newCS.tools
	cs.cancel = newCS.cancel
	cs.stderrBuf = newCS.stderrBuf
	cs.normalizedTools = newCS.normalizedTools
	cs.Status = StatusConnected
	cs.reconnectAttempt = 0

	return nil
}

// Close terminates the connection to the MCP server. For stdio transport,
// this kills the subprocess with graceful shutdown escalation (SIGTERM then SIGKILL).
func (cs *ConnectedServer) Close() error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.cancel != nil {
		cs.cancel()
	}
	if cs.session != nil {
		return cs.session.Close()
	}
	return nil
}

// buildHTTPClient creates an http.Client with custom headers from the config
// and a per-request timeout matching MCPRequestTimeoutMs. This timeout applies
// to individual POST/auth operations; long-lived SSE GET streams use their own
// context cancellation and are unaffected.
func buildHTTPClient(cfg MCPServerConfig) *http.Client {
	timeout := time.Duration(MCPRequestTimeoutMs) * time.Millisecond
	base := http.DefaultTransport
	if len(cfg.Headers) == 0 {
		return &http.Client{Timeout: timeout}
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &headerTransport{
			base:    base,
			headers: cfg.Headers,
		},
	}
}

// headerTransport is an http.RoundTripper that injects custom headers
// into every request.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid mutating the original
	clone := req.Clone(req.Context())
	for k, v := range t.headers {
		clone.Header.Set(k, v)
	}
	return t.base.RoundTrip(clone)
}

// buildEnv creates an environment variable slice by starting from the current
// process environment and applying overrides from the config.
func buildEnv(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return os.Environ()
	}

	// Start with current environment
	env := os.Environ()

	// Build a set of override keys for fast lookup
	overrideKeys := make(map[string]bool, len(overrides))
	for k := range overrides {
		overrideKeys[k] = true
	}

	// Filter out existing vars that will be overridden
	filtered := make([]string, 0, len(env)+len(overrides))
	for _, e := range env {
		key := envKey(e)
		if !overrideKeys[key] {
			filtered = append(filtered, e)
		}
	}

	// Append overrides
	for k, v := range overrides {
		filtered = append(filtered, k+"="+v)
	}

	return filtered
}

// envKey extracts the key from an environment variable string "KEY=VALUE".
func envKey(envVar string) string {
	for i, c := range envVar {
		if c == '=' {
			return envVar[:i]
		}
	}
	return envVar
}
