// Package bridge - transport.go defines the bridge transport interface and
// the WebSocket-based V1 transport implementation.
//
// The transport abstracts the bidirectional message channel between the local
// bridge session and claude.ai. It handles connection lifecycle, reconnection
// with exponential backoff, batched writes, and keepalive pings.
//
// Matches the TypeScript HybridTransport + WebSocketTransport patterns from
// cli/transports/HybridTransport.ts and cli/transports/WebSocketTransport.ts.
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// TransportState represents the connection state of a bridge transport.
type TransportState string

const (
	TransportIdle         TransportState = "idle"
	TransportConnected    TransportState = "connected"
	TransportReconnecting TransportState = "reconnecting"
	TransportClosing      TransportState = "closing"
	TransportClosed       TransportState = "closed"
)

// Transport abstracts the bidirectional bridge transport.
// Implementations handle WebSocket (V1) or SSE+HTTP (V2).
type Transport interface {
	// Write sends a single message to the bridge server.
	Write(msg json.RawMessage) error

	// WriteBatch sends multiple messages atomically.
	WriteBatch(msgs []json.RawMessage) error

	// Close permanently closes the transport.
	Close() error

	// IsConnected returns true if the transport is currently connected.
	IsConnected() bool

	// State returns the current transport state label.
	State() TransportState

	// SetOnData sets the callback invoked when data is received.
	SetOnData(fn func(data json.RawMessage))

	// SetOnClose sets the callback invoked when the transport closes.
	SetOnClose(fn func(code int))

	// SetOnConnect sets the callback invoked when (re)connected.
	SetOnConnect(fn func())

	// Connect initiates the transport connection.
	Connect(ctx context.Context) error
}

// TransportConfig configures the WebSocket bridge transport.
type TransportConfig struct {
	// URL is the WebSocket URL (e.g., wss://api.anthropic.com/v1/sessions/{id}/subscribe).
	URL string

	// Headers are sent on the initial WebSocket handshake.
	Headers http.Header

	// SessionID identifies the bridge session.
	SessionID string

	// PingInterval is the interval between keepalive pings. Default: 10s.
	PingInterval time.Duration

	// KeepAliveInterval is the interval between keep_alive frames. Default: 5min.
	KeepAliveInterval time.Duration

	// ReconnectBaseDelay is the initial reconnect delay. Default: 1s.
	ReconnectBaseDelay time.Duration

	// ReconnectMaxDelay is the max reconnect delay. Default: 30s.
	ReconnectMaxDelay time.Duration

	// ReconnectGiveUpAfter is the max time to keep trying reconnects. Default: 10min.
	ReconnectGiveUpAfter time.Duration

	// BatchFlushInterval is the interval for batching stream events. Default: 100ms.
	BatchFlushInterval time.Duration

	// PostURL is the HTTP POST URL for hybrid mode writes. If empty, writes go over WebSocket.
	PostURL string

	// PostTimeout is the timeout for HTTP POST writes. Default: 15s.
	PostTimeout time.Duration

	// GetToken returns the current auth token for reconnections.
	GetToken func() string

	// OnDebug is an optional debug logging callback.
	OnDebug func(string)
}

const (
	defaultPingIntervalTransport  = 10 * time.Second
	defaultKeepAliveInterval      = 5 * time.Minute
	defaultReconnectBaseDelay     = 1 * time.Second
	defaultReconnectMaxDelay      = 30 * time.Second
	defaultReconnectGiveUpAfter   = 10 * time.Minute
	defaultBatchFlushInterval     = 100 * time.Millisecond
	defaultPostTimeout            = 15 * time.Second
	sleepDetectionThreshold       = 60 * time.Second

	keepAliveFrame = `{"type":"keep_alive"}`

	// Permanent close codes (no retry).
	closeProtocolError = 1002
	closeExpired       = 4001
	closeUnauth        = 4003
)

func (c TransportConfig) withDefaults() TransportConfig {
	if c.PingInterval <= 0 {
		c.PingInterval = defaultPingIntervalTransport
	}
	if c.KeepAliveInterval <= 0 {
		c.KeepAliveInterval = defaultKeepAliveInterval
	}
	if c.ReconnectBaseDelay <= 0 {
		c.ReconnectBaseDelay = defaultReconnectBaseDelay
	}
	if c.ReconnectMaxDelay <= 0 {
		c.ReconnectMaxDelay = defaultReconnectMaxDelay
	}
	if c.ReconnectGiveUpAfter <= 0 {
		c.ReconnectGiveUpAfter = defaultReconnectGiveUpAfter
	}
	if c.BatchFlushInterval <= 0 {
		c.BatchFlushInterval = defaultBatchFlushInterval
	}
	if c.PostTimeout <= 0 {
		c.PostTimeout = defaultPostTimeout
	}
	return c
}

// WSTransport implements Transport using a WebSocket connection with optional
// HTTP POST for writes (hybrid mode, matching the TS HybridTransport).
type WSTransport struct {
	config TransportConfig
	mu     sync.Mutex
	conn   *websocket.Conn
	state  TransportState
	ctx    context.Context
	cancel context.CancelFunc

	// Callbacks
	onData    func(json.RawMessage)
	onClose   func(int)
	onConnect func()

	// Reconnect state
	reconnectStart time.Time
	lastAttemptAt  time.Time

	// Batch buffer for writes
	batchMu    sync.Mutex
	batchBuf   []json.RawMessage
	batchTimer *time.Timer

	// HTTP client for hybrid POST writes
	httpClient *http.Client
}

// NewWSTransport creates a new WebSocket bridge transport.
func NewWSTransport(cfg TransportConfig) *WSTransport {
	cfg = cfg.withDefaults()
	return &WSTransport{
		config: cfg,
		state:  TransportIdle,
		httpClient: &http.Client{
			Timeout: cfg.PostTimeout,
		},
	}
}

func (t *WSTransport) debug(msg string) {
	if t.config.OnDebug != nil {
		t.config.OnDebug(msg)
	}
}

// Connect establishes the WebSocket connection and starts background loops.
func (t *WSTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	if t.state == TransportClosed {
		t.mu.Unlock()
		return fmt.Errorf("transport is permanently closed")
	}
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.state = TransportConnected
	t.mu.Unlock()

	if err := t.dial(); err != nil {
		t.mu.Lock()
		t.state = TransportIdle
		t.mu.Unlock()
		return err
	}

	// Start background loops
	go t.readLoop()
	go t.pingLoop()
	go t.keepAliveLoop()

	if t.onConnect != nil {
		t.onConnect()
	}

	return nil
}

// dial establishes the WebSocket connection.
func (t *WSTransport) dial() error {
	url := t.config.URL
	headers := t.config.Headers
	if headers == nil {
		headers = http.Header{}
	}

	// Refresh auth token on reconnects
	if t.config.GetToken != nil {
		token := t.config.GetToken()
		if token != "" {
			headers.Set("Authorization", "Bearer "+token)
		}
	}

	conn, _, err := websocket.Dial(t.ctx, url, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		return fmt.Errorf("transport dial: %w", err)
	}

	// Allow large messages (bridge messages can be large with file contents)
	conn.SetReadLimit(10 * 1024 * 1024) // 10MB

	t.mu.Lock()
	t.conn = conn
	t.state = TransportConnected
	t.mu.Unlock()

	t.debug("[transport] connected to " + url)
	return nil
}

// readLoop reads messages from the WebSocket and dispatches via onData callback.
func (t *WSTransport) readLoop() {
	for {
		t.mu.Lock()
		if t.state == TransportClosed || t.state == TransportClosing {
			t.mu.Unlock()
			return
		}
		conn := t.conn
		t.mu.Unlock()

		if conn == nil {
			return
		}

		var msg json.RawMessage
		err := wsjson.Read(t.ctx, conn, &msg)
		if err != nil {
			t.mu.Lock()
			if t.state == TransportClosed || t.state == TransportClosing {
				t.mu.Unlock()
				return
			}
			t.mu.Unlock()

			// Check if this is a permanent close
			closeCode := websocket.CloseStatus(err)
			if isPermanentCloseCode(int(closeCode)) {
				t.debug(fmt.Sprintf("[transport] permanent close code %d, not reconnecting", closeCode))
				t.closePermanent(int(closeCode))
				return
			}

			// Transient failure -- attempt reconnect
			t.attemptReconnect()
			return
		}

		if t.onData != nil {
			t.onData(msg)
		}
	}
}

// pingLoop sends periodic WebSocket pings.
func (t *WSTransport) pingLoop() {
	ticker := time.NewTicker(t.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.mu.Lock()
			if t.state != TransportConnected || t.conn == nil {
				t.mu.Unlock()
				return
			}
			conn := t.conn
			t.mu.Unlock()

			if err := conn.Ping(t.ctx); err != nil {
				// Ping failure -- readLoop will handle reconnect
				return
			}
		}
	}
}

// keepAliveLoop sends periodic keep_alive frames to prevent server-side timeouts.
func (t *WSTransport) keepAliveLoop() {
	ticker := time.NewTicker(t.config.KeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.mu.Lock()
			if t.state != TransportConnected || t.conn == nil {
				t.mu.Unlock()
				return
			}
			conn := t.conn
			t.mu.Unlock()

			_ = wsjson.Write(t.ctx, conn, json.RawMessage(keepAliveFrame))
		}
	}
}

// Write sends a single message. In hybrid mode, uses HTTP POST; otherwise WebSocket.
func (t *WSTransport) Write(msg json.RawMessage) error {
	if t.config.PostURL != "" {
		return t.postWrite([]json.RawMessage{msg})
	}
	return t.wsWrite(msg)
}

// WriteBatch sends multiple messages. In hybrid mode, uses a single HTTP POST.
func (t *WSTransport) WriteBatch(msgs []json.RawMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	if t.config.PostURL != "" {
		return t.postWrite(msgs)
	}
	// For plain WebSocket, send each individually
	for _, msg := range msgs {
		if err := t.wsWrite(msg); err != nil {
			return err
		}
	}
	return nil
}

// wsWrite sends a message over the WebSocket connection.
func (t *WSTransport) wsWrite(msg json.RawMessage) error {
	t.mu.Lock()
	if t.state != TransportConnected || t.conn == nil {
		t.mu.Unlock()
		return fmt.Errorf("transport not connected")
	}
	conn := t.conn
	t.mu.Unlock()

	return wsjson.Write(t.ctx, conn, msg)
}

// postWrite sends messages via HTTP POST (hybrid mode).
func (t *WSTransport) postWrite(msgs []json.RawMessage) error {
	payload, err := json.Marshal(msgs)
	if err != nil {
		return fmt.Errorf("marshal batch: %w", err)
	}

	req, err := http.NewRequestWithContext(t.ctx, http.MethodPost, t.config.PostURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.config.GetToken != nil {
		token := t.config.GetToken()
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("POST write: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("POST write: status %d", resp.StatusCode)
	}

	return nil
}

// Close permanently closes the transport.
func (t *WSTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == TransportClosed {
		return nil
	}

	t.state = TransportClosed
	if t.cancel != nil {
		t.cancel()
	}

	var err error
	if t.conn != nil {
		err = t.conn.CloseNow()
		t.conn = nil
	}

	if t.batchTimer != nil {
		t.batchTimer.Stop()
	}

	return err
}

// IsConnected returns true if the transport is currently connected.
func (t *WSTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state == TransportConnected
}

// State returns the current transport state.
func (t *WSTransport) State() TransportState {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// SetOnData sets the callback for incoming data.
func (t *WSTransport) SetOnData(fn func(json.RawMessage)) {
	t.onData = fn
}

// SetOnClose sets the callback for transport close events.
func (t *WSTransport) SetOnClose(fn func(int)) {
	t.onClose = fn
}

// SetOnConnect sets the callback for connection events.
func (t *WSTransport) SetOnConnect(fn func()) {
	t.onConnect = fn
}

// attemptReconnect tries to reconnect with exponential backoff.
func (t *WSTransport) attemptReconnect() {
	t.mu.Lock()
	if t.state == TransportClosed || t.state == TransportClosing {
		t.mu.Unlock()
		return
	}

	// Close existing connection
	if t.conn != nil {
		t.conn.CloseNow()
		t.conn = nil
	}
	t.state = TransportReconnecting
	t.mu.Unlock()

	now := time.Now()

	// Initialize reconnect tracking
	if t.reconnectStart.IsZero() {
		t.reconnectStart = now
	}

	// Sleep detection: if gap since last attempt is > threshold, reset budget
	// (machine likely slept)
	if !t.lastAttemptAt.IsZero() && now.Sub(t.lastAttemptAt) > sleepDetectionThreshold {
		t.debug("[transport] sleep detected, resetting reconnect budget")
		t.reconnectStart = now
	}

	// Check if we've exceeded the give-up timeout
	elapsed := now.Sub(t.reconnectStart)
	if elapsed > t.config.ReconnectGiveUpAfter {
		t.debug(fmt.Sprintf("[transport] reconnect budget exhausted after %s", elapsed))
		t.closePermanent(0)
		return
	}

	// Calculate delay with exponential backoff
	attempt := int(elapsed.Seconds())
	delay := time.Duration(float64(t.config.ReconnectBaseDelay) * math.Pow(2, float64(attempt/5)))
	if delay > t.config.ReconnectMaxDelay {
		delay = t.config.ReconnectMaxDelay
	}

	t.lastAttemptAt = now

	t.debug(fmt.Sprintf("[transport] reconnecting in %s (elapsed: %s)", delay, elapsed))

	select {
	case <-t.ctx.Done():
		return
	case <-time.After(delay):
	}

	if err := t.dial(); err != nil {
		t.debug(fmt.Sprintf("[transport] reconnect dial failed: %v", err))
		t.attemptReconnect() // retry
		return
	}

	// Reset reconnect tracking
	t.reconnectStart = time.Time{}
	t.lastAttemptAt = time.Time{}

	// Restart background loops
	go t.readLoop()
	go t.pingLoop()
	go t.keepAliveLoop()

	if t.onConnect != nil {
		t.onConnect()
	}
}

// closePermanent closes the transport with a final close code.
func (t *WSTransport) closePermanent(code int) {
	t.mu.Lock()
	t.state = TransportClosed
	if t.conn != nil {
		t.conn.CloseNow()
		t.conn = nil
	}
	if t.cancel != nil {
		t.cancel()
	}
	t.mu.Unlock()

	if t.onClose != nil {
		t.onClose(code)
	}
}

// isPermanentCloseCode returns true if the close code means no retry should happen.
func isPermanentCloseCode(code int) bool {
	switch code {
	case closeProtocolError, closeExpired, closeUnauth:
		return true
	}
	return false
}

// BuildWSURL constructs a WebSocket URL from an API base URL and session ID.
// Converts http(s):// to ws(s):// scheme.
func BuildWSURL(apiBaseURL, sessionID string) string {
	base := apiBaseURL
	scheme := "wss"

	switch {
	case strings.HasPrefix(base, "http://"):
		scheme = "ws"
		base = strings.TrimPrefix(base, "http://")
	case strings.HasPrefix(base, "https://"):
		base = strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "ws://"):
		scheme = "ws"
		base = strings.TrimPrefix(base, "ws://")
	case strings.HasPrefix(base, "wss://"):
		base = strings.TrimPrefix(base, "wss://")
	}

	base = strings.TrimSuffix(base, "/")
	return fmt.Sprintf("%s://%s/v1/sessions/%s/subscribe", scheme, base, sessionID)
}

// BuildPostURL constructs the HTTP POST URL from a WebSocket URL.
// Converts ws(s):// back to http(s):// and changes the path.
func BuildPostURL(wsURL string) string {
	url := wsURL
	switch {
	case strings.HasPrefix(url, "wss://"):
		url = "https://" + strings.TrimPrefix(url, "wss://")
	case strings.HasPrefix(url, "ws://"):
		url = "http://" + strings.TrimPrefix(url, "ws://")
	}
	// Replace /subscribe with /messages for POST endpoint
	url = strings.Replace(url, "/subscribe", "/messages", 1)
	return url
}
