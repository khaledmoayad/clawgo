package uds

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// ParseAddress parses a target address and returns its scheme and target.
// Supported schemes: "uds:" (Unix domain socket), "bridge:" (bridge mode),
// absolute paths (treated as UDS), and anything else as "other".
func ParseAddress(to string) (scheme, target string) {
	switch {
	case strings.HasPrefix(to, "uds:"):
		return "uds", to[4:]
	case strings.HasPrefix(to, "bridge:"):
		return "bridge", to[7:]
	case strings.HasPrefix(to, "/"):
		return "uds", to
	default:
		return "other", to
	}
}

// Listen creates a Unix domain socket listener at the given path.
// Any existing socket file is removed first to avoid "address already in use" errors.
func Listen(socketPath string) (net.Listener, error) {
	// Remove stale socket file
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("removing stale socket: %w", err)
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listening on unix socket %s: %w", socketPath, err)
	}
	return listener, nil
}

// Send connects to a Unix domain socket and sends a JSON-encoded message.
// The connection is closed after the message is sent.
func Send(ctx context.Context, socketPath string, msg interface{}) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return fmt.Errorf("dialing unix socket %s: %w", socketPath, err)
	}
	defer conn.Close()

	// Apply context deadline to the connection if set
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetWriteDeadline(deadline); err != nil {
			return fmt.Errorf("setting write deadline: %w", err)
		}
	}

	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		return fmt.Errorf("encoding message: %w", err)
	}
	return nil
}

// Receive accepts a connection on the listener and reads a JSON message.
// The accepted connection is closed after the message is read.
func Receive(ctx context.Context, listener net.Listener) (json.RawMessage, error) {
	// Apply context deadline to the listener if set
	if deadline, ok := ctx.Deadline(); ok {
		if ul, ok := listener.(*net.UnixListener); ok {
			if err := ul.SetDeadline(deadline); err != nil {
				return nil, fmt.Errorf("setting accept deadline: %w", err)
			}
		}
	}

	conn, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("accepting connection: %w", err)
	}
	defer conn.Close()

	// Apply context deadline to the connection for reading
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetReadDeadline(deadline); err != nil {
			return nil, fmt.Errorf("setting read deadline: %w", err)
		}
	}

	var msg json.RawMessage
	if err := json.NewDecoder(conn).Decode(&msg); err != nil {
		return nil, fmt.Errorf("decoding message: %w", err)
	}
	return msg, nil
}

// SocketPath generates a deterministic, short socket path for a session.
// Uses SHA-256 hash truncated to 16 hex characters to stay well under
// the 108-byte Unix socket path limit.
// The socket is placed in configDir/sockets/ directory (created if needed).
func SocketPath(configDir, sessionID string) string {
	socketsDir := filepath.Join(configDir, "sockets")
	// Best-effort directory creation
	os.MkdirAll(socketsDir, 0755)

	hash := sha256.Sum256([]byte(sessionID))
	short := hex.EncodeToString(hash[:])[:16]
	return filepath.Join(socketsDir, short+".sock")
}
