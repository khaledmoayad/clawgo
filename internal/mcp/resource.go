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
