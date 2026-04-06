package sdk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSDKSessionSave(t *testing.T) {
	// SaveSession should write JSONL entries to the correct session dir path.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectRoot := "/test/sdk-project"
	engine := &QueryEngine{
		config: QueryEngineConfig{
			ProjectRoot: projectRoot,
		},
		sessionID: "sdk-test-session",
		messages: []api.Message{
			{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "hello"}}},
			{Role: "assistant", Content: []api.ContentBlock{{Type: api.ContentText, Text: "hi there"}}},
		},
	}

	err := engine.SaveSession()
	require.NoError(t, err)

	// Verify the session file was written to the correct path
	sessionPath := session.GetSessionPath(projectRoot, "sdk-test-session")
	assert.FileExists(t, sessionPath)

	// Verify the file has JSONL entries
	entries, err := session.LoadSession(sessionPath)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "should have 2 entries (user + assistant)")

	// Verify entry types
	assert.Equal(t, "user", entries[0].Type)
	assert.Equal(t, "assistant", entries[1].Type)
}

func TestSDKSessionSave_CreatesDirectory(t *testing.T) {
	// SaveSession should create the session directory if it does not exist.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectRoot := "/test/new-project"
	sessionDir := session.GetSessionDir(projectRoot)

	// Verify directory does not exist yet
	_, err := os.Stat(sessionDir)
	assert.True(t, os.IsNotExist(err))

	engine := &QueryEngine{
		config: QueryEngineConfig{
			ProjectRoot: projectRoot,
		},
		sessionID: "new-session",
		messages: []api.Message{
			{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "test"}}},
		},
	}

	err = engine.SaveSession()
	require.NoError(t, err)

	// Directory should now exist
	_, err = os.Stat(sessionDir)
	assert.NoError(t, err)
}

func TestSDKSessionLoad(t *testing.T) {
	// LoadSDKSession should read JSONL entries and reconstruct api.Message slice.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectRoot := "/test/load-project"

	// Write a session file manually using the session package
	sessionPath := session.GetSessionPath(projectRoot, "load-test")
	require.NoError(t, os.MkdirAll(filepath.Dir(sessionPath), 0755))

	ct := session.NewChainTracker()

	// Write user message
	userMsg := api.Message{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "what is Go?"}}}
	tm1 := session.TranscriptFromMessage(userMsg, ct, session.SerializedMessage{
		SessionID: "load-test",
		Timestamp: "2026-04-05T12:00:00Z",
		Version:   "1.0.0",
	})
	require.NoError(t, session.AppendTranscriptMessage(sessionPath, tm1))

	// Write assistant message
	assistantMsg := api.Message{Role: "assistant", Content: []api.ContentBlock{{Type: api.ContentText, Text: "Go is a programming language."}}}
	tm2 := session.TranscriptFromMessage(assistantMsg, ct, session.SerializedMessage{
		SessionID: "load-test",
		Timestamp: "2026-04-05T12:01:00Z",
		Version:   "1.0.0",
	})
	require.NoError(t, session.AppendTranscriptMessage(sessionPath, tm2))

	// Load via SDK function
	messages, err := LoadSDKSession(projectRoot, "load-test")
	require.NoError(t, err)
	require.Len(t, messages, 2)

	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)

	// Verify content round-tripped
	assert.Len(t, messages[0].Content, 1)
	assert.Equal(t, "what is Go?", messages[0].Content[0].Text)
	assert.Len(t, messages[1].Content, 1)
	assert.Equal(t, "Go is a programming language.", messages[1].Content[0].Text)
}

func TestSDKSessionLoad_RoundTrip(t *testing.T) {
	// Round-trip: save messages via engine, load them back, content matches.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectRoot := "/test/roundtrip-project"
	originalMessages := []api.Message{
		{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "first question"}}},
		{Role: "assistant", Content: []api.ContentBlock{{Type: api.ContentText, Text: "first answer"}}},
		{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "second question"}}},
		{Role: "assistant", Content: []api.ContentBlock{{Type: api.ContentText, Text: "second answer"}}},
	}

	engine := &QueryEngine{
		config: QueryEngineConfig{
			ProjectRoot: projectRoot,
		},
		sessionID: "roundtrip-session",
		messages:  originalMessages,
	}

	// Save
	err := engine.SaveSession()
	require.NoError(t, err)

	// Load
	loaded, err := LoadSDKSession(projectRoot, "roundtrip-session")
	require.NoError(t, err)
	require.Len(t, loaded, len(originalMessages))

	// Verify each message
	for i, orig := range originalMessages {
		assert.Equal(t, orig.Role, loaded[i].Role, "message %d role mismatch", i)
		require.Len(t, loaded[i].Content, len(orig.Content), "message %d content length mismatch", i)
		for j, cb := range orig.Content {
			assert.Equal(t, cb.Text, loaded[i].Content[j].Text, "message %d block %d text mismatch", i, j)
		}
	}
}

func TestSDKSessionLoad_Nonexistent(t *testing.T) {
	// LoadSDKSession should return empty slice for non-existent session.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	messages, err := LoadSDKSession("/test/nonexistent-project", "no-such-session")
	require.NoError(t, err)
	assert.Empty(t, messages)
}

func TestSDKNewQueryEngineFromSession(t *testing.T) {
	// NewQueryEngineFromSession should create an engine pre-populated with messages.
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectRoot := "/test/resume-project"

	// Write a session file
	sessionPath := session.GetSessionPath(projectRoot, "resume-session")
	require.NoError(t, os.MkdirAll(filepath.Dir(sessionPath), 0755))

	ct := session.NewChainTracker()
	userMsg := api.Message{Role: "user", Content: []api.ContentBlock{{Type: api.ContentText, Text: "hello"}}}
	tm := session.TranscriptFromMessage(userMsg, ct, session.SerializedMessage{
		SessionID: "resume-session",
		Timestamp: "2026-04-05T12:00:00Z",
		Version:   "1.0.0",
	})
	require.NoError(t, session.AppendTranscriptMessage(sessionPath, tm))

	assistantMsg := api.Message{Role: "assistant", Content: []api.ContentBlock{{Type: api.ContentText, Text: "hi"}}}
	tm2 := session.TranscriptFromMessage(assistantMsg, ct, session.SerializedMessage{
		SessionID: "resume-session",
		Timestamp: "2026-04-05T12:01:00Z",
		Version:   "1.0.0",
	})
	require.NoError(t, session.AppendTranscriptMessage(sessionPath, tm2))

	// Create engine from session
	cfg := QueryEngineConfig{
		ProjectRoot: projectRoot,
	}
	engine, err := NewQueryEngineFromSession(cfg, projectRoot, "resume-session")
	require.NoError(t, err)
	require.NotNil(t, engine)

	// Engine should have the loaded messages
	msgs := engine.Messages()
	require.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)

	// Session ID should be preserved
	assert.Equal(t, "resume-session", engine.SessionID())
}
