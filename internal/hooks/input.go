package hooks

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
)

// HookInput carries contextual data passed to hook commands.
// Serialized as JSON and provided to hooks via the ARGUMENTS environment
// variable. Fields match the Claude Code TypeScript BaseHookInput plus
// per-event extensions.
type HookInput struct {
	// Core context (BaseHookInput fields from TS)
	SessionID      string `json:"session_id"`
	ProjectRoot    string `json:"cwd"`
	TranscriptPath string `json:"transcript_path,omitempty"`
	AgentID        string `json:"agent_id,omitempty"`
	AgentType      string `json:"agent_type,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	HookID         string `json:"hook_id,omitempty"` // unique ID per hook execution

	// Hook event discriminator (matches TS hook_event_name)
	HookEventName string `json:"hook_event_name,omitempty"`

	// Tool context (PreToolUse, PostToolUse, PostToolUseFailure, PermissionRequest, PermissionDenied)
	ToolName  string          `json:"tool_name,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`

	// PostToolUse: tool response
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`

	// PostToolUseFailure: error info
	Error       string `json:"error,omitempty"`
	IsInterrupt bool   `json:"is_interrupt,omitempty"`

	// Notification context
	Title            string `json:"title,omitempty"`
	Message          string `json:"message,omitempty"`
	NotificationType string `json:"notification_type,omitempty"`

	// UserPromptSubmit context
	Prompt string `json:"prompt,omitempty"`

	// PermissionRequest: permission suggestions
	PermissionSuggestions json.RawMessage `json:"permission_suggestions,omitempty"`

	// PermissionDenied: reason
	Reason string `json:"reason,omitempty"`

	// SessionStart context
	Source string `json:"source,omitempty"` // "startup", "resume", "clear", "compact"
	Model  string `json:"model,omitempty"`

	// SessionEnd context
	ExitReason string `json:"exit_reason,omitempty"`

	// Stop/StopFailure context
	StopHookActive       bool            `json:"stop_hook_active,omitempty"`
	LastAssistantMessage string          `json:"last_assistant_message,omitempty"`
	ErrorDetails         string          `json:"error_details,omitempty"`
	StopError            json.RawMessage `json:"stop_error,omitempty"`

	// SubagentStart/SubagentStop context
	SubagentID             string `json:"subagent_id,omitempty"`
	AgentTranscriptPath    string `json:"agent_transcript_path,omitempty"`
	SubagentAgentType      string `json:"subagent_agent_type,omitempty"`

	// Compact context
	CompactTrigger      string `json:"trigger,omitempty"` // "manual", "auto"
	CustomInstructions  string `json:"custom_instructions,omitempty"`
	CompactSummary      string `json:"compact_summary,omitempty"`

	// Setup context
	SetupTrigger string `json:"setup_trigger,omitempty"` // "init", "maintenance"

	// TeammateIdle context
	TeammateName string `json:"teammate_name,omitempty"`
	TeamName     string `json:"team_name,omitempty"`

	// Task context
	TaskID          string `json:"task_id,omitempty"`
	TaskSubject     string `json:"task_subject,omitempty"`
	TaskDescription string `json:"task_description,omitempty"`

	// Elicitation context
	MCPServerName   string          `json:"mcp_server_name,omitempty"`
	ElicitationMode string          `json:"mode,omitempty"` // "form", "url"
	ElicitationURL  string          `json:"url,omitempty"`
	ElicitationID   string          `json:"elicitation_id,omitempty"`
	RequestedSchema json.RawMessage `json:"requested_schema,omitempty"`

	// ElicitationResult context
	Action  string          `json:"action,omitempty"` // "accept", "decline", "cancel"
	Content json.RawMessage `json:"content,omitempty"`

	// ConfigChange context
	ConfigSource string `json:"config_source,omitempty"` // "user_settings", "project_settings", etc.

	// InstructionsLoaded context
	FilePath        string   `json:"file_path,omitempty"`
	MemoryType      string   `json:"memory_type,omitempty"` // "User", "Project", "Local", "Managed"
	LoadReason      string   `json:"load_reason,omitempty"` // "session_start", "nested_traversal", etc.
	Globs           []string `json:"globs,omitempty"`
	TriggerFilePath string   `json:"trigger_file_path,omitempty"`
	ParentFilePath  string   `json:"parent_file_path,omitempty"`

	// WorktreeCreate context
	WorktreeName string `json:"name,omitempty"`

	// WorktreeRemove context
	WorktreePath string `json:"worktree_path,omitempty"`

	// CwdChanged context
	OldCwd string `json:"old_cwd,omitempty"`
	NewCwd string `json:"new_cwd,omitempty"`

	// FileChanged context
	FileEvent string `json:"event,omitempty"` // "change", "add", "unlink"

	// Extra fields for extensibility
	Extra map[string]any `json:"extra,omitempty"`
}

// GenerateHookID creates a cryptographically random hook execution ID.
// Format: 16 hex characters (8 random bytes).
func GenerateHookID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback: should never happen with crypto/rand
		return "0000000000000000"
	}
	return fmt.Sprintf("%x", b)
}

// HookJSONOutput represents the parsed JSON output from a hook's stdout.
// Matches the Claude Code hookJSONOutputSchema union type.
type HookJSONOutput struct {
	// Sync response fields
	Continue       *bool  `json:"continue,omitempty"`
	SuppressOutput bool   `json:"suppressOutput,omitempty"`
	StopReason     string `json:"stopReason,omitempty"`
	Decision       string `json:"decision,omitempty"` // "approve", "block"
	Reason         string `json:"reason,omitempty"`
	SystemMessage  string `json:"systemMessage,omitempty"`

	// Hook-specific output (raw JSON for event-specific processing)
	HookSpecificOutput json.RawMessage `json:"hookSpecificOutput,omitempty"`

	// Async response fields
	Async        bool `json:"async,omitempty"`
	AsyncTimeout int  `json:"asyncTimeout,omitempty"`
}
