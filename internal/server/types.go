package server

// SessionState represents the lifecycle state of an IDE server session.
type SessionState string

const (
	// StateStarting indicates the session is being initialized.
	StateStarting SessionState = "starting"
	// StateRunning indicates the session is active and connected.
	StateRunning SessionState = "running"
	// StateDetached indicates the client disconnected but the session is still alive.
	StateDetached SessionState = "detached"
	// StateStopping indicates the session is shutting down.
	StateStopping SessionState = "stopping"
	// StateStopped indicates the session has been terminated.
	StateStopped SessionState = "stopped"
)

// ServerConfig holds extended configuration for the IDE direct-connect server.
// Matches the TS ServerConfig type from server/types.ts.
type ServerConfig struct {
	// Port is the port to listen on. 0 means random.
	Port int `json:"port"`
	// Host is the hostname/IP to bind to.
	Host string `json:"host"`
	// AuthToken is the bearer token required for API access. Empty means no auth.
	AuthToken string `json:"authToken"`
	// UnixSocket is an optional Unix socket path to listen on instead of TCP.
	UnixSocket string `json:"unix,omitempty"`
	// IdleTimeoutMs is how long a detached session stays alive before stopping (milliseconds).
	IdleTimeoutMs int `json:"idleTimeoutMs,omitempty"`
	// MaxSessions is the maximum number of concurrent sessions. 0 means unlimited.
	MaxSessions int `json:"maxSessions,omitempty"`
	// Workspace is the default working directory for new sessions.
	Workspace string `json:"workspace,omitempty"`
}

// SessionInfo holds metadata about an active or recent session.
// Matches the TS SessionInfo type from server/types.ts.
type SessionInfo struct {
	// ID is the unique session identifier.
	ID string `json:"id"`
	// Status is the current lifecycle state of the session.
	Status SessionState `json:"status"`
	// CreatedAt is the Unix timestamp (milliseconds) when the session was created.
	CreatedAt int64 `json:"createdAt"`
	// WorkDir is the working directory for this session.
	WorkDir string `json:"workDir"`
	// SessionKey is an optional key for session authentication.
	SessionKey string `json:"sessionKey,omitempty"`
}

// ConnectResponse is the response body for POST /sessions.
// Matches the TS ConnectResponse type.
type ConnectResponse struct {
	// SessionID is the newly created session identifier.
	SessionID string `json:"session_id"`
	// WsURL is the WebSocket URL to connect to for this session.
	WsURL string `json:"ws_url"`
	// WorkDir is the working directory for the session.
	WorkDir string `json:"work_dir,omitempty"`
}

// CreateSessionRequest is the request body for POST /sessions.
type CreateSessionRequest struct {
	// Cwd is the desired working directory for the session.
	Cwd string `json:"cwd"`
	// DangerouslySkipPermissions bypasses permission checks if true.
	DangerouslySkipPermissions bool `json:"dangerously_skip_permissions"`
}

// SessionIndexEntry represents a session in the session index file.
type SessionIndexEntry struct {
	// SessionID is the session identifier.
	SessionID string `json:"sessionId"`
	// TranscriptSessionID links to the transcript storage.
	TranscriptSessionID string `json:"transcriptSessionId"`
	// Cwd is the working directory.
	Cwd string `json:"cwd"`
	// PermissionMode describes how permissions are handled.
	PermissionMode string `json:"permissionMode"`
	// CreatedAt is the Unix timestamp (milliseconds) of creation.
	CreatedAt int64 `json:"createdAt"`
	// LastActiveAt is the Unix timestamp (milliseconds) of last activity.
	LastActiveAt int64 `json:"lastActiveAt"`
}
