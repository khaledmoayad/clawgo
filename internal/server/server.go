package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
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
	// AuthToken is the bearer token required for API access. Empty means no auth.
	AuthToken string
	// IdleTimeoutMs is how long a detached session stays alive before stopping (milliseconds).
	IdleTimeoutMs int
	// MaxSessions is the maximum number of concurrent sessions. 0 means unlimited.
	MaxSessions int
	// Workspace is the default working directory for new sessions.
	Workspace string
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
	mux.HandleFunc("POST /sessions", s.handleCreateSession)

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

// handleCreateSession handles POST /sessions to create a new direct-connect session.
// Returns a ConnectResponse with session_id, ws_url, and work_dir.
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(w, r) {
		return
	}

	// Check max sessions limit
	if s.config.MaxSessions > 0 && s.sessions.Count() >= s.config.MaxSessions {
		http.Error(w, `{"error":"max sessions reached"}`, http.StatusServiceUnavailable)
		return
	}

	var req CreateSessionRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
	}

	// Determine working directory: request cwd > config workspace > current dir
	workDir := req.Cwd
	if workDir == "" {
		workDir = s.config.Workspace
	}
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	// Generate session ID and create session info
	sessionID := generateSessionID()
	info := &SessionInfo{
		ID:        sessionID,
		Status:    StateStarting,
		CreatedAt: time.Now().UnixMilli(),
		WorkDir:   workDir,
	}
	s.sessions.AddSession(info)

	// Transition to running
	s.sessions.UpdateStatus(sessionID, StateRunning)

	// Build WebSocket URL
	s.mu.Lock()
	port := s.port
	s.mu.Unlock()
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws?session=%s", port, sessionID)

	resp := ConnectResponse{
		SessionID: sessionID,
		WsURL:     wsURL,
		WorkDir:   workDir,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleWS accepts a WebSocket connection and enters a read loop to handle
// messages from the IDE extension. If a session query parameter is provided,
// the connection is associated with that existing session; otherwise a new
// session is created on the fly.
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

	// Check if connecting to an existing session via query param
	sessionID := r.URL.Query().Get("session")
	if sessionID != "" {
		// Validate the session exists
		if info := s.sessions.GetSession(sessionID); info == nil {
			conn.Close(websocket.StatusPolicyViolation, "unknown session")
			return
		}
	} else {
		// Generate a new ad-hoc session ID
		sessionID = generateSessionID()
	}

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

// checkAuth validates the Authorization bearer token if AuthToken is configured.
// Returns true if auth passes (or no auth required), false if unauthorized.
func (s *Server) checkAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.config.AuthToken == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return false
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token != s.config.AuthToken {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return false
	}
	return true
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
