package cli

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCmd_Version(t *testing.T) {
	cmd := NewRootCmd("v0.1.0-test")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--version"})
	err := cmd.Execute()
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "clawgo")
	assert.Contains(t, output, "v0.1.0-test")
}

func TestNewRootCmd_Help(t *testing.T) {
	cmd := NewRootCmd("v0.1.0")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "--model")
	assert.Contains(t, output, "--permission-mode")
	assert.Contains(t, output, "--resume")
	assert.Contains(t, output, "--verbose")
	assert.Contains(t, output, "--session-id")
	assert.Contains(t, output, "--max-turns")
	assert.Contains(t, output, "--output-format")
	assert.Contains(t, output, "--system-prompt")
}

func TestNewRootCmd_PositionalArg(t *testing.T) {
	// Set a fake API key so app.Run() doesn't fail on key resolution.
	// The test will still fail on the actual API call, but that's expected
	// in non-interactive mode -- we just verify flag parsing works.
	t.Setenv("ANTHROPIC_API_KEY", "test-key-for-cli-test")

	cmd := NewRootCmd("v0.1.0")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"do something"})
	// This will attempt to connect to the API but the key is fake.
	// We accept that the error is from the API, not from argument parsing.
	err := cmd.Execute()
	// With a fake key, this will get a network/auth error, which is fine.
	// The important thing is that the positional arg was accepted.
	if err != nil {
		// Verify it's NOT an argument parsing error
		assert.NotContains(t, err.Error(), "unknown command")
		assert.NotContains(t, err.Error(), "accepts at most")
	}
}

func TestNewRootCmd_UnknownFlag(t *testing.T) {
	cmd := NewRootCmd("v0.1.0")
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--nonexistent"})
	err := cmd.Execute()
	assert.Error(t, err)
}

func TestBuildCGO(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build test in short mode")
	}
	// Build from the go/ directory (parent of internal/cli/)
	cmd := exec.Command("go", "build", "./cmd/clawgo/")
	cmd.Dir = "../.." // go/ is two levels up from internal/cli/
	cmd.Env = append(cmd.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("CGO_ENABLED=0 build failed: %v\n%s", err, string(output))
	}
	// Verify no output means success
	assert.True(t, len(strings.TrimSpace(string(output))) == 0 || err == nil,
		"build should succeed with no error output")
}
