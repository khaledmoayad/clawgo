package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	reconnectDelay       = 2 * time.Second
	maxReconnectDelay    = 30 * time.Second
	maxReconnectAttempts = 5
	pingInterval         = 30 * time.Second

	// Custom close codes from the server
	closeSessionNotFound = 4001
	closeUnauthorized    = 4003
)

// wsState represents the current state of the WebSocket connection.
type wsState string

const (
	wsStateDisconnected wsState = "disconnected"
	wsStateConnecting   wsState = "connecting"
	wsStateConnected    wsState = "connected"
	wsStateClosed       wsState = "closed"
)

// WSCallbacks holds callback functions invoked on WebSocket lifecycle events.
type WSCallbacks struct {
	OnConnected    func()
	OnMessage      func(json.RawMessage)
	OnDisconnected func(error)
	OnReconnecting func(attempt int)
}

// SessionsWebSocket manages a WebSocket connection to the sessions subscription endpoint.
// It handles automatic reconnection with exponential backoff for transient failures
// and permanent disconnection for authorization errors.
type SessionsWebSocket struct {
	sessionID  string
	orgUUID    string
	getToken   func() string
	apiBaseURL string

	callbacks         WSCallbacks
	conn              *websocket.Conn
	mu                sync.Mutex
	state             wsState
	reconnectAttempts int
}

// NewSessionsWebSocket creates a new WebSocket client for subscribing to session events.
func NewSessionsWebSocket(sessionID, orgUUID, apiBaseURL string, getToken func() string, callbacks WSCallbacks) *SessionsWebSocket {
	return &SessionsWebSocket{
		sessionID:  sessionID,
		orgUUID:    orgUUID,
		getToken:   getToken,
		apiBaseURL: apiBaseURL,
		callbacks:  callbacks,
		state:      wsStateDisconnected,
	}
}

// Connect establishes the WebSocket connection and starts read/ping loops.
func (ws *SessionsWebSocket) Connect(ctx context.Context) error {
	ws.mu.Lock()
	if ws.state == wsStateClosed {
		ws.mu.Unlock()
		return fmt.Errorf("websocket is permanently closed")
	}
	ws.state = wsStateConnecting
	ws.mu.Unlock()

	// Build WebSocket URL: wss://{apiBaseURL}/v1/sessions/{sessionID}/subscribe?org={orgUUID}
	// Supports ws://, wss://, http://, https:// base URLs (http/ws used in tests).
	base := ws.apiBaseURL
	scheme := "wss"
	switch {
	case strings.HasPrefix(base, "ws://"):
		scheme = "ws"
		base = strings.TrimPrefix(base, "ws://")
	case strings.HasPrefix(base, "wss://"):
		base = strings.TrimPrefix(base, "wss://")
	case strings.HasPrefix(base, "http://"):
		scheme = "ws"
		base = strings.TrimPrefix(base, "http://")
	case strings.HasPrefix(base, "https://"):
		base = strings.TrimPrefix(base, "https://")
	}
	base = strings.TrimSuffix(base, "/")
	url := fmt.Sprintf("%s://%s/v1/sessions/%s/subscribe?org=%s", scheme, base, ws.sessionID, ws.orgUUID)

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+ws.getToken())
	headers.Set("anthropic-version", "2023-06-01")

	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		ws.mu.Lock()
		ws.state = wsStateDisconnected
		ws.mu.Unlock()
		return fmt.Errorf("websocket dial: %w", err)
	}

	ws.mu.Lock()
	ws.conn = conn
	ws.state = wsStateConnected
	ws.reconnectAttempts = 0
	ws.mu.Unlock()

	if ws.callbacks.OnConnected != nil {
		ws.callbacks.OnConnected()
	}

	// Start background loops
	go ws.readLoop(ctx)
	go ws.pingLoop(ctx)

	return nil
}

// readLoop reads messages from the WebSocket and dispatches them via callbacks.
func (ws *SessionsWebSocket) readLoop(ctx context.Context) {
	for {
		ws.mu.Lock()
		if ws.state == wsStateClosed {
			ws.mu.Unlock()
			return
		}
		conn := ws.conn
		ws.mu.Unlock()

		if conn == nil {
			return
		}

		var msg json.RawMessage
		err := wsjson.Read(ctx, conn, &msg)
		if err != nil {
			ws.mu.Lock()
			if ws.state == wsStateClosed {
				ws.mu.Unlock()
				return
			}
			ws.mu.Unlock()

			// Check close code for permanent vs transient failures
			closeStatus := websocket.CloseStatus(err)
			switch closeStatus {
			case closeUnauthorized:
				// Permanent failure -- do not reconnect
				ws.mu.Lock()
				ws.state = wsStateDisconnected
				ws.mu.Unlock()
				if ws.callbacks.OnDisconnected != nil {
					ws.callbacks.OnDisconnected(fmt.Errorf("unauthorized (close code %d)", closeUnauthorized))
				}
				return
			case closeSessionNotFound:
				// Transient during compaction -- try reconnect
				ws.reconnect(ctx)
				return
			default:
				// Other errors -- attempt reconnect
				ws.reconnect(ctx)
				return
			}
		}

		if ws.callbacks.OnMessage != nil {
			ws.callbacks.OnMessage(msg)
		}
	}
}

// pingLoop sends periodic pings to keep the connection alive.
func (ws *SessionsWebSocket) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ws.mu.Lock()
			if ws.state != wsStateConnected || ws.conn == nil {
				ws.mu.Unlock()
				return
			}
			conn := ws.conn
			ws.mu.Unlock()

			if err := conn.Ping(ctx); err != nil {
				// Ping failure -- readLoop will handle reconnect via read error
				return
			}
		}
	}
}

// reconnect attempts to re-establish the WebSocket connection with exponential backoff.
func (ws *SessionsWebSocket) reconnect(ctx context.Context) {
	ws.mu.Lock()
	if ws.state == wsStateClosed {
		ws.mu.Unlock()
		return
	}

	ws.reconnectAttempts++
	attempts := ws.reconnectAttempts
	if attempts > maxReconnectAttempts {
		ws.state = wsStateDisconnected
		ws.mu.Unlock()
		if ws.callbacks.OnDisconnected != nil {
			ws.callbacks.OnDisconnected(fmt.Errorf("max reconnect attempts (%d) exceeded", maxReconnectAttempts))
		}
		return
	}

	// Close existing connection if any
	if ws.conn != nil {
		ws.conn.Close(websocket.StatusGoingAway, "reconnecting")
		ws.conn = nil
	}
	ws.state = wsStateDisconnected
	ws.mu.Unlock()

	if ws.callbacks.OnReconnecting != nil {
		ws.callbacks.OnReconnecting(attempts)
	}

	// Exponential backoff: reconnectDelay * 2^(attempts-1), capped at maxReconnectDelay
	delay := time.Duration(float64(reconnectDelay) * math.Pow(2, float64(attempts-1)))
	if delay > maxReconnectDelay {
		delay = maxReconnectDelay
	}

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	if err := ws.Connect(ctx); err != nil {
		// Connect failed -- it will try reconnect again via readLoop
		return
	}
}

// Close permanently closes the WebSocket connection.
// Uses CloseNow for immediate teardown instead of blocking on the close handshake.
func (ws *SessionsWebSocket) Close() error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.state = wsStateClosed
	if ws.conn != nil {
		err := ws.conn.CloseNow()
		ws.conn = nil
		return err
	}
	return nil
}

// State returns the current WebSocket connection state.
func (ws *SessionsWebSocket) State() wsState {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.state
}
