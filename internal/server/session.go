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

// SessionManager tracks active WebSocket sessions and their associated metadata.
type SessionManager struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	sessionInfos map[string]*SessionInfo
	idleTimers   map[string]*time.Timer
}

// NewSessionManager creates a new session manager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:     make(map[string]*Session),
		sessionInfos: make(map[string]*SessionInfo),
		idleTimers:   make(map[string]*time.Timer),
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
	for id, timer := range sm.idleTimers {
		timer.Stop()
		delete(sm.idleTimers, id)
	}
}

// AddSession registers a SessionInfo for tracking session lifecycle metadata.
func (sm *SessionManager) AddSession(info *SessionInfo) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.sessionInfos[info.ID] = info
}

// GetSession returns session metadata by ID, or nil if not found.
func (sm *SessionManager) GetSession(id string) *SessionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.sessionInfos[id]
}

// UpdateStatus transitions a session to a new lifecycle state.
// If transitioning to StateDetached and idleTimeoutMs > 0, starts an idle timer
// that will move the session to StateStopped after the timeout.
func (sm *SessionManager) UpdateStatus(id string, status SessionState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	info := sm.sessionInfos[id]
	if info == nil {
		return
	}
	info.Status = status

	// Cancel any existing idle timer for this session
	if timer, ok := sm.idleTimers[id]; ok {
		timer.Stop()
		delete(sm.idleTimers, id)
	}
}

// UpdateStatusWithIdleTimeout transitions a session to StateDetached and
// starts a timer that moves it to StateStopped after the given duration.
func (sm *SessionManager) UpdateStatusWithIdleTimeout(id string, timeout time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	info := sm.sessionInfos[id]
	if info == nil {
		return
	}
	info.Status = StateDetached

	// Cancel any existing idle timer
	if timer, ok := sm.idleTimers[id]; ok {
		timer.Stop()
	}

	if timeout > 0 {
		sm.idleTimers[id] = time.AfterFunc(timeout, func() {
			sm.mu.Lock()
			defer sm.mu.Unlock()
			if i := sm.sessionInfos[id]; i != nil && i.Status == StateDetached {
				i.Status = StateStopped
			}
			delete(sm.idleTimers, id)
		})
	}
}

// ListSessions returns all tracked session infos.
func (sm *SessionManager) ListSessions() []*SessionInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make([]*SessionInfo, 0, len(sm.sessionInfos))
	for _, info := range sm.sessionInfos {
		result = append(result, info)
	}
	return result
}

// RemoveSession removes session metadata by ID.
func (sm *SessionManager) RemoveSession(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.sessionInfos, id)
	if timer, ok := sm.idleTimers[id]; ok {
		timer.Stop()
		delete(sm.idleTimers, id)
	}
}
