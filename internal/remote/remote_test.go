package remote

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wsTestServer creates an httptest server that upgrades to WebSocket and calls handler.
func wsTestServer(t *testing.T, handler func(ctx context.Context, conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			t.Logf("websocket accept error: %v", err)
			return
		}
		handler(r.Context(), conn)
	}))
}

// testURL returns the httptest server URL for use as apiBaseURL.
// The WebSocket client handles http:// -> ws:// conversion internally.
func testURL(s *httptest.Server) string {
	return s.URL
}

func TestSessionsWebSocket_Connect(t *testing.T) {
	connected := make(chan struct{})
	srv := wsTestServer(t, func(ctx context.Context, conn *websocket.Conn) {
		// Keep connection open until context cancels
		<-ctx.Done()
		conn.Close(websocket.StatusNormalClosure, "server done")
	})
	defer srv.Close()

	ws := NewSessionsWebSocket("sess-123", "org-456", testURL(srv), func() string { return "test-token" }, WSCallbacks{
		OnConnected: func() {
			close(connected)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ws.Connect(ctx)
	require.NoError(t, err)
	defer ws.Close()

	select {
	case <-connected:
		// Success
	case <-ctx.Done():
		t.Fatal("timeout waiting for OnConnected callback")
	}

	assert.Equal(t, wsStateConnected, ws.State())
}

func TestSessionsWebSocket_ReadLoop(t *testing.T) {
	msgReceived := make(chan json.RawMessage, 1)
	srv := wsTestServer(t, func(ctx context.Context, conn *websocket.Conn) {
		testMsg := map[string]string{"type": "session_update", "status": "active"}
		if err := wsjson.Write(ctx, conn, testMsg); err != nil {
			t.Logf("server write error: %v", err)
			return
		}
		// Keep connection open briefly
		time.Sleep(500 * time.Millisecond)
		conn.Close(websocket.StatusNormalClosure, "done")
	})
	defer srv.Close()

	ws := NewSessionsWebSocket("sess-123", "org-456", testURL(srv), func() string { return "test-token" }, WSCallbacks{
		OnMessage: func(msg json.RawMessage) {
			select {
			case msgReceived <- msg:
			default:
			}
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ws.Connect(ctx)
	require.NoError(t, err)
	defer ws.Close()

	select {
	case msg := <-msgReceived:
		var parsed map[string]string
		err := json.Unmarshal(msg, &parsed)
		require.NoError(t, err)
		assert.Equal(t, "session_update", parsed["type"])
		assert.Equal(t, "active", parsed["status"])
	case <-ctx.Done():
		t.Fatal("timeout waiting for message")
	}
}

func TestSessionsWebSocket_Reconnect(t *testing.T) {
	var connCount atomic.Int32
	var reconnectAttempts []int
	var mu sync.Mutex

	srv := wsTestServer(t, func(ctx context.Context, conn *websocket.Conn) {
		count := connCount.Add(1)
		if count == 1 {
			// First connection: close with 4001 (session not found -- transient)
			conn.Close(websocket.StatusCode(closeSessionNotFound), "session not found")
			return
		}
		// Subsequent connections: keep open briefly then close normally
		time.Sleep(200 * time.Millisecond)
		conn.Close(websocket.StatusNormalClosure, "done")
	})
	defer srv.Close()

	ws := NewSessionsWebSocket("sess-123", "org-456", testURL(srv), func() string { return "test-token" }, WSCallbacks{
		OnReconnecting: func(attempt int) {
			mu.Lock()
			reconnectAttempts = append(reconnectAttempts, attempt)
			mu.Unlock()
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := ws.Connect(ctx)
	require.NoError(t, err)
	defer ws.Close()

	// Wait for reconnect to happen
	time.Sleep(5 * time.Second)

	mu.Lock()
	attempts := make([]int, len(reconnectAttempts))
	copy(attempts, reconnectAttempts)
	mu.Unlock()

	assert.NotEmpty(t, attempts, "expected at least one reconnect attempt")
	assert.Equal(t, 1, attempts[0], "first reconnect attempt should be 1")
	assert.True(t, connCount.Load() >= 2, "server should have received at least 2 connections")
}

func TestSessionsWebSocket_PermanentClose(t *testing.T) {
	disconnected := make(chan error, 1)
	var reconnectCalled atomic.Int32

	srv := wsTestServer(t, func(ctx context.Context, conn *websocket.Conn) {
		// Close with 4003 (unauthorized) -- should NOT reconnect
		conn.Close(websocket.StatusCode(closeUnauthorized), "unauthorized")
	})
	defer srv.Close()

	ws := NewSessionsWebSocket("sess-123", "org-456", testURL(srv), func() string { return "test-token" }, WSCallbacks{
		OnDisconnected: func(err error) {
			select {
			case disconnected <- err:
			default:
			}
		},
		OnReconnecting: func(attempt int) {
			reconnectCalled.Add(1)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ws.Connect(ctx)
	require.NoError(t, err)
	defer ws.Close()

	select {
	case err := <-disconnected:
		assert.Contains(t, err.Error(), "unauthorized")
	case <-ctx.Done():
		t.Fatal("timeout waiting for OnDisconnected callback")
	}

	// Verify no reconnect was attempted
	assert.Equal(t, int32(0), reconnectCalled.Load(), "reconnect should NOT be attempted on 4003")
}

func TestSessionsWebSocket_Close(t *testing.T) {
	srv := wsTestServer(t, func(ctx context.Context, conn *websocket.Conn) {
		<-ctx.Done()
	})
	defer srv.Close()

	ws := NewSessionsWebSocket("sess-123", "org-456", testURL(srv), func() string { return "test-token" }, WSCallbacks{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ws.Connect(ctx)
	require.NoError(t, err)

	err = ws.Close()
	require.NoError(t, err)
	assert.Equal(t, wsStateClosed, ws.State())
}

func TestRemoteSessionManager_Events(t *testing.T) {
	srv := wsTestServer(t, func(ctx context.Context, conn *websocket.Conn) {
		// Send a session event
		evt := map[string]interface{}{
			"type": "session_update",
			"data": map[string]string{"status": "running"},
		}
		if err := wsjson.Write(ctx, conn, evt); err != nil {
			t.Logf("server write error: %v", err)
			return
		}
		// Keep connection open
		time.Sleep(1 * time.Second)
		conn.Close(websocket.StatusNormalClosure, "done")
	})
	defer srv.Close()

	mgr := NewRemoteSessionManager("sess-123", "org-456", testURL(srv), func() string { return "test-token" })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := mgr.Start(ctx)
	require.NoError(t, err)
	defer mgr.Close()

	// First event should be the synthetic "connected" event
	var gotConnected, gotUpdate bool
	timeout := time.After(5 * time.Second)

	for i := 0; i < 3; i++ {
		select {
		case evt := <-mgr.Events():
			switch evt.Type {
			case "connected":
				gotConnected = true
			case "session_update":
				gotUpdate = true
			}
			if gotConnected && gotUpdate {
				break
			}
		case <-timeout:
			break
		}
		if gotConnected && gotUpdate {
			break
		}
	}

	assert.True(t, gotConnected, "should receive connected event")
	assert.True(t, gotUpdate, "should receive session_update event")
	assert.Equal(t, "sess-123", mgr.SessionID())
}
