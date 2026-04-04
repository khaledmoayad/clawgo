package hooks

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipOnWindows skips tests that depend on bash.
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Skipping test that requires bash on Windows")
	}
}

// --- Matcher tests ---

func TestMatchesToolName_ExactMatch(t *testing.T) {
	assert.True(t, MatchesToolName("Bash", "Bash"))
	assert.True(t, MatchesToolName("FileRead", "FileRead"))
}

func TestMatchesToolName_CaseInsensitive(t *testing.T) {
	assert.True(t, MatchesToolName("bash", "Bash"))
	assert.True(t, MatchesToolName("BASH", "bash"))
	assert.True(t, MatchesToolName("Bash", "BASH"))
}

func TestMatchesToolName_EmptyPattern(t *testing.T) {
	assert.True(t, MatchesToolName("", "Bash"))
	assert.True(t, MatchesToolName("", ""))
	assert.True(t, MatchesToolName("", "AnyToolName"))
}

func TestMatchesToolName_GlobPattern(t *testing.T) {
	assert.True(t, MatchesToolName("File*", "FileRead"))
	assert.True(t, MatchesToolName("File*", "FileWrite"))
	assert.True(t, MatchesToolName("File*", "FileEdit"))
	assert.False(t, MatchesToolName("File*", "Bash"))
}

func TestMatchesToolName_ParenthesizedArguments(t *testing.T) {
	assert.True(t, MatchesToolName("Bash(git *)", "Bash(git push)"))
	assert.True(t, MatchesToolName("Bash(git *)", "Bash(git pull)"))
	assert.False(t, MatchesToolName("Bash(git *)", "Bash(npm install)"))
	assert.False(t, MatchesToolName("Bash(git *)", "FileRead(test.go)"))
}

func TestMatchesToolName_NoMatch(t *testing.T) {
	assert.False(t, MatchesToolName("Bash", "FileRead"))
	assert.False(t, MatchesToolName("Write", "Read"))
}

func TestFilterMatchers(t *testing.T) {
	matchers := []HookMatcher{
		{
			Matcher: "Bash",
			Hooks: []HookCommand{
				{Type: CommandType, Command: "echo bash-hook"},
			},
		},
		{
			Matcher: "FileRead",
			Hooks: []HookCommand{
				{Type: CommandType, Command: "echo read-hook"},
			},
		},
		{
			Matcher: "", // matches everything
			Hooks: []HookCommand{
				{Type: CommandType, Command: "echo global-hook"},
			},
		},
	}

	// Bash should match "Bash" matcher and empty matcher
	result := FilterMatchers(matchers, "Bash")
	assert.Len(t, result, 2)
	assert.Equal(t, "echo bash-hook", result[0].Command)
	assert.Equal(t, "echo global-hook", result[1].Command)

	// FileRead should match "FileRead" matcher and empty matcher
	result = FilterMatchers(matchers, "FileRead")
	assert.Len(t, result, 2)
	assert.Equal(t, "echo read-hook", result[0].Command)
	assert.Equal(t, "echo global-hook", result[1].Command)

	// Unknown tool should only match empty matcher
	result = FilterMatchers(matchers, "Unknown")
	assert.Len(t, result, 1)
	assert.Equal(t, "echo global-hook", result[0].Command)
}

// --- Hook execution tests ---

func TestRunHooks_SimpleEcho(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "Bash",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo hello-from-hook"},
				},
			},
		},
	}

	input := &HookInput{
		ToolName:  "Bash",
		SessionID: "test-session",
	}

	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 0, results[0].ExitCode)
	assert.Contains(t, results[0].Stdout, "hello-from-hook")
}

func TestRunHooks_EnvironmentVariables(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo $HOOK_EVENT $TOOL_NAME $SESSION_ID"},
				},
			},
		},
	}

	input := &HookInput{
		ToolName:  "Bash",
		SessionID: "sess-123",
	}

	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Contains(t, results[0].Stdout, "PreToolUse")
	assert.Contains(t, results[0].Stdout, "Bash")
	assert.Contains(t, results[0].Stdout, "sess-123")
}

func TestRunPreToolHook_BlocksOnNonZeroExit(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "exit 1"},
				},
			},
		},
	}

	blocked, err := RunPreToolHook(context.Background(), "Bash", nil, "sess", "/tmp", config)
	require.NoError(t, err)
	assert.True(t, blocked)
}

func TestRunPreToolHook_DoesNotBlockOnZeroExit(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "exit 0"},
				},
			},
		},
	}

	blocked, err := RunPreToolHook(context.Background(), "Bash", nil, "sess", "/tmp", config)
	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestRunPostToolHook_DoesNotBlock(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PostToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "exit 1"},
				},
			},
		},
	}

	// Should not panic or return error -- fire-and-forget
	RunPostToolHook(context.Background(), "Bash", nil, "sess", "/tmp", config)
}

func TestRunHooks_OnceFlag(t *testing.T) {
	skipOnWindows(t)
	ResetOnceTracker()
	defer ResetOnceTracker()

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo once-hook", Once: true},
				},
			},
		},
	}

	input := &HookInput{ToolName: "Bash", SessionID: "test"}

	// First call should execute
	results1, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	assert.Len(t, results1, 1)
	assert.Contains(t, results1[0].Stdout, "once-hook")

	// Second call should skip the once-hook
	results2, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	assert.Len(t, results2, 0)
}

func TestRunHooks_Timeout(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "sleep 10", Timeout: 1},
				},
			},
		},
	}

	input := &HookInput{ToolName: "Bash", SessionID: "test"}

	start := time.Now()
	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.NotEqual(t, 0, results[0].ExitCode, "timeout should produce non-zero exit code")
	assert.Less(t, elapsed, 5*time.Second, "should timeout well before 10 seconds")
}

func TestRunHooks_UnimplementedType(t *testing.T) {
	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: PromptType, Command: "test prompt"},
				},
			},
		},
	}

	input := &HookInput{ToolName: "Bash", SessionID: "test"}

	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 1, results[0].ExitCode)
	assert.Contains(t, results[0].Stderr, "not yet implemented")
}

func TestRunHooks_NoMatchingEvent(t *testing.T) {
	config := HooksConfig{
		PostToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo test"},
				},
			},
		},
	}

	input := &HookInput{ToolName: "Bash", SessionID: "test"}

	// Query for PreToolUse but config only has PostToolUse
	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestRunHooks_ArgumentsEnvVar(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo $ARGUMENTS"},
				},
			},
		},
	}

	input := &HookInput{
		ToolName:    "Bash",
		ToolInput:   json.RawMessage(`{"command":"ls"}`),
		SessionID:   "test",
		ProjectRoot: "/tmp",
	}

	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	require.Len(t, results, 1)
	// The ARGUMENTS env var should contain the JSON-serialized input
	assert.Contains(t, results[0].Stdout, "tool_name")
	assert.Contains(t, results[0].Stdout, "Bash")
}
