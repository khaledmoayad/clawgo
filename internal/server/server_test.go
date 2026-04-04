package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startTestServer(t *testing.T) (*Server, int) {
	t.Helper()
	srv := NewServer(Config{
		Addr: "127.0.0.1:0",
	})
	ctx := context.Background()
	port, err := srv.Start(ctx)
	require.NoError(t, err)
	require.Greater(t, port, 0)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	return srv, port
}

func TestServer_Start(t *testing.T) {
	srv, port := startTestServer(t)
	assert.Greater(t, port, 0)
	assert.Equal(t, port, srv.Port())
}

func TestServer_HealthCheck(t *testing.T) {
	_, port := startTestServer(t)

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestServer_WebSocket(t *testing.T) {
	_, port := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws", port), nil)
	require.NoError(t, err)
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Send a test message
	testMsg := map[string]string{"type": "ping"}
	require.NoError(t, wsjson.Write(ctx, conn, testMsg))

	// Server echoes back (current behavior)
	var response map[string]string
	require.NoError(t, wsjson.Read(ctx, conn, &response))
	assert.Equal(t, "ping", response["type"])
}

func TestServer_Stop(t *testing.T) {
	srv := NewServer(Config{
		Addr: "127.0.0.1:0",
	})
	ctx := context.Background()
	port, err := srv.Start(ctx)
	require.NoError(t, err)

	// Verify server is running
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Stop the server
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, srv.Stop(stopCtx))

	// Give the OS a moment to release the port
	time.Sleep(50 * time.Millisecond)

	// Verify server is no longer accepting connections
	_, err = http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	assert.Error(t, err, "server should refuse connections after stop")
}

func TestSessionManager_AddRemove(t *testing.T) {
	sm := NewSessionManager()
	assert.Equal(t, 0, sm.Count())

	// We can't create a real websocket.Conn without a server,
	// so test with nil conn (just testing the map operations)
	sess := &Session{
		ID:        "test-session-1",
		Conn:      nil,
		CreatedAt: time.Now(),
	}

	sm.Add(sess)
	assert.Equal(t, 1, sm.Count())

	got := sm.Get("test-session-1")
	assert.NotNil(t, got)
	assert.Equal(t, "test-session-1", got.ID)

	sm.Remove("test-session-1")
	assert.Equal(t, 0, sm.Count())
	assert.Nil(t, sm.Get("test-session-1"))
}

func TestSessionManager_CloseAll(t *testing.T) {
	_, port := startTestServer(t)
	sm := NewSessionManager()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create 2 real WebSocket connections
	conn1, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws", port), nil)
	require.NoError(t, err)

	conn2, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws", port), nil)
	require.NoError(t, err)

	sm.Add(&Session{ID: "s1", Conn: conn1, CreatedAt: time.Now()})
	sm.Add(&Session{ID: "s2", Conn: conn2, CreatedAt: time.Now()})
	assert.Equal(t, 2, sm.Count())

	sm.CloseAll()
	assert.Equal(t, 0, sm.Count())
}

func TestServer_MultipleSessions(t *testing.T) {
	srv, port := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect two clients
	conn1, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws", port), nil)
	require.NoError(t, err)
	defer conn1.Close(websocket.StatusNormalClosure, "")

	conn2, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws", port), nil)
	require.NoError(t, err)
	defer conn2.Close(websocket.StatusNormalClosure, "")

	// Give the server a moment to register sessions
	time.Sleep(50 * time.Millisecond)

	// Server's internal session manager should track both
	assert.GreaterOrEqual(t, srv.Sessions().Count(), 2)
}

func TestNewServer_DefaultAddr(t *testing.T) {
	srv := NewServer(Config{})
	assert.Equal(t, "127.0.0.1:0", srv.config.Addr)
}

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()
	assert.Len(t, id1, 32, "session ID should be 32 hex chars (16 bytes)")
	assert.NotEqual(t, id1, id2, "session IDs should be unique")
}
