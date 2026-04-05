package bridge

import "time"

const (
	defaultMaxConcurrentSessions = 5
	defaultPollInterval          = 5 * time.Second
)

// BridgeConfig holds configuration for the bridge mode, matching the
// TypeScript BridgeConfig type from bridge/types.ts.
type BridgeConfig struct {
	// Dir is the working directory for the bridge.
	Dir string
	// EnvironmentName is the display name for this bridge environment (machineName in TS).
	EnvironmentName string
	// Branch is the current git branch.
	Branch string
	// GitRepoURL is the git repository URL, if available.
	GitRepoURL string
	// MaxConcurrentSessions limits the number of simultaneous child sessions.
	// Defaults to 5 if zero.
	MaxConcurrentSessions int
	// SpawnMode controls how session working directories are chosen.
	// One of "single-session", "worktree", "same-dir".
	SpawnMode string
	// Verbose enables verbose logging.
	Verbose bool
	// Sandbox enables sandboxed execution.
	Sandbox bool
	// WorkerType is sent as metadata.worker_type for filtering.
	// Defaults to "claude_code".
	WorkerType string
	// APIBaseURL is the base URL for the bridge API (e.g., "https://api.anthropic.com").
	APIBaseURL string
	// GetToken returns the current auth token for API requests.
	GetToken func() string
	// PollInterval is the duration between work polling requests.
	// Defaults to 5s if zero.
	PollInterval time.Duration
	// SessionTimeoutMs is the per-session timeout in milliseconds.
	SessionTimeoutMs int
	// OnDebug is an optional debug logging callback.
	OnDebug func(string)
}

// Config is an alias for BridgeConfig for backward compatibility.
type Config = BridgeConfig

// withDefaults returns a copy of the config with zero values filled with defaults.
func (c BridgeConfig) withDefaults() BridgeConfig {
	if c.MaxConcurrentSessions <= 0 {
		c.MaxConcurrentSessions = defaultMaxConcurrentSessions
	}
	if c.PollInterval <= 0 {
		c.PollInterval = defaultPollInterval
	}
	if c.WorkerType == "" {
		c.WorkerType = "claude_code"
	}
	if c.SpawnMode == "" {
		c.SpawnMode = "single-session"
	}
	return c
}
