package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionID(t *testing.T) {
	id := NewSessionID()
	assert.NotEmpty(t, id)
	assert.Len(t, id, 36) // UUID-like format: 8-4-4-4-12 = 32 hex + 4 hyphens

	// Each call should produce a unique ID
	id2 := NewSessionID()
	assert.NotEqual(t, id, id2)
}

func TestGetSessionDir(t *testing.T) {
	// Override config dir for testing
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	dir := GetSessionDir("/home/user/project")
	assert.Contains(t, dir, "projects")
	assert.Contains(t, dir, tmpDir)
	// Should contain a hash component
	base := filepath.Base(dir)
	assert.Len(t, base, 16) // 8 bytes = 16 hex chars
}

func TestAppendAndLoadSession(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "test-session.jsonl")

	// Append 3 entries
	entries := []Entry{
		{Type: "user", Message: json.RawMessage(`{"role":"user","content":[{"type":"text","text":"hello"}]}`)},
		{Type: "assistant", Message: json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"hi there"}]}`)},
		{Type: "user", Message: json.RawMessage(`{"role":"user","content":[{"type":"text","text":"bye"}]}`)},
	}

	for _, e := range entries {
		err := AppendEntry(sessionPath, e)
		require.NoError(t, err)
	}

	// Load and verify
	loaded, err := LoadSession(sessionPath)
	require.NoError(t, err)
	assert.Len(t, loaded, 3)

	// Verify order and content
	assert.Equal(t, "user", loaded[0].Type)
	assert.Equal(t, "assistant", loaded[1].Type)
	assert.Equal(t, "user", loaded[2].Type)
}

func TestLoadSession_Nonexistent(t *testing.T) {
	entries, err := LoadSession("/nonexistent/path/session.jsonl")
	assert.Error(t, err)
	assert.Nil(t, entries)
}

func TestEntryFromUserMessage(t *testing.T) {
	entry := EntryFromUserMessage("hello world")
	assert.Equal(t, "user", entry.Type)
	assert.NotEmpty(t, entry.Message)

	// Verify the message content
	var msg struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	err := json.Unmarshal(entry.Message, &msg)
	require.NoError(t, err)
	assert.Equal(t, "user", msg.Role)
	assert.Len(t, msg.Content, 1)
	assert.Equal(t, "text", msg.Content[0].Type)
	assert.Equal(t, "hello world", msg.Content[0].Text)
}

func TestEntriesToMessages(t *testing.T) {
	entries := []Entry{
		{Type: "user", Message: json.RawMessage(`{"role":"user","content":[{"type":"text","text":"hello"}]}`)},
		{Type: "assistant", Message: json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"hi"}]}`)},
	}

	messages := EntriesToMessages(entries)
	assert.Len(t, messages, 2)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "assistant", messages[1].Role)
}

func TestFindLatestSession(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectPath := "/test/project"
	sessionDir := GetSessionDir(projectPath)
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Create 2 session files with different times
	file1 := filepath.Join(sessionDir, "session-old.jsonl")
	file2 := filepath.Join(sessionDir, "session-new.jsonl")

	require.NoError(t, os.WriteFile(file1, []byte(`{"type":"user","message":{}}`+"\n"), 0644))
	// Ensure file2 has a newer modification time
	time.Sleep(50 * time.Millisecond)
	require.NoError(t, os.WriteFile(file2, []byte(`{"type":"user","message":{}}`+"\n"), 0644))

	latestPath, err := FindLatestSession(projectPath)
	require.NoError(t, err)
	assert.Equal(t, file2, latestPath)
}

func TestResume_ByID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectPath := "/test/project"
	sessionID := "test-session-123"
	sessionPath := GetSessionPath(projectPath, sessionID)

	// Create session directory and write entries
	require.NoError(t, os.MkdirAll(filepath.Dir(sessionPath), 0755))
	entry := Entry{Type: "user", Message: json.RawMessage(`{"role":"user","content":[{"type":"text","text":"hello"}]}`)}
	require.NoError(t, AppendEntry(sessionPath, entry))

	// Resume by ID
	entries, resolvedID, err := Resume(projectPath, sessionID)
	require.NoError(t, err)
	assert.Equal(t, sessionID, resolvedID)
	assert.Len(t, entries, 1)
	assert.Equal(t, "user", entries[0].Type)
}

func TestResume_Latest(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)

	projectPath := "/test/project"
	sessionDir := GetSessionDir(projectPath)
	require.NoError(t, os.MkdirAll(sessionDir, 0755))

	// Create older session
	old := filepath.Join(sessionDir, "old-session.jsonl")
	require.NoError(t, os.WriteFile(old, []byte(`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"old"}]}}`+"\n"), 0644))

	time.Sleep(50 * time.Millisecond)

	// Create newer session
	newer := filepath.Join(sessionDir, "new-session.jsonl")
	require.NoError(t, os.WriteFile(newer, []byte(`{"type":"user","message":{"role":"user","content":[{"type":"text","text":"new"}]}}`+"\n"), 0644))

	// Resume without ID should get latest
	entries, resolvedID, err := Resume(projectPath, "")
	require.NoError(t, err)
	assert.Equal(t, "new-session", resolvedID)
	assert.Len(t, entries, 1)
}

func TestHistory_AddAndPrevious(t *testing.T) {
	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, "history.jsonl")

	h := NewHistoryWithPath(histPath)

	h.Add("first")
	h.Add("second")
	h.Add("third")

	// Previous should return in reverse order
	val, ok := h.Previous()
	assert.True(t, ok)
	assert.Equal(t, "third", val)

	val, ok = h.Previous()
	assert.True(t, ok)
	assert.Equal(t, "second", val)

	val, ok = h.Previous()
	assert.True(t, ok)
	assert.Equal(t, "first", val)

	// No more history
	_, ok = h.Previous()
	assert.False(t, ok)
}

func TestHistory_Next(t *testing.T) {
	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, "history.jsonl")

	h := NewHistoryWithPath(histPath)
	h.Add("first")
	h.Add("second")
	h.Add("third")

	// Navigate up
	h.Previous() // third
	h.Previous() // second
	h.Previous() // first

	// Navigate back down
	val, ok := h.Next()
	assert.True(t, ok)
	assert.Equal(t, "second", val)

	val, ok = h.Next()
	assert.True(t, ok)
	assert.Equal(t, "third", val)

	// Past end should return empty
	val, ok = h.Next()
	assert.True(t, ok)
	assert.Equal(t, "", val)
}

func TestHistory_Dedup(t *testing.T) {
	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, "history.jsonl")

	h := NewHistoryWithPath(histPath)
	h.Add("hello")
	h.Add("hello")
	h.Add("hello")

	assert.Equal(t, 1, h.Len())
}

func TestHistory_EmptyString(t *testing.T) {
	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, "history.jsonl")

	h := NewHistoryWithPath(histPath)
	h.Add("")
	h.Add("  ")

	assert.Equal(t, 0, h.Len())
}

func TestHistory_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	histPath := filepath.Join(tmpDir, "history.jsonl")

	// Create first history and add items
	h1 := NewHistoryWithPath(histPath)
	h1.Add("command1")
	h1.Add("command2")
	h1.Add("command3")

	// Create new history from same path -- should load persisted entries
	h2 := NewHistoryWithPath(histPath)
	assert.Equal(t, 3, h2.Len())

	// Verify the entries are correct by navigating
	val, ok := h2.Previous()
	assert.True(t, ok)
	assert.Equal(t, "command3", val)

	val, ok = h2.Previous()
	assert.True(t, ok)
	assert.Equal(t, "command2", val)
}
