package tui

import (
	"github.com/khaledmoayad/clawgo/internal/api"
)

// Bubble Tea message types for communication between TUI components.

// StreamEventMsg wraps an API stream event for the TUI.
type StreamEventMsg struct {
	Event api.StreamEvent
}

// SubmitMsg is sent when the user submits input.
type SubmitMsg struct {
	Text string
}

// ShellCommandMsg is sent when the user submits input prefixed with "!".
// The Command field contains the shell command with the "!" prefix stripped.
type ShellCommandMsg struct {
	Command string
}

// PermissionRequestMsg asks the user to approve a tool use.
type PermissionRequestMsg struct {
	ToolName    string
	ToolInput   string // human-readable summary of input
	Description string // what the tool will do
}

// PermissionResponseMsg carries the user's permission decision.
type PermissionResponseMsg struct {
	Approved bool
	Always   bool // true if user chose "always allow"
	ToolName string
}

// CostUpdateMsg updates the cost display.
type CostUpdateMsg struct {
	TurnCost    string
	SessionCost string
	Tokens      string
}

// ErrorMsg represents an error to display.
type ErrorMsg struct {
	Err error
}

// QuitMsg signals the application should exit.
type QuitMsg struct{}

// StatusMsg updates the status bar text.
type StatusMsg struct {
	Text string
}

// DiffDisplayMsg explicitly signals diff content from a tool result.
// This allows tool results to bypass auto-detection and render directly as a diff.
type DiffDisplayMsg struct {
	ToolName string
	Content  string
	FilePath string
}

// CommandResultMsg carries the result of a slash command execution back to the TUI.
type CommandResultMsg struct {
	Type    string // "text", "clear", "compact", "model_change", "exit", "skip", "rewind"
	Value   string // Display text or new model name
	Command string // The command name that produced this result (e.g. "help")
}

// DetailedPermissionRequestMsg asks the user to approve a tool use with
// full tool-specific context. This replaces PermissionRequestMsg when the
// query loop has enough context to populate a PermissionRequestDetails.
type DetailedPermissionRequestMsg struct {
	Details PermissionRequestDetails
}

// NotificationMsg signals the TUI to display a toast notification.
type NotificationMsg struct {
	Notification Notification
}

// ShowPermissionRulesMsg signals the TUI to open the permission rules panel.
type ShowPermissionRulesMsg struct {
	Rules []PermissionRuleEntry
}

// --- Wave 1 integration message types ---

// ShowOverlayMsg triggers an overlay from anywhere in the TUI.
// Type is one of "selector", "history", "search", "fullscreen".
type ShowOverlayMsg struct {
	Type string
}

// ShowHelpMsg triggers the help dialog.
type ShowHelpMsg struct{}

// SuggestionUpdateMsg triggers a suggestion refresh for the given input.
type SuggestionUpdateMsg struct {
	Input     string
	CursorPos int
}

// ContextUpdateMsg updates the context usage displayed in the status line.
type ContextUpdateMsg struct {
	Percent int
	Tokens  string
}

// ModelChangeMsg updates the model name displayed in the status line and header.
type ModelChangeMsg struct {
	Name string
}

// ToastMsg triggers a toast notification from anywhere in the TUI.
// Level is "info", "warning", or "error".
type ToastMsg struct {
	Level   string
	Message string
}

// APIErrorMsg carries a classified API error to the TUI for user-facing display.
// This replaces raw error strings with structured error info that includes
// category, user message, and recovery information.
type APIErrorMsg struct {
	// ErrorInfo is the classified error from api.ClassifyAPIError().
	ErrorInfo *api.APIErrorInfo

	// QuotaStatus holds parsed rate limit quota info (nil if not a rate limit error).
	QuotaStatus *api.QuotaStatus
}
