package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/khaledmoayad/clawgo/internal/app"
)

// SessionEvent represents an event received from the remote sessions WebSocket.
type SessionEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// RemoteSessionManager manages a WebSocket subscription to remote session events.
// It wraps SessionsWebSocket and provides a channel-based event interface.
type RemoteSessionManager struct {
	ws        *SessionsWebSocket
	sessionID string
	events    chan SessionEvent
	done      chan struct{}
	closeOnce sync.Once
}

// NewRemoteSessionManager creates a new manager that subscribes to session events
// for the given session via WebSocket.
func NewRemoteSessionManager(sessionID, orgUUID, apiBaseURL string, getToken func() string) *RemoteSessionManager {
	m := &RemoteSessionManager{
		sessionID: sessionID,
		events:    make(chan SessionEvent, 64),
		done:      make(chan struct{}),
	}

	callbacks := WSCallbacks{
		OnConnected: func() {
			// Push a synthetic connected event
			select {
			case m.events <- SessionEvent{Type: "connected"}:
			default:
			}
		},
		OnMessage: func(raw json.RawMessage) {
			var evt SessionEvent
			if err := json.Unmarshal(raw, &evt); err != nil {
				// If unmarshalling fails, wrap the raw message as data
				evt = SessionEvent{Type: "raw", Data: raw}
			}
			select {
			case m.events <- evt:
			default:
				// Drop event if channel is full to avoid blocking
			}
		},
		OnDisconnected: func(err error) {
			select {
			case m.events <- SessionEvent{
				Type: "disconnected",
				Data: json.RawMessage(fmt.Sprintf(`{"error":%q}`, err.Error())),
			}:
			default:
			}
		},
		OnReconnecting: func(attempt int) {
			select {
			case m.events <- SessionEvent{
				Type: "reconnecting",
				Data: json.RawMessage(fmt.Sprintf(`{"attempt":%d}`, attempt)),
			}:
			default:
			}
		},
	}

	m.ws = NewSessionsWebSocket(sessionID, orgUUID, apiBaseURL, getToken, callbacks)
	return m
}

// Start connects the WebSocket and registers cleanup on application shutdown.
func (m *RemoteSessionManager) Start(ctx context.Context) error {
	if err := m.ws.Connect(ctx); err != nil {
		return fmt.Errorf("starting remote session manager: %w", err)
	}

	app.RegisterCleanup(func() {
		m.Close()
	})

	return nil
}

// Events returns a read-only channel of session events.
func (m *RemoteSessionManager) Events() <-chan SessionEvent {
	return m.events
}

// Close permanently closes the WebSocket and the events channel.
func (m *RemoteSessionManager) Close() error {
	var err error
	m.closeOnce.Do(func() {
		close(m.done)
		err = m.ws.Close()
		close(m.events)
	})
	return err
}

// SessionID returns the session ID this manager is subscribed to.
func (m *RemoteSessionManager) SessionID() string {
	return m.sessionID
}
