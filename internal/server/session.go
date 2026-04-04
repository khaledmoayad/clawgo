package server

import (
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Session represents a connected IDE extension session.
type Session struct {
	ID        string
	Conn      *websocket.Conn
	CreatedAt time.Time
}

// SessionManager tracks active WebSocket sessions.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// Add registers a new session.
func (sm *SessionManager) Add(sess *Session) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessions[sess.ID] = sess
}

// Remove deregisters a session by ID.
func (sm *SessionManager) Remove(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, id)
}

// Get returns a session by ID, or nil if not found.
func (sm *SessionManager) Get(id string) *Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessions[id]
}

// Count returns the number of active sessions.
func (sm *SessionManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// CloseAll closes all active sessions and clears the map.
func (sm *SessionManager) CloseAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for id, sess := range sm.sessions {
		sess.Conn.Close(websocket.StatusNormalClosure, "server shutting down")
		delete(sm.sessions, id)
	}
}
