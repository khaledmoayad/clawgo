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
