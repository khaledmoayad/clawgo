package bridge

import (
	"context"
	"fmt"
	"sync"
)

// SessionHandle tracks a running child session spawned by the bridge.
type SessionHandle struct {
	WorkID          string
	SessionID       string
	AccessToken     string
	Cancel          context.CancelFunc
	Done            <-chan struct{}
	Activities      []SessionActivity
	CurrentActivity *SessionActivity

	mu sync.Mutex
}

// UpdateAccessToken updates the access token for a running session.
func (h *SessionHandle) UpdateAccessToken(token string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.AccessToken = token
}

// AddActivity records a new activity for this session.
func (h *SessionHandle) AddActivity(activity SessionActivity) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Activities = append(h.Activities, activity)
	h.CurrentActivity = &activity
	// Keep a ring buffer of last 10 activities
	if len(h.Activities) > 10 {
		h.Activities = h.Activities[len(h.Activities)-10:]
	}
}

// SessionPool manages concurrent child sessions with a configurable limit.
// Sessions are keyed by work ID (not session ID).
type SessionPool struct {
	mu            sync.Mutex
	sessions      map[string]*SessionHandle
	maxConcurrent int
}

// NewSessionPool creates a session pool with the given concurrency limit.
func NewSessionPool(maxConcurrent int) *SessionPool {
	return &SessionPool{
		sessions:      make(map[string]*SessionHandle),
		maxConcurrent: maxConcurrent,
	}
}

// CanSpawn returns true if the pool has capacity for another session.
func (p *SessionPool) CanSpawn() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.sessions) < p.maxConcurrent
}

// ActiveCount returns the number of currently running sessions.
func (p *SessionPool) ActiveCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.sessions)
}

// GetByWorkID returns the session handle for the given work ID, or nil.
func (p *SessionPool) GetByWorkID(workID string) *SessionHandle {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sessions[workID]
}

// Spawn starts a new child session for the given work response.
// The handler is called in a new goroutine with a child context.
// Sessions are keyed by work.ID (the work item ID from the API).
// When the handler returns, the session is automatically removed from the pool.
func (p *SessionPool) Spawn(ctx context.Context, work *WorkResponse, handler func(context.Context, *WorkResponse)) (*SessionHandle, error) {
	p.mu.Lock()
	if len(p.sessions) >= p.maxConcurrent {
		p.mu.Unlock()
		return nil, fmt.Errorf("session pool at capacity (%d/%d)", len(p.sessions), p.maxConcurrent)
	}

	childCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	handle := &SessionHandle{
		WorkID:    work.ID,
		SessionID: work.Data.ID,
		Cancel:    cancel,
		Done:      done,
	}
	p.sessions[work.ID] = handle
	p.mu.Unlock()

	go func() {
		defer close(done)
		defer func() {
			p.mu.Lock()
			delete(p.sessions, work.ID)
			p.mu.Unlock()
		}()
		handler(childCtx, work)
	}()

	return handle, nil
}

// StopAll cancels all running sessions and waits for them to finish.
func (p *SessionPool) StopAll() {
	p.mu.Lock()
	handles := make([]*SessionHandle, 0, len(p.sessions))
	for _, h := range p.sessions {
		handles = append(handles, h)
	}
	p.mu.Unlock()

	// Cancel all sessions
	for _, h := range handles {
		h.Cancel()
	}

	// Wait for all to finish
	for _, h := range handles {
		<-h.Done
	}
}
