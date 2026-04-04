package uds

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAddress_UDS(t *testing.T) {
	scheme, target := ParseAddress("uds:/tmp/my.sock")
	assert.Equal(t, "uds", scheme)
	assert.Equal(t, "/tmp/my.sock", target)
}

func TestParseAddress_Bridge(t *testing.T) {
	scheme, target := ParseAddress("bridge:target-host")
	assert.Equal(t, "bridge", scheme)
	assert.Equal(t, "target-host", target)
}

func TestParseAddress_Absolute(t *testing.T) {
	scheme, target := ParseAddress("/var/run/claude.sock")
	assert.Equal(t, "uds", scheme)
	assert.Equal(t, "/var/run/claude.sock", target)
}

func TestParseAddress_Other(t *testing.T) {
	scheme, target := ParseAddress("http://localhost:8080")
	assert.Equal(t, "other", scheme)
	assert.Equal(t, "http://localhost:8080", target)
}

func TestListenSend_Roundtrip(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")

	listener, err := Listen(sockPath)
	require.NoError(t, err)
	defer listener.Close()

	type TestMessage struct {
		Action string `json:"action"`
		Value  int    `json:"value"`
	}

	sent := TestMessage{Action: "update", Value: 42}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Send in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- Send(ctx, sockPath, sent)
	}()

	// Receive
	raw, err := Receive(ctx, listener)
	require.NoError(t, err)

	// Wait for send to finish
	sendErr := <-errCh
	require.NoError(t, sendErr)

	// Verify message contents
	var received TestMessage
	err = json.Unmarshal(raw, &received)
	require.NoError(t, err)
	assert.Equal(t, "update", received.Action)
	assert.Equal(t, 42, received.Value)
}

func TestSocketPath_Length(t *testing.T) {
	// Typical configDir for Linux: /home/username/.claude
	configDir := "/home/username/.claude"
	sessionID := "sess-a1b2c3d4-e5f6-7890-abcd-ef1234567890"

	path := SocketPath(configDir, sessionID)

	// Unix socket path limit is 108 bytes (Linux)
	assert.Less(t, len(path), 108, "socket path must be under 108 bytes, got %d: %s", len(path), path)
	assert.Contains(t, path, "sockets")
	assert.True(t, filepath.Ext(path) == ".sock", "should have .sock extension")
}

func TestSocketPath_Consistency(t *testing.T) {
	configDir := t.TempDir()
	sessionID := "sess-test-12345"

	path1 := SocketPath(configDir, sessionID)
	path2 := SocketPath(configDir, sessionID)

	assert.Equal(t, path1, path2, "same session ID should produce same path")
}
