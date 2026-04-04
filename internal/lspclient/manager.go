package lspclient

import (
	"context"
	"fmt"
	"sync"
)

// Manager tracks LSP client instances by language, reusing existing clients
// when possible and creating new ones on demand.
type Manager struct {
	mu      sync.Mutex
	clients map[string]*Client
}

// NewManager creates a new LSP client manager.
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
	}
}

// GetOrCreate returns an existing LSP client for the given language, or
// creates a new one if none exists or the existing one is no longer running.
func (m *Manager) GetOrCreate(ctx context.Context, language string, serverCmd string, args ...string) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for existing client with running process
	if client, ok := m.clients[language]; ok {
		if client.cmd != nil && client.cmd.Process != nil {
			// Check if process is still alive (non-blocking)
			select {
			case <-client.done:
				// Process has exited; remove stale client
				delete(m.clients, language)
			default:
				return client, nil
			}
		}
	}

	// Create new client
	client, err := NewClient(ctx, serverCmd, args...)
	if err != nil {
		return nil, fmt.Errorf("lspclient manager: create client for %s: %w", language, err)
	}

	m.clients[language] = client
	return client, nil
}

// Get returns the LSP client for the given language, or nil if none exists.
func (m *Manager) Get(language string) *Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients[language]
}

// CloseAll closes all managed LSP clients and clears the map.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for lang, client := range m.clients {
		_ = client.Close()
		delete(m.clients, lang)
	}
}

// Count returns the number of active clients.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.clients)
}
