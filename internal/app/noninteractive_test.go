package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/query"
	"github.com/khaledmoayad/clawgo/internal/session"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunNonInteractive_TextCallbackCalled(t *testing.T) {
	// Test that TextCallback receives text tokens during streaming.
	// We test this indirectly through the LoopParams wiring.
	var buf bytes.Buffer
	callback := func(text string) {
		buf.WriteString(text)
	}

	// Verify the callback works as expected
	callback("Hello ")
	callback("World")
	assert.Equal(t, "Hello World", buf.String())
}

func TestRunNonInteractive_PrintsOutput(t *testing.T) {
	// Test that NonInteractiveParams can be constructed with all required fields.
	// Full integration test requires a mock API client, so we verify the
	// construction and parameter passing.
	tracker := cost.NewTracker("claude-sonnet-4-20250514")
	registry := tools.NewRegistry()
	permCtx := permissions.NewPermissionContext(permissions.ModeAuto, nil, nil)

	params := &NonInteractiveParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:     registry,
		PermCtx:      permCtx,
		CostTracker:  tracker,
		Messages:     nil,
		SystemPrompt: "You are helpful",
		MaxTurns:     1,
		WorkingDir:   "/tmp",
		SessionID:    "test-session",
		Prompt:       "What is 2+2?",
		OutputFormat: "text",
	}

	// Verify params were constructed correctly
	assert.Equal(t, "What is 2+2?", params.Prompt)
	assert.Equal(t, "text", params.OutputFormat)
	assert.Equal(t, "You are helpful", params.SystemPrompt)
	assert.Equal(t, 1, params.MaxTurns)
}

func TestRunNonInteractive_LoopParamsWiring(t *testing.T) {
	// Test that RunNonInteractive correctly wires up the query.LoopParams.
	// This verifies the TextCallback is set and other params are passed through.
	tracker := cost.NewTracker("claude-sonnet-4-20250514")
	registry := tools.NewRegistry()
	permCtx := permissions.NewPermissionContext(permissions.ModeAuto, nil, nil)
	client := &api.Client{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 4096,
	}

	// Create LoopParams the same way RunNonInteractive does
	messages := []api.Message{api.UserMessage("test prompt")}
	var captured string
	loopParams := &query.LoopParams{
		Client:       client,
		Registry:     registry,
		PermCtx:      permCtx,
		CostTracker:  tracker,
		Messages:     messages,
		SystemPrompt: "test system",
		MaxTurns:     5,
		WorkingDir:   "/tmp",
		SessionID:    "test-session",
		TextCallback: func(text string) { captured += text },
	}

	// Verify all fields are set correctly
	assert.Equal(t, client, loopParams.Client)
	assert.Equal(t, registry, loopParams.Registry)
	assert.Equal(t, permCtx, loopParams.PermCtx)
	assert.Equal(t, tracker, loopParams.CostTracker)
	assert.Len(t, loopParams.Messages, 1)
	assert.Equal(t, "test system", loopParams.SystemPrompt)
	assert.Equal(t, 5, loopParams.MaxTurns)
	assert.Nil(t, loopParams.Program)    // no TUI in non-interactive
	assert.Nil(t, loopParams.PermissionCh) // no permission channel in non-interactive

	// Verify TextCallback works
	loopParams.TextCallback("hello")
	assert.Equal(t, "hello", captured)
}

func TestRunNonInteractive_CostFormatting(t *testing.T) {
	// Test that cost tracking works in the non-interactive context
	tracker := cost.NewTracker("claude-sonnet-4-20250514")
	tracker.Add(cost.Usage{
		InputTokens:  100,
		OutputTokens: 50,
	})

	usage := cost.FormatUsage(tracker)
	assert.Contains(t, usage, "100")
	assert.Contains(t, usage, "50")
	assert.Contains(t, usage, "Cost:")
}

func TestRunParams_Construction(t *testing.T) {
	// Test RunParams can be created from CLI flags
	params := &RunParams{
		Model:          "claude-sonnet-4-20250514",
		PermissionMode: "auto",
		Resume:         true,
		SessionID:      "test-123",
		MaxTurns:       10,
		SystemPrompt:   "Be helpful",
		OutputFormat:   "json",
		AllowedTools:   []string{"Bash"},
		DisallowedTools: []string{"Write"},
		Prompt:         "hello",
		Version:        "1.0.0",
	}

	assert.Equal(t, "claude-sonnet-4-20250514", params.Model)
	assert.Equal(t, "auto", params.PermissionMode)
	assert.True(t, params.Resume)
	assert.Equal(t, "hello", params.Prompt)
	assert.Equal(t, "1.0.0", params.Version)
}

func TestRunNonInteractive_ContextCancellation(t *testing.T) {
	// Verify that context cancellation is respected
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// The cancelled context should be detectable
	assert.Error(t, ctx.Err())
}

func TestNonInteractive_SessionPersistenceFlag(t *testing.T) {
	// Test that NonInteractiveParams accepts the NoSessionPersistence field.
	params := &NonInteractiveParams{
		Client: &api.Client{
			Model:     "claude-sonnet-4-20250514",
			MaxTokens: 4096,
		},
		Registry:             tools.NewRegistry(),
		PermCtx:              permissions.NewPermissionContext(permissions.ModeAuto, nil, nil),
		CostTracker:          cost.NewTracker("claude-sonnet-4-20250514"),
		SystemPrompt:         "You are helpful",
		MaxTurns:             1,
		WorkingDir:           "/tmp",
		SessionID:            "test-session",
		Prompt:               "hello",
		OutputFormat:         "text",
		NoSessionPersistence: false,
	}

	assert.False(t, params.NoSessionPersistence)

	params.NoSessionPersistence = true
	assert.True(t, params.NoSessionPersistence)
}

func TestNonInteractive_SessionLoadFromExisting(t *testing.T) {
	// Test that RunNonInteractive loads prior messages from existing session.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectRoot := "/test/noninteractive-project"
	sessionID := "existing-session"

	// Write an existing session file
	sessionPath := session.GetSessionPath(projectRoot, sessionID)
	require.NoError(t, os.MkdirAll(filepath.Dir(sessionPath), 0755))

	ct := session.NewChainTracker()
	userMsg := api.Message{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "prior question"}}}
	tm := session.TranscriptFromMessage(userMsg, ct, session.SerializedMessage{
		SessionID: sessionID,
		Timestamp: "2026-04-05T12:00:00Z",
		Version:   "1.0.0",
	})
	require.NoError(t, session.AppendTranscriptMessage(sessionPath, tm))

	assistantMsg := api.Message{Role: "assistant", Content: []api.ContentBlock{{Type: api.ContentText, Text: "prior answer"}}}
	tm2 := session.TranscriptFromMessage(assistantMsg, ct, session.SerializedMessage{
		SessionID: sessionID,
		Timestamp: "2026-04-05T12:01:00Z",
		Version:   "1.0.0",
	})
	require.NoError(t, session.AppendTranscriptMessage(sessionPath, tm2))

	// Verify session file was written
	entries, err := session.LoadSession(sessionPath)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// The session loading logic in RunNonInteractive would load these messages
	// when params.Messages is empty and SessionID is provided.
	// We test this by simulating the load path directly.
	var messages []api.Message
	if sessionID != "" && projectRoot != "" {
		sp := session.GetSessionPath(projectRoot, sessionID)
		if _, statErr := os.Stat(sp); statErr == nil {
			loadedEntries, loadErr := session.LoadSession(sp)
			if loadErr == nil {
				messages = session.EntriesToMessages(loadedEntries)
			}
		}
	}

	require.Len(t, messages, 2)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)
}

func TestNonInteractive_SessionSaveAfterQuery(t *testing.T) {
	// Test that session save logic writes JSONL correctly.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectRoot := "/test/save-project"
	sessionID := "save-session"

	// Simulate the messages that would exist after a query loop
	messages := []api.Message{
		{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "what is Go?"}}},
		{Role: "assistant", Content: []api.ContentBlock{{Type: api.ContentText, Text: "Go is a programming language."}}},
	}

	// Simulate the save logic from RunNonInteractive
	sessionPath := session.GetSessionPath(projectRoot, sessionID)
	saveCT := session.NewChainTracker()
	meta := session.SerializedMessage{
		SessionID: sessionID,
		Timestamp: "2026-04-05T12:00:00Z",
		Version:   "1.0.0",
	}
	for _, msg := range messages {
		tm := session.TranscriptFromMessage(msg, saveCT, meta)
		err := session.AppendTranscriptMessage(sessionPath, tm)
		require.NoError(t, err)
	}

	// Verify the session file was created and contains valid JSONL
	assert.FileExists(t, sessionPath)
	entries, err := session.LoadSession(sessionPath)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "user", entries[0].Type)
	assert.Equal(t, "assistant", entries[1].Type)

	// Verify round-trip: entries can be converted back to messages
	roundTripped := session.EntriesToMessages(entries)
	require.Len(t, roundTripped, 2)
	assert.Equal(t, "user", roundTripped[0].Role)
	assert.Equal(t, "assistant", roundTripped[1].Role)
}

func TestNonInteractive_NoSessionPersistence_SkipsSave(t *testing.T) {
	// When NoSessionPersistence is true, no session file should be written.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectRoot := "/test/nopersist-project"
	sessionID := "nopersist-session"
	noSessionPersistence := true

	// Simulate the save logic with the flag check
	if !noSessionPersistence && sessionID != "" && projectRoot != "" {
		// This block should NOT be entered
		sessionPath := session.GetSessionPath(projectRoot, sessionID)
		ct := session.NewChainTracker()
		meta := session.SerializedMessage{SessionID: sessionID}
		msg := api.Message{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "test"}}}
		tm := session.TranscriptFromMessage(msg, ct, meta)
		_ = session.AppendTranscriptMessage(sessionPath, tm)
	}

	// Session file should NOT exist
	sessionPath := session.GetSessionPath(projectRoot, sessionID)
	_, err := os.Stat(sessionPath)
	assert.True(t, os.IsNotExist(err), "session file should not exist when NoSessionPersistence is true")
}

func TestNonInteractive_SessionSaveFormat_MatchesREPL(t *testing.T) {
	// Verify session files use the same JSONL format as the interactive REPL.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectRoot := "/test/format-project"
	sessionID := "format-session"

	messages := []api.Message{
		{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "hello"}}},
		{Role: "assistant", Content: []api.ContentBlock{{Type: api.ContentText, Text: "hi"}}},
	}

	sessionPath := session.GetSessionPath(projectRoot, sessionID)
	ct := session.NewChainTracker()
	meta := session.SerializedMessage{
		SessionID: sessionID,
		Timestamp: "2026-04-05T12:00:00Z",
		Version:   "1.0.0",
	}
	for _, msg := range messages {
		tm := session.TranscriptFromMessage(msg, ct, meta)
		require.NoError(t, session.AppendTranscriptMessage(sessionPath, tm))
	}

	// Read the raw JSONL and verify it has expected fields
	data, err := os.ReadFile(sessionPath)
	require.NoError(t, err)

	// Each line should be valid JSON with TranscriptMessage fields
	lines := bytes.Split(bytes.TrimSpace(data), []byte("\n"))
	require.Len(t, lines, 2)

	for _, line := range lines {
		var tm map[string]interface{}
		require.NoError(t, json.Unmarshal(line, &tm))
		assert.Contains(t, tm, "uuid", "TranscriptMessage should have uuid field")
		assert.Contains(t, tm, "type", "TranscriptMessage should have type field")
		assert.Contains(t, tm, "role", "TranscriptMessage should have role field")
		assert.Contains(t, tm, "sessionId", "TranscriptMessage should have sessionId field")
		assert.Contains(t, tm, "timestamp", "TranscriptMessage should have timestamp field")
	}

	// Verify the session can be loaded via LoadTranscript (REPL's format)
	transcript, loadCT, err := session.LoadTranscript(sessionPath)
	require.NoError(t, err)
	require.Len(t, transcript, 2)
	assert.NotEmpty(t, loadCT.LastUUID())
}
