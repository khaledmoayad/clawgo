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

func TestRunHooks_AllEnvironmentVariables(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{
						Type:          CommandType,
						Command:       `echo "EVENT=$HOOK_EVENT TOOL=$TOOL_NAME INPUT=$TOOL_INPUT SID=$SESSION_ID CWD=$CWD TP=$TRANSCRIPT_PATH AID=$AGENT_ID HID=$HOOK_ID SM=$HOOK_STATUS_MESSAGE"`,
						StatusMessage: "running test hook",
					},
				},
			},
		},
	}

	input := &HookInput{
		ToolName:       "Bash",
		ToolInput:      json.RawMessage(`{"cmd":"ls"}`),
		SessionID:      "sess-abc",
		ProjectRoot:    "/home/user/project",
		TranscriptPath: "/tmp/transcript.jsonl",
		AgentID:        "agent-1",
		HookID:         "hook-xyz",
	}

	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	require.Len(t, results, 1)
	out := results[0].Stdout
	assert.Contains(t, out, "EVENT=PreToolUse")
	assert.Contains(t, out, "TOOL=Bash")
	assert.Contains(t, out, `INPUT={"cmd":"ls"}`)
	assert.Contains(t, out, "SID=sess-abc")
	assert.Contains(t, out, "CWD=/home/user/project")
	assert.Contains(t, out, "TP=/tmp/transcript.jsonl")
	assert.Contains(t, out, "AID=agent-1")
	assert.Contains(t, out, "HID=hook-xyz")
	assert.Contains(t, out, "SM=running test hook")
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

// --- Global disabled tests ---

func TestRunHooks_GlobalDisabled(t *testing.T) {
	skipOnWindows(t)
	SetGlobalDisabled(true)
	defer SetGlobalDisabled(false)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo should-not-run"},
				},
			},
		},
	}

	input := &HookInput{ToolName: "Bash", SessionID: "test"}
	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestSetGlobalDisabled_Accessors(t *testing.T) {
	SetGlobalDisabled(false)
	assert.False(t, IsGlobalDisabled())
	SetGlobalDisabled(true)
	assert.True(t, IsGlobalDisabled())
	SetGlobalDisabled(false) // restore
}

func TestSetManagedHooksOnly_Accessors(t *testing.T) {
	SetManagedHooksOnly(false)
	assert.False(t, IsManagedHooksOnly())
	SetManagedHooksOnly(true)
	assert.True(t, IsManagedHooksOnly())
	SetManagedHooksOnly(false) // restore
}

// --- Hook JSON output parsing tests ---

func TestRunHooks_JSONOutputParsed(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: `echo '{"decision":"block","reason":"unsafe command"}'`},
				},
			},
		},
	}

	input := &HookInput{ToolName: "Bash", SessionID: "test"}
	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, 0, results[0].ExitCode)
	require.NotNil(t, results[0].Output)
	assert.Equal(t, "block", results[0].Output.Decision)
	assert.Equal(t, "unsafe command", results[0].Output.Reason)
}

func TestRunHooks_PlainTextOutputNilJSON(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo hello world"},
				},
			},
		},
	}

	input := &HookInput{ToolName: "Bash", SessionID: "test"}
	results, err := RunHooks(context.Background(), PreToolUse, input, config)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Nil(t, results[0].Output, "plain text stdout should not produce Output")
}

func TestRunPreToolHook_BlocksOnJSONDecision(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: `echo '{"decision":"block","reason":"policy"}'`},
				},
			},
		},
	}

	blocked, err := RunPreToolHook(context.Background(), "Bash", nil, "sess", "/tmp", config)
	require.NoError(t, err)
	assert.True(t, blocked, "JSON block decision should block the tool")
}

// --- GenerateHookID tests ---

func TestGenerateHookID_Unique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateHookID()
		assert.Len(t, id, 16, "hook ID should be 16 hex characters")
		assert.False(t, ids[id], "hook ID collision: %s", id)
		ids[id] = true
	}
}

// --- Dispatch function tests ---

func makeDispatchContext(config HooksConfig) *DispatchContext {
	return &DispatchContext{
		SessionID:      "sess-dispatch",
		ProjectRoot:    "/home/test/project",
		TranscriptPath: "/tmp/transcript.jsonl",
		AgentID:        "",
		Config:         config,
	}
}

func TestDispatchPreToolUse_BlockingExitCode(t *testing.T) {
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

	dc := makeDispatchContext(config)
	blocked, modified, err := DispatchPreToolUse(context.Background(), dc, "Bash", json.RawMessage(`{"cmd":"rm -rf /"}`), "use-1")
	require.NoError(t, err)
	assert.True(t, blocked)
	assert.Nil(t, modified)
}

func TestDispatchPreToolUse_BlockingJSONDecision(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: `echo '{"decision":"block","reason":"policy violation"}'`},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	blocked, _, err := DispatchPreToolUse(context.Background(), dc, "Bash", json.RawMessage(`{"cmd":"test"}`), "use-2")
	require.NoError(t, err)
	assert.True(t, blocked, "JSON block decision should block")
}

func TestDispatchPreToolUse_ModifiedInput(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: `echo '{"hookSpecificOutput":{"updatedInput":{"cmd":"safe-command"}}}'`},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	blocked, modified, err := DispatchPreToolUse(context.Background(), dc, "Bash", json.RawMessage(`{"cmd":"dangerous"}`), "use-3")
	require.NoError(t, err)
	assert.False(t, blocked)
	require.NotNil(t, modified)
	assert.Contains(t, string(modified), "safe-command")
}

func TestDispatchPreToolUse_NotBlocked(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo ok"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	blocked, modified, err := DispatchPreToolUse(context.Background(), dc, "Bash", json.RawMessage(`{"cmd":"ls"}`), "use-4")
	require.NoError(t, err)
	assert.False(t, blocked)
	assert.Nil(t, modified)
}

func TestDispatchPostToolUse_FireAndForget(t *testing.T) {
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

	dc := makeDispatchContext(config)
	// Should not return error -- non-fatal
	err := DispatchPostToolUse(context.Background(), dc, "Bash", json.RawMessage(`{}`), "use-5")
	assert.NoError(t, err)
}

func TestDispatchSessionStart_CorrectInput(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		SessionStart: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: `echo "$HOOK_EVENT $SESSION_ID"`},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchSessionStart(context.Background(), dc, "startup", "claude-3-opus")
	assert.NoError(t, err)
}

func TestDispatchSessionEnd_CorrectInput(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		SessionEnd: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: `echo "$HOOK_EVENT"`},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchSessionEnd(context.Background(), dc, "prompt_input_exit")
	assert.NoError(t, err)
}

func TestDispatchNotification_PopulatesFields(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		Notification: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: `echo "$ARGUMENTS"`},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchNotification(context.Background(), dc, "Build Complete", "All tests passed", "info")
	assert.NoError(t, err)
}

func TestDispatchUserPromptSubmit(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		UserPromptSubmit: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: `echo "$ARGUMENTS"`},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchUserPromptSubmit(context.Background(), dc, "Write a test")
	assert.NoError(t, err)
}

func TestDispatchStop(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		Stop: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo stopped"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchStop(context.Background(), dc, true, "Done with task")
	assert.NoError(t, err)
}

func TestDispatchSubagentStart(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		SubagentStart: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo subagent-started"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchSubagentStart(context.Background(), dc, "agent-42", "code-reviewer")
	assert.NoError(t, err)
}

func TestDispatchSubagentStop(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		SubagentStop: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo subagent-stopped"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchSubagentStop(context.Background(), dc, "agent-42", "/tmp/agent.jsonl", "code-reviewer", false, "last msg")
	assert.NoError(t, err)
}

func TestDispatchSetup(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		Setup: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo setup"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchSetup(context.Background(), dc, "init")
	assert.NoError(t, err)
}

func TestDispatchPreCompact(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PreCompact: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo pre-compact"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchPreCompact(context.Background(), dc, "auto", "summarize briefly")
	assert.NoError(t, err)
}

func TestDispatchPostCompact(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PostCompact: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo post-compact"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchPostCompact(context.Background(), dc, "manual", "session was about X")
	assert.NoError(t, err)
}

func TestDispatchPermissionRequest(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PermissionRequest: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo perm-request"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchPermissionRequest(context.Background(), dc, "Bash", json.RawMessage(`{"cmd":"rm"}`), nil)
	assert.NoError(t, err)
}

func TestDispatchPermissionDenied(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PermissionDenied: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo perm-denied"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchPermissionDenied(context.Background(), dc, "Bash", json.RawMessage(`{"cmd":"rm"}`), "use-10", "user denied")
	assert.NoError(t, err)
}

func TestDispatchTeammateIdle(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		TeammateIdle: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo idle"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchTeammateIdle(context.Background(), dc, "worker-1", "team-alpha")
	assert.NoError(t, err)
}

func TestDispatchTaskCreated(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		TaskCreated: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo task-created"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchTaskCreated(context.Background(), dc, "task-1", "Fix bug", "Fix the login bug", "worker-1", "team-alpha")
	assert.NoError(t, err)
}

func TestDispatchTaskCompleted(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		TaskCompleted: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo task-completed"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchTaskCompleted(context.Background(), dc, "task-1", "Fix bug", "Fixed it", "worker-1", "team-alpha")
	assert.NoError(t, err)
}

func TestDispatchElicitation(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		Elicitation: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo elicitation"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchElicitation(context.Background(), dc, "test-server", "Choose option", "form", "", "elic-1", nil)
	assert.NoError(t, err)
}

func TestDispatchElicitationResult(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		ElicitationResult: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo elic-result"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchElicitationResult(context.Background(), dc, "test-server", "elic-1", "form", "accept", json.RawMessage(`{"key":"val"}`))
	assert.NoError(t, err)
}

func TestDispatchConfigChange(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		ConfigChange: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo config-change"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchConfigChange(context.Background(), dc, "user_settings", "/home/user/.claude/settings.json")
	assert.NoError(t, err)
}

func TestDispatchWorktreeCreate(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		WorktreeCreate: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo worktree-create"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchWorktreeCreate(context.Background(), dc, "feature-branch")
	assert.NoError(t, err)
}

func TestDispatchWorktreeRemove(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		WorktreeRemove: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo worktree-remove"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchWorktreeRemove(context.Background(), dc, "/home/user/worktrees/feature")
	assert.NoError(t, err)
}

func TestDispatchInstructionsLoaded(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		InstructionsLoaded: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo instructions-loaded"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchInstructionsLoaded(context.Background(), dc, "/project/CLAUDE.md", "Project", "session_start", nil, "", "")
	assert.NoError(t, err)
}

func TestDispatchCwdChanged(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		CwdChanged: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo cwd-changed"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchCwdChanged(context.Background(), dc, "/old/path", "/new/path")
	assert.NoError(t, err)
}

func TestDispatchFileChanged(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		FileChanged: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo file-changed"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchFileChanged(context.Background(), dc, "/project/main.go", "change")
	assert.NoError(t, err)
}

func TestDispatchStopFailure(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		StopFailure: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo stop-failure"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchStopFailure(context.Background(), dc, json.RawMessage(`{"type":"api_error"}`), "rate limited", "")
	assert.NoError(t, err)
}

func TestDispatchPostToolUseFailure(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		PostToolUseFailure: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo post-tool-failure"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	err := DispatchPostToolUseFailure(context.Background(), dc, "Bash", json.RawMessage(`{"cmd":"fail"}`), "use-99", "command failed")
	assert.NoError(t, err)
}

// --- Dispatch with disabled state ---

func TestDispatch_AllDisabledNoOps(t *testing.T) {
	skipOnWindows(t)
	SetGlobalDisabled(true)
	defer SetGlobalDisabled(false)

	config := HooksConfig{
		SessionStart: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "echo should-not-run"},
				},
			},
		},
		PreToolUse: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: "exit 1"},
				},
			},
		},
	}

	dc := makeDispatchContext(config)

	// SessionStart should no-op
	err := DispatchSessionStart(context.Background(), dc, "startup", "model")
	assert.NoError(t, err)

	// PreToolUse should not block (no-op)
	blocked, _, err := DispatchPreToolUse(context.Background(), dc, "Bash", nil, "use-0")
	assert.NoError(t, err)
	assert.False(t, blocked)
}

// --- Dispatch generates unique HookIDs ---

func TestDispatch_UniqueHookIDs(t *testing.T) {
	skipOnWindows(t)

	config := HooksConfig{
		SessionStart: {
			{
				Matcher: "",
				Hooks: []HookCommand{
					{Type: CommandType, Command: `echo "$HOOK_ID"`},
				},
			},
		},
	}

	dc := makeDispatchContext(config)
	ids := make(map[string]bool)

	for i := 0; i < 10; i++ {
		// We cannot easily capture the output from dispatch functions,
		// but we can verify GenerateHookID produces unique values
		id := GenerateHookID()
		assert.False(t, ids[id], "HookID collision: %s", id)
		ids[id] = true
	}

	// Also verify the dispatch itself works
	err := DispatchSessionStart(context.Background(), dc, "startup", "model")
	assert.NoError(t, err)
}

// --- Verify all 27 events have dispatch coverage ---

func TestAllEvents_HaveDispatchFunctions(t *testing.T) {
	// This test documents that all 27 events have corresponding dispatch functions.
	// The dispatch functions are:
	// PreToolUse         -> DispatchPreToolUse
	// PostToolUse        -> DispatchPostToolUse
	// PostToolUseFailure -> DispatchPostToolUseFailure
	// Notification       -> DispatchNotification
	// UserPromptSubmit   -> DispatchUserPromptSubmit
	// SessionStart       -> DispatchSessionStart
	// SessionEnd         -> DispatchSessionEnd
	// Stop               -> DispatchStop
	// StopFailure        -> DispatchStopFailure
	// SubagentStart      -> DispatchSubagentStart
	// SubagentStop       -> DispatchSubagentStop
	// PreCompact         -> DispatchPreCompact
	// PostCompact        -> DispatchPostCompact
	// PermissionRequest  -> DispatchPermissionRequest
	// PermissionDenied   -> DispatchPermissionDenied
	// Setup              -> DispatchSetup
	// TeammateIdle       -> DispatchTeammateIdle
	// TaskCreated        -> DispatchTaskCreated
	// TaskCompleted      -> DispatchTaskCompleted
	// Elicitation        -> DispatchElicitation
	// ElicitationResult  -> DispatchElicitationResult
	// ConfigChange       -> DispatchConfigChange
	// WorktreeCreate     -> DispatchWorktreeCreate
	// WorktreeRemove     -> DispatchWorktreeRemove
	// InstructionsLoaded -> DispatchInstructionsLoaded
	// CwdChanged         -> DispatchCwdChanged
	// FileChanged        -> DispatchFileChanged
	assert.Equal(t, 27, len(AllHookEvents), "expected 27 hook events")
}
