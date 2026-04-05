package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ansiRegexp matches ANSI escape sequences for stripping in tests.
var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestPermissionDialogTypeForTool(t *testing.T) {
	tests := []struct {
		toolName string
		expected PermissionDialogType
	}{
		{"Bash", PermDialogBash},
		{"bash", PermDialogBash},
		{"Write", PermDialogFileWrite},
		{"FileWrite", PermDialogFileWrite},
		{"Edit", PermDialogFileEdit},
		{"FileEdit", PermDialogFileEdit},
		{"Read", PermDialogFilesystem},
		{"Glob", PermDialogFilesystem},
		{"Grep", PermDialogFilesystem},
		{"WebFetch", PermDialogWebFetch},
		{"EnterPlanMode", PermDialogPlanMode},
		{"ExitPlanMode", PermDialogPlanMode},
		{"unknown_tool", PermDialogFallback},
		{"my-tool (MCP)", PermDialogMCP},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := PermissionDialogTypeForTool(tt.toolName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSpecializedPermissionModel_BashDialog(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:   "Bash",
		DialogType: PermDialogBash,
		Command:    "git status",
		WorkingDir: "/home/user/project",
	})

	assert.True(t, m.IsActive())

	view := m.View()
	assert.Contains(t, view, "Bash")
	assert.Contains(t, view, "git status")
	assert.Contains(t, view, "/home/user/project")
}

func TestSpecializedPermissionModel_FileWriteDialog(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:   "Write",
		DialogType: PermDialogFileWrite,
		FilePath:   "/tmp/test.txt",
		NewContent: "Hello, world!\nSecond line",
	})

	view := m.View()
	assert.Contains(t, view, "Write file")
	assert.Contains(t, view, "/tmp/test.txt")
	assert.Contains(t, view, "Hello, world!")
}

func TestSpecializedPermissionModel_FileEditDialog(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:    "Edit",
		DialogType:  PermDialogFileEdit,
		FilePath:    "/tmp/test.go",
		DiffPreview: "-old line\n+new line",
	})

	view := m.View()
	assert.Contains(t, view, "Edit file")
	assert.Contains(t, view, "/tmp/test.go")
	assert.Contains(t, view, "old line")
}

func TestSpecializedPermissionModel_WebFetchDialog(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:   "WebFetch",
		DialogType: PermDialogWebFetch,
		URL:        "https://example.com/api/data",
	})

	view := m.View()
	assert.Contains(t, view, "Web Fetch")
	// Strip ANSI codes for URL check since underline styling wraps per-character
	plain := ansiRegexp.ReplaceAllString(view, "")
	assert.Contains(t, plain, "https://example.com/api/data")
}

func TestSpecializedPermissionModel_PlanModeDialog(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:        "EnterPlanMode",
		DialogType:      PermDialogPlanMode,
		PlanDescription: "Implement auth system",
	})

	view := m.View()
	assert.Contains(t, view, "Plan Mode")
	assert.Contains(t, view, "Implement auth system")
}

func TestSpecializedPermissionModel_SandboxDialog(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:    "sandbox",
		DialogType:  PermDialogSandbox,
		Description: "Allow network access to api.example.com",
	})

	view := m.View()
	assert.Contains(t, view, "Sandbox Network Access")
	assert.Contains(t, view, "api.example.com")
}

func TestSpecializedPermissionModel_MCPDialog(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:    "read_file (MCP)",
		DialogType:  PermDialogMCP,
		ServerName:  "filesystem",
		Description: "Read file from filesystem",
	})

	view := m.View()
	assert.Contains(t, view, "filesystem")
	assert.Contains(t, view, "Read file from filesystem")
}

func TestSpecializedPermissionModel_FallbackDialog(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:   "unknown_tool",
		DialogType: PermDialogFallback,
		ToolInput:  "some input",
	})

	view := m.View()
	// Fallback delegates to base PermissionModel.View
	assert.Contains(t, view, "Permission Required")
}

func TestSpecializedPermissionModel_InactiveReturnsEmpty(t *testing.T) {
	m := NewSpecializedPermissionModel()
	assert.Equal(t, "", m.View())
}

func TestSpecializedPermissionModel_BashSandboxIndicator(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:   "Bash",
		DialogType: PermDialogBash,
		Command:    "npm install",
		IsSandbox:  true,
	})

	view := m.View()
	assert.Contains(t, view, "sandboxed")
}

func TestSpecializedPermissionModel_AlwaysAllowOption(t *testing.T) {
	m := NewSpecializedPermissionModel()
	m.ShowDetailed(PermissionRequestDetails{
		ToolName:        "Bash",
		DialogType:      PermDialogBash,
		Command:         "ls",
		ShowAlwaysAllow: true,
	})

	view := m.View()
	assert.Contains(t, view, "Always approve")
}

func TestDefaultPermissionOptions(t *testing.T) {
	opts := DefaultPermissionOptions(false)
	require.Len(t, opts, 2)
	assert.Equal(t, "yes", opts[0].Value)
	assert.Equal(t, "no", opts[1].Value)

	opts = DefaultPermissionOptions(true)
	require.Len(t, opts, 3)
	assert.Equal(t, "always", opts[2].Value)
}

func TestTruncateContent(t *testing.T) {
	content := strings.Repeat("line\n", 20)
	result := truncateContent(content, 5, 80)
	lines := strings.Split(result, "\n")
	// 5 content lines + 1 "more lines" indicator
	assert.LessOrEqual(t, len(lines), 7)
	assert.Contains(t, result, "more lines")
}

func TestTruncateContent_ShortContent(t *testing.T) {
	content := "short text"
	result := truncateContent(content, 5, 80)
	assert.Equal(t, "short text", result)
}
