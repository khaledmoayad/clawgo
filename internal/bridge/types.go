package bridge

// WorkData carries the type and ID embedded in a WorkResponse.
type WorkData struct {
	Type string `json:"type"` // "session" or "healthcheck"
	ID   string `json:"id"`
}

// WorkResponse represents a unit of work received from the bridge API.
type WorkResponse struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"` // "work"
	EnvironmentID string   `json:"environment_id"`
	State         string   `json:"state"`
	Data          WorkData `json:"data"`
	Secret        string   `json:"secret"` // base64url-encoded JSON
	CreatedAt     string   `json:"created_at"`
}

// WorkSecret is the decoded payload from WorkResponse.Secret.
type WorkSecret struct {
	Version              int                `json:"version"`
	SessionIngressToken  string             `json:"session_ingress_token"`
	APIBaseURL           string             `json:"api_base_url"`
	Sources              []WorkSecretSource `json:"sources"`
	Auth                 []WorkSecretAuth   `json:"auth"`
	ClaudeCodeArgs       map[string]string  `json:"claude_code_args,omitempty"`
	MCPConfig            interface{}        `json:"mcp_config,omitempty"`
	EnvironmentVariables map[string]string  `json:"environment_variables,omitempty"`
	UseCodeSessions      bool               `json:"use_code_sessions,omitempty"`
}

// WorkSecretSource describes a code source in the work secret.
type WorkSecretSource struct {
	Type    string              `json:"type"`
	GitInfo *WorkSecretGitInfo  `json:"git_info,omitempty"`
}

// WorkSecretGitInfo contains git repository information.
type WorkSecretGitInfo struct {
	Type  string `json:"type"`
	Repo  string `json:"repo"`
	Ref   string `json:"ref,omitempty"`
	Token string `json:"token,omitempty"`
}

// WorkSecretAuth describes an auth entry in the work secret.
type WorkSecretAuth struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

// SessionDoneStatus indicates how a session completed.
type SessionDoneStatus string

const (
	SessionDoneStatusCompleted   SessionDoneStatus = "completed"
	SessionDoneStatusFailed      SessionDoneStatus = "failed"
	SessionDoneStatusInterrupted SessionDoneStatus = "interrupted"
)

// SessionActivityType categorizes a session activity event.
type SessionActivityType string

const (
	SessionActivityToolStart SessionActivityType = "tool_start"
	SessionActivityText      SessionActivityType = "text"
	SessionActivityResult    SessionActivityType = "result"
	SessionActivityError     SessionActivityType = "error"
)

// SessionActivity records an activity event within a session.
type SessionActivity struct {
	Type      SessionActivityType `json:"type"`
	Summary   string              `json:"summary"`
	Timestamp int64               `json:"timestamp"`
}

// HeartbeatResponse holds the server's response to a heartbeat request.
type HeartbeatResponse struct {
	LeaseExtended bool   `json:"lease_extended"`
	State         string `json:"state"`
	LastHeartbeat string `json:"last_heartbeat"`
	TTLSeconds    int    `json:"ttl_seconds"`
}
