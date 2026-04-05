package mcp

import (
	"context"
	"fmt"
	"sync"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Manager coordinates all connected MCP servers and provides a unified
// interface for listing/reading resources, discovering tools, and managing
// the lifecycle of server connections.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*ConnectedServer // keyed by server name
}

// NewManager creates a new MCP Manager with no connected servers.
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*ConnectedServer),
	}
}

// AddServer registers a connected server with the manager.
func (m *Manager) AddServer(name string, cs *ConnectedServer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[name] = cs
}

// RemoveServer removes a server from the manager.
func (m *Manager) RemoveServer(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.servers, name)
}

// GetServer returns a connected server by name, or nil if not found.
func (m *Manager) GetServer(name string) *ConnectedServer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.servers[name]
}

// Servers returns a snapshot of all connected servers.
func (m *Manager) Servers() map[string]*ConnectedServer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*ConnectedServer, len(m.servers))
	for k, v := range m.servers {
		result[k] = v
	}
	return result
}

// ListResources aggregates discovered resources from all connected servers,
// optionally filtered by server name. An empty serverFilter returns resources
// from all servers.
func (m *Manager) ListResources(ctx context.Context, serverFilter string) ([]DiscoveredResource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []DiscoveredResource
	for name, cs := range m.servers {
		if cs.Status != StatusConnected {
			continue
		}
		if serverFilter != "" && name != serverFilter {
			continue
		}
		resources, err := cs.ListResources(ctx)
		if err != nil {
			// One server failure shouldn't block the entire listing
			continue
		}
		all = append(all, resources...)
	}
	return all, nil
}

// ReadResource reads a specific resource from the named server.
// The serverName must match a connected server's name exactly.
func (m *Manager) ReadResource(ctx context.Context, serverName, uri string) (*gomcp.ReadResourceResult, error) {
	m.mu.RLock()
	cs, ok := m.servers[serverName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("server %q not found", serverName)
	}
	if cs.Status != StatusConnected {
		return nil, fmt.Errorf("server %q is not connected (status: %s)", serverName, cs.Status)
	}
	return cs.ReadResource(ctx, uri)
}

// ConnectAll connects to all provided MCP server configurations concurrently.
// Each server is connected individually; failures are recorded as StatusFailed
// or StatusNeedsAuth on the server entry rather than aborting the batch.
// Disabled servers are registered with StatusDisabled. Servers that require
// OAuth but have no token are registered with StatusNeedsAuth.
func (m *Manager) ConnectAll(ctx context.Context, configs []MCPServerConfig) {
	for _, cfg := range configs {
		if cfg.Disabled {
			m.AddServer(cfg.Name, &ConnectedServer{
				Config: cfg,
				Status: StatusDisabled,
			})
			continue
		}

		// Check if OAuth is needed but not yet available
		if cfg.OAuth != nil {
			provider := NewClaudeAuthProvider(cfg)
			if NeedsAuth(cfg, provider) {
				m.AddServer(cfg.Name, &ConnectedServer{
					Config: cfg,
					Status: StatusNeedsAuth,
				})
				continue
			}
		}

		cs, err := ConnectToServer(ctx, cfg)
		if err != nil {
			m.AddServer(cfg.Name, &ConnectedServer{
				Config: cfg,
				Status: StatusFailed,
			})
			continue
		}

		// Run initial discovery to populate tools, resources, prompts
		if discErr := cs.RefreshDiscovery(ctx); discErr != nil {
			// Connection succeeded but discovery failed -- still register
			// with connected status; tools may be partially populated.
			_ = discErr
		}
		m.AddServer(cfg.Name, cs)
	}
}

// ListDiscoveredPrompts aggregates discovered prompts from all connected servers.
// Returns a flat slice of all prompts across servers.
func (m *Manager) ListDiscoveredPrompts() []DiscoveredPrompt {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []DiscoveredPrompt
	for _, cs := range m.servers {
		if cs.Status != StatusConnected {
			continue
		}
		cs.discovery.mu.RLock()
		all = append(all, cs.discovery.prompts...)
		cs.discovery.mu.RUnlock()
	}
	return all
}

// ListDiscoveredTools aggregates discovered tools from all connected servers.
// Returns a flat slice of all tools across servers.
func (m *Manager) ListDiscoveredTools() []DiscoveredTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []DiscoveredTool
	for _, cs := range m.servers {
		if cs.Status != StatusConnected {
			continue
		}
		all = append(all, cs.DiscoveredTools()...)
	}
	return all
}

// ServerStatus returns a summary of each server's name and connection status.
// Useful for the /mcp command to display live state.
func (m *Manager) ServerStatus() []ServerStatusEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]ServerStatusEntry, 0, len(m.servers))
	for name, cs := range m.servers {
		entry := ServerStatusEntry{
			Name:      name,
			Status:    cs.Status,
			Transport: string(cs.Config.Type),
			ToolCount: len(cs.DiscoveredTools()),
		}
		entries = append(entries, entry)
	}
	return entries
}

// ServerStatusEntry describes the current state of a single MCP server.
type ServerStatusEntry struct {
	Name      string
	Status    ConnectionStatus
	Transport string
	ToolCount int
}

// Close terminates all managed server connections.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, cs := range m.servers {
		if err := cs.Close(); err != nil {
			lastErr = fmt.Errorf("closing server %q: %w", name, err)
		}
	}
	m.servers = make(map[string]*ConnectedServer)
	return lastErr
}
