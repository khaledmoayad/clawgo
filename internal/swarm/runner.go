package swarm

// runner.go implements the backend abstraction layer for teammate execution,
// porting the TypeScript utils/swarm/backends/types.ts TeammateExecutor interface.
// This provides a unified interface for spawning, messaging, and terminating
// teammates regardless of the execution backend (in-process, tmux, iTerm2).

// BackendType identifies the execution backend for a teammate.
type BackendType string

const (
	// BackendInProcess runs the teammate in the same Go process using goroutines
	// with isolated context. This is the primary backend for ClawGo.
	BackendInProcess BackendType = "in-process"

	// BackendTmux runs the teammate in a tmux pane (future implementation).
	BackendTmux BackendType = "tmux"

	// BackendITerm2 runs the teammate in an iTerm2 split pane (future implementation).
	BackendITerm2 BackendType = "iterm2"
)

// SystemPromptMode controls how a custom system prompt is applied to a teammate.
type SystemPromptMode string

const (
	// SystemPromptDefault uses the standard system prompt (no custom prompt applied).
	SystemPromptDefault SystemPromptMode = "default"

	// SystemPromptReplace completely replaces the default system prompt.
	SystemPromptReplace SystemPromptMode = "replace"

	// SystemPromptAppend appends the custom prompt after the default system prompt.
	SystemPromptAppend SystemPromptMode = "append"
)

// TeammateSpawnConfig holds all configuration needed to spawn a new teammate.
// This matches the TypeScript TeammateSpawnConfig from backends/types.ts.
type TeammateSpawnConfig struct {
	// Name is the agent name (e.g., "researcher", "tester").
	// Must not contain '@' -- use SanitizeAgentName if unsure.
	Name string

	// TeamName is the team this teammate belongs to.
	TeamName string

	// Prompt is the initial task prompt for the teammate.
	Prompt string

	// Cwd is the working directory for the teammate.
	Cwd string

	// Color is an optional display color for UI differentiation.
	Color string

	// PlanModeRequired determines whether the teammate must get plan approval
	// before making changes.
	PlanModeRequired bool

	// Model overrides the model for this teammate. Empty uses the default.
	Model string

	// SystemPrompt provides custom system prompt text.
	// How it's applied depends on SystemPromptMode.
	SystemPrompt string

	// SystemPromptMode controls how SystemPrompt is applied: default, replace, or append.
	SystemPromptMode SystemPromptMode

	// WorktreePath is an optional git worktree path for isolated changes.
	WorktreePath string

	// ParentSessionID links this teammate to the parent session for context.
	ParentSessionID string

	// Permissions lists tool names this teammate is allowed to use.
	// Empty or nil means all tools ('*').
	Permissions []string

	// AllowPermissionPrompts controls whether the teammate can show permission
	// prompts for unlisted tools. When false, unlisted tools are auto-denied.
	AllowPermissionPrompts bool
}

// AgentID returns the deterministic agent ID for this spawn config.
func (c *TeammateSpawnConfig) AgentID() string {
	return FormatAgentID(c.Name, c.TeamName)
}

// TeammateSpawnResult is returned after a teammate spawn attempt.
type TeammateSpawnResult struct {
	// Success indicates whether the spawn succeeded.
	Success bool

	// AgentID is the deterministic agent ID (agentName@teamName).
	AgentID string

	// TaskID is the internal task store ID (for progress tracking and UI).
	TaskID string

	// Error contains the error message if spawn failed.
	Error string
}

// TeammateMessage represents a message sent to a teammate.
type TeammateMessage struct {
	// Text is the message content.
	Text string

	// From identifies the sender agent ID.
	From string

	// Color is the sender's display color.
	Color string

	// Timestamp is an ISO 8601 timestamp. Empty uses current time.
	Timestamp string

	// Summary is a 5-10 word preview shown in the UI.
	Summary string
}

// TeammateExecutor is the unified interface for teammate lifecycle management.
// It abstracts differences between execution backends (in-process, tmux, iTerm2).
// This matches the TypeScript TeammateExecutor from backends/types.ts.
type TeammateExecutor interface {
	// Type returns the backend type identifier.
	Type() BackendType

	// IsAvailable checks if this executor's backend is available on the system.
	IsAvailable() (bool, error)

	// Spawn creates a new teammate with the given configuration.
	Spawn(config TeammateSpawnConfig) (*TeammateSpawnResult, error)

	// SendMessage delivers a message to a running teammate.
	SendMessage(agentID string, msg TeammateMessage) error

	// Terminate requests graceful shutdown of a teammate.
	// The teammate processes the request and may approve (exit) or reject (continue).
	Terminate(agentID string, reason string) (bool, error)

	// Kill forcefully terminates a teammate immediately.
	Kill(agentID string) (bool, error)

	// IsActive checks if a teammate is still running.
	IsActive(agentID string) (bool, error)
}

// IsPaneBackend returns true if the backend type uses terminal panes (tmux or iTerm2).
func IsPaneBackend(bt BackendType) bool {
	return bt == BackendTmux || bt == BackendITerm2
}
