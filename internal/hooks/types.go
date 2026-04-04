// Package hooks implements the hook lifecycle system for ClawGo.
// Hooks are user-configured shell commands that fire before and after
// tool execution events, enabling custom automation such as linting
// after file writes or auditing before bash commands.
package hooks

import "encoding/json"

// HookEvent identifies the lifecycle point at which a hook fires.
// Values match the TypeScript HOOK_EVENTS array exactly.
type HookEvent string

const (
	PreToolUse         HookEvent = "PreToolUse"
	PostToolUse        HookEvent = "PostToolUse"
	PostToolUseFailure HookEvent = "PostToolUseFailure"
	Notification       HookEvent = "Notification"
	UserPromptSubmit   HookEvent = "UserPromptSubmit"
	SessionStart       HookEvent = "SessionStart"
	SessionEnd         HookEvent = "SessionEnd"
	Stop               HookEvent = "Stop"
	StopFailure        HookEvent = "StopFailure"
	SubagentStart      HookEvent = "SubagentStart"
	SubagentStop       HookEvent = "SubagentStop"
	PreCompact         HookEvent = "PreCompact"
	PostCompact        HookEvent = "PostCompact"
	PermissionRequest  HookEvent = "PermissionRequest"
	PermissionDenied   HookEvent = "PermissionDenied"
	Setup              HookEvent = "Setup"
	TeammateIdle       HookEvent = "TeammateIdle"
	TaskCreated        HookEvent = "TaskCreated"
	TaskCompleted      HookEvent = "TaskCompleted"
	Elicitation        HookEvent = "Elicitation"
	ElicitationResult  HookEvent = "ElicitationResult"
	ConfigChange       HookEvent = "ConfigChange"
	WorktreeCreate     HookEvent = "WorktreeCreate"
	WorktreeRemove     HookEvent = "WorktreeRemove"
	InstructionsLoaded HookEvent = "InstructionsLoaded"
	CwdChanged         HookEvent = "CwdChanged"
	FileChanged        HookEvent = "FileChanged"
)

// AllHookEvents lists every valid hook event for validation and enumeration.
var AllHookEvents = []HookEvent{
	PreToolUse, PostToolUse, PostToolUseFailure, Notification,
	UserPromptSubmit, SessionStart, SessionEnd, Stop, StopFailure,
	SubagentStart, SubagentStop, PreCompact, PostCompact,
	PermissionRequest, PermissionDenied, Setup, TeammateIdle,
	TaskCreated, TaskCompleted, Elicitation, ElicitationResult,
	ConfigChange, WorktreeCreate, WorktreeRemove, InstructionsLoaded,
	CwdChanged, FileChanged,
}

// HookCommandType identifies the execution mechanism for a hook.
// Phase 6 implements "command" only; other types return a clear error.
type HookCommandType string

const (
	CommandType HookCommandType = "command"
	PromptType  HookCommandType = "prompt"
	HTTPType    HookCommandType = "http"
	AgentType   HookCommandType = "agent"
)

// HookCommand describes a single hook action to execute.
type HookCommand struct {
	// Type selects the hook execution mechanism.
	Type HookCommandType `json:"type"`

	// Command is the shell command string (for type "command").
	Command string `json:"command,omitempty"`

	// If is an optional matcher pattern using permission rule syntax
	// (e.g., "Bash(git *)"). Only runs if the tool call matches.
	If string `json:"if,omitempty"`

	// Shell is the shell interpreter (default "bash"). Matches TS "bash" | "powershell".
	Shell string `json:"shell,omitempty"`

	// Timeout in seconds for hook execution (default 30).
	Timeout int `json:"timeout,omitempty"`

	// StatusMessage is a custom message displayed while the hook runs.
	StatusMessage string `json:"statusMessage,omitempty"`

	// Once means the hook runs at most once per session and is skipped after.
	Once bool `json:"once,omitempty"`

	// Async means the hook runs in a background goroutine without blocking.
	Async bool `json:"async,omitempty"`
}

// HookMatcher associates a tool-name pattern with a list of hooks.
// An empty Matcher matches all tools.
type HookMatcher struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []HookCommand `json:"hooks"`
}

// HooksConfig maps hook events to their matcher configurations.
// Not all events need entries -- missing events simply have no hooks.
type HooksConfig map[HookEvent][]HookMatcher

// HookInput carries contextual data passed to hook commands.
// Serialized as JSON and provided to hooks via environment variables.
type HookInput struct {
	ToolName    string          `json:"tool_name,omitempty"`
	ToolInput   json.RawMessage `json:"tool_input,omitempty"`
	SessionID   string          `json:"session_id"`
	ProjectRoot string          `json:"cwd"`
}

// HookResult captures the outcome of a single hook execution.
type HookResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`

	// Blocked is true when a pre-hook returned non-zero exit code,
	// signalling that the associated tool execution should be prevented.
	Blocked bool `json:"blocked"`
}
