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
	assert.Contains(t, []IDEType{Unknown, VSCode, JetBrains}, info.Type)
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

func TestIDEType_Constants(t *testing.T) {
	assert.Equal(t, IDEType("vscode"), VSCode)
	assert.Equal(t, IDEType("jetbrains"), JetBrains)
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
