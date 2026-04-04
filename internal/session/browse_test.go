package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListSessions_NonExistentDirectory(t *testing.T) {
	sessions, err := ListSessions("/nonexistent/path/that/does/not/exist", 50)
	assert.NoError(t, err, "should not error for non-existent directory")
	assert.Empty(t, sessions, "should return empty list")
}

func TestListSessions_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	// Create the projects/<hash> directory structure
	projectPath := "/test/project"
	sessionDir := filepath.Join(tmpDir, "projects", hashPath(projectPath))
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Override the config dir to use our temp dir
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	sessions, err := ListSessions(projectPath, 50)
	assert.NoError(t, err)
	assert.Empty(t, sessions, "should return empty list for directory with no .jsonl files")
}

func TestListSessions_SortedByDateDescending(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := "/test/sort-project"
	sessionDir := filepath.Join(tmpDir, "projects", hashPath(projectPath))
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	// Create 3 session files with different modification times
	sessions := []struct {
		id      string
		content string
		age     time.Duration // how far in the past
	}{
		{id: "session-oldest", content: makeSessionEntry("oldest message"), age: 3 * time.Hour},
		{id: "session-middle", content: makeSessionEntry("middle message"), age: 1 * time.Hour},
		{id: "session-newest", content: makeSessionEntry("newest message"), age: 0},
	}

	now := time.Now()
	for _, s := range sessions {
		path := filepath.Join(sessionDir, s.id+".jsonl")
		require.NoError(t, os.WriteFile(path, []byte(s.content+"\n"), 0644))
		modTime := now.Add(-s.age)
		require.NoError(t, os.Chtimes(path, modTime, modTime))
	}

	result, err := ListSessions(projectPath, 50)
	require.NoError(t, err)
	require.Len(t, result, 3)

	// Should be sorted newest first
	assert.Equal(t, "session-newest", result[0].ID)
	assert.Equal(t, "session-middle", result[1].ID)
	assert.Equal(t, "session-oldest", result[2].ID)
}

func TestListSessions_RespectsLimit(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := "/test/limit-project"
	sessionDir := filepath.Join(tmpDir, "projects", hashPath(projectPath))
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	// Create 5 session files
	for i := 0; i < 5; i++ {
		path := filepath.Join(sessionDir, fmt.Sprintf("session-%d.jsonl", i))
		content := makeSessionEntry(fmt.Sprintf("message %d", i))
		require.NoError(t, os.WriteFile(path, []byte(content+"\n"), 0644))
	}

	result, err := ListSessions(projectPath, 2)
	require.NoError(t, err)
	assert.Len(t, result, 2, "should return at most 2 sessions")
}

func TestGetSessionPreview_ExtractsFirstUserMessage(t *testing.T) {
	entries := []Entry{
		makeEntry("system", "You are a helpful assistant"),
		makeEntry("user", "Fix the authentication bug"),
		makeEntry("assistant", "I'll look into that"),
	}

	preview := GetSessionPreview(entries)
	assert.Equal(t, "Fix the authentication bug", preview)
}

func TestGetSessionPreview_TruncatesLongMessages(t *testing.T) {
	longMsg := "This is a very long message that exceeds one hundred characters and should be truncated with an ellipsis suffix to keep the preview compact"
	entries := []Entry{
		makeEntry("user", longMsg),
	}

	preview := GetSessionPreview(entries)
	assert.Len(t, preview, 103, "should be 100 chars + '...'")
	assert.True(t, len(preview) <= 103)
	assert.Contains(t, preview, "...")
}

func TestGetSessionPreview_EmptyEntries(t *testing.T) {
	preview := GetSessionPreview(nil)
	assert.Empty(t, preview)
}

func TestGetSessionPreview_NoUserMessage(t *testing.T) {
	entries := []Entry{
		makeEntry("system", "You are helpful"),
		makeEntry("assistant", "Hello!"),
	}
	preview := GetSessionPreview(entries)
	assert.Empty(t, preview, "should return empty string when no user message exists")
}

func TestFormatSessionList_EmptySessions(t *testing.T) {
	result := FormatSessionList(nil)
	assert.Equal(t, "No sessions found.", result)
}

func TestFormatSessionList_ProducesFormattedOutput(t *testing.T) {
	sessions := []SessionInfo{
		{
			ID:           "abc123",
			StartTime:    time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC),
			MessageCount: 42,
			FirstMessage: "Fix the authentication bug in login",
		},
		{
			ID:           "def456",
			StartTime:    time.Date(2026, 4, 2, 15, 0, 0, 0, time.UTC),
			MessageCount: 15,
			FirstMessage: "Create a new endpoint",
		},
	}

	result := FormatSessionList(sessions)
	assert.Contains(t, result, "ID")
	assert.Contains(t, result, "Date")
	assert.Contains(t, result, "Messages")
	assert.Contains(t, result, "Preview")
	assert.Contains(t, result, "abc123")
	assert.Contains(t, result, "2026-04-03")
	assert.Contains(t, result, "42")
	assert.Contains(t, result, "Fix the authentication bug")
	assert.Contains(t, result, "def456")
	assert.Contains(t, result, "2026-04-02")
	assert.Contains(t, result, "15")
}

// Helper functions for tests

func makeSessionEntry(text string) string {
	msg := api.Message{
		Role: "user",
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: text},
		},
	}
	msgData, _ := json.Marshal(msg)
	entry := Entry{
		Type:    "user",
		Message: msgData,
	}
	data, _ := json.Marshal(entry)
	return string(data)
}

func makeEntry(role, text string) Entry {
	msg := api.Message{
		Role: role,
		Content: []api.ContentBlock{
			{Type: api.ContentText, Text: text},
		},
	}
	data, _ := json.Marshal(msg)
	return Entry{
		Type:    role,
		Message: data,
	}
}
