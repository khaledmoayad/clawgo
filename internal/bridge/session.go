package bridge

import (
	"context"
	"fmt"
	"sync"
)

// SessionHandle tracks a running child session spawned by the bridge.
type SessionHandle struct {
	ID     string
	Cancel context.CancelFunc
	Done   <-chan struct{}
}

// SessionPool manages concurrent child sessions with a configurable limit.
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

// Spawn starts a new child session for the given work item.
// The handler is called in a new goroutine with a child context.
// When the handler returns, the session is automatically removed from the pool.
func (p *SessionPool) Spawn(ctx context.Context, work WorkItem, handler func(context.Context, WorkItem)) (*SessionHandle, error) {
	p.mu.Lock()
	if len(p.sessions) >= p.maxConcurrent {
		p.mu.Unlock()
		return nil, fmt.Errorf("session pool at capacity (%d/%d)", len(p.sessions), p.maxConcurrent)
	}

	childCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	handle := &SessionHandle{
		ID:     work.SessionID,
		Cancel: cancel,
		Done:   done,
	}
	p.sessions[work.SessionID] = handle
	p.mu.Unlock()

	go func() {
		defer close(done)
		defer func() {
			p.mu.Lock()
			delete(p.sessions, work.SessionID)
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
