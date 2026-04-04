package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/khaledmoayad/clawgo/internal/app"
)

// Config holds configuration for the direct-connect server.
type Config struct {
	// Addr is the address to listen on. Default: "127.0.0.1:0" (random port, localhost only).
	Addr string
	// AllowOrigins is the list of allowed WebSocket origin patterns.
	AllowOrigins []string
}

// Server is a direct-connect WebSocket server that IDE extensions connect to
// for ClawGo capabilities. It listens on localhost and provides a health
// endpoint and a WebSocket endpoint for bidirectional communication.
type Server struct {
	config     Config
	httpServer *http.Server
	sessions   *SessionManager
	mu         sync.Mutex
	port       int
}

// NewServer creates a new direct-connect server with the given configuration.
func NewServer(cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:0"
	}
	return &Server{
		config:   cfg,
		sessions: NewSessionManager(),
	}
}

// Start begins listening and serving. It returns the actual port number
// (useful when Addr specifies port 0 for random assignment).
func (s *Server) Start(ctx context.Context) (int, error) {
	listener, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		return 0, fmt.Errorf("server: listen on %s: %w", s.config.Addr, err)
	}

	// Extract the actual port from the listener address
	addr := listener.Addr().(*net.TCPAddr)
	s.mu.Lock()
	s.port = addr.Port
	s.mu.Unlock()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Serve in background
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash; the server may have been stopped intentionally
			_ = err
		}
	}()

	// Register cleanup for graceful shutdown
	app.RegisterCleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Stop(shutdownCtx)
	})

	return s.port, nil
}

// handleHealth responds with a simple JSON health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleWS accepts a WebSocket connection, creates a session, and enters
// a read loop to handle messages from the IDE extension.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	opts := &websocket.AcceptOptions{}
	if len(s.config.AllowOrigins) > 0 {
		opts.OriginPatterns = s.config.AllowOrigins
	} else {
		// Allow all origins in development
		opts.InsecureSkipVerify = true
	}

	conn, err := websocket.Accept(w, r, opts)
	if err != nil {
		http.Error(w, "websocket accept failed", http.StatusInternalServerError)
		return
	}

	// Generate session ID
	sessionID := generateSessionID()
	sess := &Session{
		ID:        sessionID,
		Conn:      conn,
		CreatedAt: time.Now(),
	}
	s.sessions.Add(sess)

	defer func() {
		s.sessions.Remove(sessionID)
		conn.Close(websocket.StatusNormalClosure, "session ended")
	}()

	// Read loop: receive messages from the IDE extension
	ctx := r.Context()
	for {
		var msg json.RawMessage
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			// Client disconnected or read error
			return
		}

		// Dispatch message to handler
		s.handleMessage(ctx, sess, msg)
	}
}

// handleMessage processes an incoming message from an IDE extension.
// Currently a no-op placeholder for future message dispatch.
func (s *Server) handleMessage(ctx context.Context, sess *Session, msg json.RawMessage) {
	// TODO: Implement message dispatch based on message type
	// For now, echo back the message
	_ = wsjson.Write(ctx, sess.Conn, msg)
}

// Stop gracefully shuts down the server, closing all sessions and the
// HTTP listener.
func (s *Server) Stop(ctx context.Context) error {
	s.sessions.CloseAll()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// Port returns the port the server is listening on.
// Returns 0 if the server hasn't been started.
func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.port
}

// Sessions returns the session manager for external inspection.
func (s *Server) Sessions() *SessionManager {
	return s.sessions
}

// generateSessionID creates a random hex session identifier.
func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
