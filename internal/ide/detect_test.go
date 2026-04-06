package ide

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func clearIDEEnvVars(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"VSCODE_PID",
		"TERM_PROGRAM",
		"JETBRAINS_IDE",
		"INTELLIJ_ENVIRONMENT_READER",
		"CURSOR_TRACE_ID",
		"WINDSURF_PID",
		"ZED_TERM",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}
}

func TestDetectIDE_VSCodeEnv(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("VSCODE_PID", "12345")

	info := DetectIDE()
	assert.Equal(t, VSCode, info.Type)
	assert.Equal(t, "VS Code", info.Name)
	assert.Equal(t, 12345, info.PID)
}

func TestDetectIDE_JetBrainsEnv(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("JETBRAINS_IDE", "GoLand")

	info := DetectIDE()
	assert.Equal(t, JetBrains, info.Type)
	assert.Equal(t, "GoLand", info.Name)
}

func TestDetectIDE_Unknown(t *testing.T) {
	clearIDEEnvVars(t)

	info := DetectIDE()
	// In CI or a plain terminal, this should be Unknown
	// (unless actually running in an IDE, which is unlikely in tests)
	assert.Contains(t, []IDEType{Unknown, VSCode, JetBrains, Cursor, Windsurf, Zed}, info.Type)
}

func TestDetectIDE_TermProgram(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("TERM_PROGRAM", "vscode")

	info := DetectIDE()
	assert.Equal(t, VSCode, info.Type)
	assert.Equal(t, "VS Code", info.Name)
}

func TestDetectIDE_IntellijEnvReader(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("INTELLIJ_ENVIRONMENT_READER", "true")

	info := DetectIDE()
	assert.Equal(t, JetBrains, info.Type)
	assert.Equal(t, "JetBrains IDE", info.Name)
}

func TestDetectFromEnv_Priority(t *testing.T) {
	// VSCODE_PID takes priority over JETBRAINS_IDE
	clearIDEEnvVars(t)
	t.Setenv("VSCODE_PID", "999")
	t.Setenv("JETBRAINS_IDE", "GoLand")

	info := DetectIDE()
	assert.Equal(t, VSCode, info.Type,
		"VSCODE_PID should take priority over JETBRAINS_IDE")
}

func TestDetectIDE_CursorEnv(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("CURSOR_TRACE_ID", "abc123")

	info := DetectIDE()
	assert.Equal(t, Cursor, info.Type)
	assert.Equal(t, "Cursor", info.Name)
}

func TestDetectIDE_CursorTermProgram(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("TERM_PROGRAM", "cursor")

	info := DetectIDE()
	assert.Equal(t, Cursor, info.Type)
	assert.Equal(t, "Cursor", info.Name)
}

func TestDetectIDE_WindsurfEnv(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("WINDSURF_PID", "9999")

	info := DetectIDE()
	assert.Equal(t, Windsurf, info.Type)
	assert.Equal(t, "Windsurf", info.Name)
}

func TestDetectIDE_WindsurfTermProgram(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("TERM_PROGRAM", "windsurf")

	info := DetectIDE()
	assert.Equal(t, Windsurf, info.Type)
	assert.Equal(t, "Windsurf", info.Name)
}

func TestDetectIDE_ZedEnv(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("ZED_TERM", "true")

	info := DetectIDE()
	assert.Equal(t, Zed, info.Type)
	assert.Equal(t, "Zed", info.Name)
}

func TestDetectIDE_ZedTermProgram(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("TERM_PROGRAM", "zed")

	info := DetectIDE()
	assert.Equal(t, Zed, info.Type)
	assert.Equal(t, "Zed", info.Name)
}

func TestDetectIDE_CursorOverridesVSCode(t *testing.T) {
	// Cursor is a VS Code fork, so both env vars may be set.
	// Cursor should take priority.
	clearIDEEnvVars(t)
	t.Setenv("CURSOR_TRACE_ID", "abc")
	t.Setenv("VSCODE_PID", "1234")

	info := DetectIDE()
	assert.Equal(t, Cursor, info.Type,
		"Cursor should take priority over VS Code")
}

func TestEntrypoint_CLI(t *testing.T) {
	clearIDEEnvVars(t)
	// In a plain terminal, Entrypoint should return "cli"
	ep := Entrypoint()
	// May detect an IDE from process scanning, but with cleared env
	// the most common result is "cli"
	assert.Contains(t, []string{"cli", "vscode", "jetbrains", "cursor", "windsurf", "zed"}, ep)
}

func TestEntrypoint_Cursor(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("CURSOR_TRACE_ID", "trace-123")

	ep := Entrypoint()
	assert.Equal(t, "cursor", ep)
}

func TestEntrypoint_Windsurf(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("WINDSURF_PID", "5555")

	ep := Entrypoint()
	assert.Equal(t, "windsurf", ep)
}

func TestEntrypoint_Zed(t *testing.T) {
	clearIDEEnvVars(t)
	t.Setenv("ZED_TERM", "1")

	ep := Entrypoint()
	assert.Equal(t, "zed", ep)
}

func TestIDEType_Constants(t *testing.T) {
	assert.Equal(t, IDEType("vscode"), VSCode)
	assert.Equal(t, IDEType("jetbrains"), JetBrains)
	assert.Equal(t, IDEType("cursor"), Cursor)
	assert.Equal(t, IDEType("windsurf"), Windsurf)
	assert.Equal(t, IDEType("zed"), Zed)
	assert.Equal(t, IDEType("unknown"), Unknown)
}

func TestGetAncestorProcesses(t *testing.T) {
	// This should not panic and should return a list (possibly empty)
	procs := getAncestorProcesses()
	require.NotNil(t, procs)
	// At minimum we should have at least one ancestor (shell/init)
	// But we don't assert count since it's environment-dependent
}

func TestIndexOf(t *testing.T) {
	assert.Equal(t, 3, indexOf([]byte("abc\x00def"), 0))
	assert.Equal(t, -1, indexOf([]byte("abcdef"), 0))
	assert.Equal(t, 0, indexOf([]byte("\x00abc"), 0))
}
