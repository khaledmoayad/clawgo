package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ConnectedServer represents a live connection to an external MCP server.
// It caches the server's tool list after the initial ListTools discovery.
type ConnectedServer struct {
	Config  MCPServerConfig
	session *gomcp.ClientSession
	tools   []*gomcp.Tool // cached from ListTools
	cancel  context.CancelFunc
}

// ConnectToServer connects to an external MCP server based on the given config.
// For stdio transport, it launches the server as a subprocess.
func ConnectToServer(ctx context.Context, cfg MCPServerConfig) (*ConnectedServer, error) {
	client := gomcp.NewClient(
		&gomcp.Implementation{Name: "clawgo", Version: "1.0.0"},
		nil,
	)

	var transport gomcp.Transport
	switch cfg.Type {
	case TransportStdio:
		if cfg.Command == "" {
			return nil, fmt.Errorf("stdio transport requires a command")
		}
		cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
		cmd.Env = buildEnv(cfg.Env)
		cmd.Stderr = os.Stderr // Forward server stderr for debugging
		transport = &gomcp.CommandTransport{Command: cmd}
	case TransportSSE, TransportHTTP:
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

	return &ConnectedServer{
		Config:  cfg,
		session: session,
		tools:   listResult.Tools,
		cancel:  cancel,
	}, nil
}

// ListTools returns the cached list of tools from the connected server.
func (cs *ConnectedServer) ListTools() []*gomcp.Tool {
	return cs.tools
}

// CallTool calls a tool on the connected server.
func (cs *ConnectedServer) CallTool(ctx context.Context, name string, args map[string]any) (*gomcp.CallToolResult, error) {
	return cs.session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
}

// Close terminates the connection to the MCP server. For stdio transport,
// this kills the subprocess.
func (cs *ConnectedServer) Close() error {
	if cs.cancel != nil {
		cs.cancel()
	}
	if cs.session != nil {
		return cs.session.Close()
	}
	return nil
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
