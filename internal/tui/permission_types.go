// Package tui provides permission dialog types for tool-specific rendering.
// Each tool type has a specialized dialog that shows contextually relevant
// information (bash shows command text, file-write shows path and preview,
// file-edit shows path and diff, etc.).
package tui

// PermissionDialogType identifies which specialized permission dialog to show.
type PermissionDialogType int

const (
	// PermDialogFallback is the generic permission dialog for unknown tools.
	PermDialogFallback PermissionDialogType = iota
	// PermDialogBash shows command text, working directory, and sandbox status.
	PermDialogBash
	// PermDialogFileWrite shows the target file path and optional content preview.
	PermDialogFileWrite
	// PermDialogFileEdit shows the target file path and diff preview.
	PermDialogFileEdit
	// PermDialogFilesystem is used for read-only filesystem tools (Read, Glob, Grep).
	PermDialogFilesystem
	// PermDialogWebFetch shows the URL being fetched.
	PermDialogWebFetch
	// PermDialogPlanMode shows the plan mode entry/exit with a description.
	PermDialogPlanMode
	// PermDialogSandbox shows sandbox network permission requests.
	PermDialogSandbox
	// PermDialogMCP shows MCP tool invocations with server name.
	PermDialogMCP
)

// PermissionRequestDetails carries tool-specific metadata for the permission dialog.
type PermissionRequestDetails struct {
	// Common fields
	ToolName    string
	ToolInput   string // JSON or human-readable summary of the tool input
	Description string // What the tool will do

	// Dialog type selection
	DialogType PermissionDialogType

	// Bash-specific
	Command    string // The bash command to execute
	WorkingDir string // Working directory for the command
	IsSandbox  bool   // Whether running in sandbox mode
	IsReadOnly bool   // Whether this is a read-only command

	// File-specific
	FilePath    string // Target file path
	DiffPreview string // Diff preview for file-edit operations
	NewContent  string // Content preview for file-write operations (truncated)

	// Web-specific
	URL string // URL to fetch

	// MCP-specific
	ServerName string // MCP server name

	// Plan mode
	PlanDescription string // Description of the plan

	// Permission options
	ShowAlwaysAllow bool // Whether to show "always allow" option
	ShowDeny        bool // Whether to show deny option (default true)

	// Permission rule context
	MatchedRule   string // Description of the matched permission rule, if any
	RuleSuggested bool   // Whether a rule suggestion is available
}

// PermissionOption represents a choice in the permission dialog.
type PermissionOption struct {
	Label string
	Value string
}

// DefaultPermissionOptions returns the standard y/n/a options.
func DefaultPermissionOptions(showAlways bool) []PermissionOption {
	opts := []PermissionOption{
		{Label: "Yes", Value: "yes"},
		{Label: "No", Value: "no"},
	}
	if showAlways {
		opts = append(opts, PermissionOption{Label: "Always allow", Value: "always"})
	}
	return opts
}

// BashPermissionOptions returns bash-specific options including prefix-based
// and directory-based always-allow variants.
func BashPermissionOptions(command string, showAlways bool) []PermissionOption {
	opts := []PermissionOption{
		{Label: "Yes", Value: "yes"},
		{Label: "No", Value: "no"},
	}
	if showAlways {
		opts = append(opts,
			PermissionOption{Label: "Always allow this tool", Value: "always"},
		)
	}
	return opts
}

// PermissionDialogTypeForTool returns the appropriate dialog type for a tool name.
func PermissionDialogTypeForTool(toolName string) PermissionDialogType {
	switch toolName {
	case "Bash", "bash":
		return PermDialogBash
	case "Write", "file_write", "FileWrite":
		return PermDialogFileWrite
	case "Edit", "file_edit", "FileEdit":
		return PermDialogFileEdit
	case "Read", "file_read", "FileRead", "Glob", "glob", "Grep", "grep":
		return PermDialogFilesystem
	case "WebFetch", "web_fetch":
		return PermDialogWebFetch
	case "EnterPlanMode", "ExitPlanMode":
		return PermDialogPlanMode
	default:
		// Check for MCP tool pattern (contains "(MCP)" suffix or server prefix)
		if len(toolName) > 6 && toolName[len(toolName)-5:] == "(MCP)" {
			return PermDialogMCP
		}
		return PermDialogFallback
	}
}
