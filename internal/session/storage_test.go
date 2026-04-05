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

func TestNewUUID(t *testing.T) {
	uuid := NewUUID()
	assert.NotEmpty(t, uuid)
	assert.Len(t, uuid, 36) // UUID format: 8-4-4-4-12

	// Each call should produce a unique UUID
	uuid2 := NewUUID()
	assert.NotEqual(t, uuid, uuid2)

	// Verify version 4 bits (byte 6 high nibble = 4)
	// In the string format: xxxxxxxx-xxxx-4xxx-...
	assert.Equal(t, byte('4'), uuid[14], "UUID version should be 4")
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

	// Append 3 entries using legacy format
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

// --- New tests for UUID chain and transcript messages ---

func TestChainTracker_UUIDChain(t *testing.T) {
	ct := NewChainTracker()

	// First message: no parent
	uuid1, parent1 := ct.NextUUID()
	assert.NotEmpty(t, uuid1)
	assert.Nil(t, parent1, "first message should have nil parentUUID")

	// Second message: parent is first
	uuid2, parent2 := ct.NextUUID()
	assert.NotEmpty(t, uuid2)
	assert.NotEqual(t, uuid1, uuid2)
	require.NotNil(t, parent2)
	assert.Equal(t, uuid1, *parent2, "second message's parent should be first message's UUID")

	// Third message: parent is second
	uuid3, parent3 := ct.NextUUID()
	assert.NotEmpty(t, uuid3)
	require.NotNil(t, parent3)
	assert.Equal(t, uuid2, *parent3, "third message's parent should be second message's UUID")
}

func TestChainTracker_SetLastUUID(t *testing.T) {
	ct := NewChainTracker()

	ct.SetLastUUID("existing-uuid-123")

	uuid, parent := ct.NextUUID()
	assert.NotEmpty(t, uuid)
	require.NotNil(t, parent)
	assert.Equal(t, "existing-uuid-123", *parent)
}

func TestAppendTranscriptMessage(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "test-transcript.jsonl")

	ct := NewChainTracker()
	uuid1, parent1 := ct.NextUUID()

	msg := TranscriptMessage{
		SerializedMessage: SerializedMessage{
			Type:      "user",
			Role:      "user",
			Content:   json.RawMessage(`[{"type":"text","text":"hello"}]`),
			CWD:       "/test/project",
			UserType:  "external",
			SessionID: "test-session",
			Timestamp: "2026-04-05T12:00:00Z",
			Version:   "1.0.0",
		},
		UUID:       uuid1,
		ParentUUID: parent1,
	}

	err := AppendTranscriptMessage(sessionPath, msg)
	require.NoError(t, err)

	// Verify the file has proper JSON
	data, err := os.ReadFile(sessionPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"uuid"`)
	assert.Contains(t, string(data), `"parentUuid"`)
	assert.Contains(t, string(data), `"cwd"`)
	assert.Contains(t, string(data), `"userType"`)
}

func TestAppendMetadataEntry(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "test-metadata.jsonl")

	tag := TagMessage{
		Type:      "tag",
		SessionID: "test-session",
		Tag:       "important-test",
	}

	err := AppendMetadataEntry(sessionPath, tag)
	require.NoError(t, err)

	// Load and verify
	entries, err := LoadSession(sessionPath)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "tag", entries[0].Type)

	// Verify the raw JSON contains the tag
	assert.Contains(t, string(entries[0].Raw), `"tag":"important-test"`)
}

func TestUUIDChain_AppendAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "test-chain.jsonl")

	ct := NewChainTracker()

	// Append 3 transcript messages
	for i, text := range []string{"hello", "hi there", "bye"} {
		uuid, parent := ct.NextUUID()
		role := "user"
		if i == 1 {
			role = "assistant"
		}
		msg := TranscriptMessage{
			SerializedMessage: SerializedMessage{
				Type:      role,
				Role:      role,
				Content:   json.RawMessage(`[{"type":"text","text":"` + text + `"}]`),
				SessionID: "test-session",
				Timestamp: "2026-04-05T12:00:00Z",
				Version:   "1.0.0",
			},
			UUID:       uuid,
			ParentUUID: parent,
		}
		require.NoError(t, AppendTranscriptMessage(sessionPath, msg))
	}

	// Load transcript and verify chain
	messages, loadedCT, err := LoadTranscript(sessionPath)
	require.NoError(t, err)
	require.Len(t, messages, 3)

	// First message: nil parent
	assert.NotEmpty(t, messages[0].UUID)
	assert.Nil(t, messages[0].ParentUUID, "first message should have nil parent")

	// Second message: parent is first
	assert.NotEmpty(t, messages[1].UUID)
	require.NotNil(t, messages[1].ParentUUID)
	assert.Equal(t, messages[0].UUID, *messages[1].ParentUUID)

	// Third message: parent is second
	assert.NotEmpty(t, messages[2].UUID)
	require.NotNil(t, messages[2].ParentUUID)
	assert.Equal(t, messages[1].UUID, *messages[2].ParentUUID)

	// ChainTracker should be positioned at the last message
	assert.Equal(t, messages[2].UUID, loadedCT.LastUUID())
}

func TestLoadTranscript_BackwardCompat(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "test-legacy.jsonl")

	// Write old-format entries (no UUID)
	oldEntries := []Entry{
		{Type: "user", Message: json.RawMessage(`{"role":"user","content":[{"type":"text","text":"hello"}]}`)},
		{Type: "assistant", Message: json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"hi"}]}`)},
		{Type: "user", Message: json.RawMessage(`{"role":"user","content":[{"type":"text","text":"bye"}]}`)},
	}
	for _, e := range oldEntries {
		require.NoError(t, AppendEntry(sessionPath, e))
	}

	// Load as transcript -- should assign UUIDs automatically
	messages, ct, err := LoadTranscript(sessionPath)
	require.NoError(t, err)
	require.Len(t, messages, 3)

	// All messages should have UUIDs assigned
	for i, msg := range messages {
		assert.NotEmpty(t, msg.UUID, "message %d should have UUID assigned", i)
	}

	// Chain should be intact
	assert.Nil(t, messages[0].ParentUUID, "first legacy message should have nil parent")
	require.NotNil(t, messages[1].ParentUUID)
	assert.Equal(t, messages[0].UUID, *messages[1].ParentUUID)
	require.NotNil(t, messages[2].ParentUUID)
	assert.Equal(t, messages[1].UUID, *messages[2].ParentUUID)

	// ChainTracker should track last UUID
	assert.Equal(t, messages[2].UUID, ct.LastUUID())
}

func TestHashPath(t *testing.T) {
	// Verify known path produces consistent hash
	hash1 := hashPath("/home/user/project")
	assert.Len(t, hash1, 16, "hash should be 16 hex chars (8 bytes)")

	// Same path produces same hash
	hash2 := hashPath("/home/user/project")
	assert.Equal(t, hash1, hash2)

	// Different path produces different hash
	hash3 := hashPath("/home/user/other-project")
	assert.NotEqual(t, hash1, hash3)
}

func TestIsTranscriptMessage(t *testing.T) {
	assert.True(t, IsTranscriptMessage("user"))
	assert.True(t, IsTranscriptMessage("assistant"))
	assert.True(t, IsTranscriptMessage("attachment"))
	assert.True(t, IsTranscriptMessage("system"))
	assert.False(t, IsTranscriptMessage("progress"))
	assert.False(t, IsTranscriptMessage("summary"))
	assert.False(t, IsTranscriptMessage("tag"))
	assert.False(t, IsTranscriptMessage("custom-title"))
}

func TestIsChainParticipant(t *testing.T) {
	assert.True(t, IsChainParticipant("user"))
	assert.True(t, IsChainParticipant("assistant"))
	assert.True(t, IsChainParticipant("summary"))
	assert.True(t, IsChainParticipant("tag"))
	assert.False(t, IsChainParticipant("progress"), "progress should not be a chain participant")
}

func TestParseEntry(t *testing.T) {
	// New format with type field
	data := `{"type":"user","uuid":"abc-123","parentUuid":null,"role":"user","content":[{"type":"text","text":"hello"}]}`
	entry, err := ParseEntry([]byte(data))
	require.NoError(t, err)
	assert.Equal(t, "user", entry.Type)
	assert.NotNil(t, entry.Raw)

	// Parse as TranscriptMessage
	tm := entry.AsTranscriptMessage()
	require.NotNil(t, tm)
	assert.Equal(t, "abc-123", tm.UUID)
	assert.Equal(t, "user", tm.Role)

	// Legacy format with role but no type
	legacy := `{"role":"assistant","content":[{"type":"text","text":"hi"}]}`
	entry2, err := ParseEntry([]byte(legacy))
	require.NoError(t, err)
	assert.Equal(t, "assistant", entry2.Type, "should fall back to role field")
}

func TestMetadataEntries(t *testing.T) {
	tmpDir := t.TempDir()
	sessionPath := filepath.Join(tmpDir, "test-mixed.jsonl")

	ct := NewChainTracker()

	// Append a transcript message
	uuid, parent := ct.NextUUID()
	tm := TranscriptMessage{
		SerializedMessage: SerializedMessage{
			Type:      "user",
			Role:      "user",
			Content:   json.RawMessage(`[{"type":"text","text":"hello"}]`),
			SessionID: "s1",
			Timestamp: "2026-04-05T12:00:00Z",
			Version:   "1.0.0",
		},
		UUID:       uuid,
		ParentUUID: parent,
	}
	require.NoError(t, AppendTranscriptMessage(sessionPath, tm))

	// Append a metadata entry
	summary := SummaryMessage{
		Type:     "summary",
		LeafUUID: uuid,
		Summary:  "User asked hello",
	}
	require.NoError(t, AppendMetadataEntry(sessionPath, summary))

	// Load and verify
	entries, err := LoadSession(sessionPath)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, "user", entries[0].Type)
	assert.Equal(t, "summary", entries[1].Type)
}

func TestAllMetadataEntryTypes(t *testing.T) {
	// Verify all metadata entry types can be marshaled/unmarshaled
	entries := []interface{}{
		SummaryMessage{Type: "summary", LeafUUID: "uuid-1", Summary: "test summary"},
		CustomTitleMessage{Type: "custom-title", SessionID: "s1", CustomTitle: "My Session"},
		AiTitleMessage{Type: "ai-title", SessionID: "s1", AiTitle: "Debugging Auth"},
		LastPromptMessage{Type: "last-prompt", SessionID: "s1", LastPrompt: "fix the bug"},
		TaskSummaryMessage{Type: "task-summary", SessionID: "s1", Summary: "Working on auth", Timestamp: "2026-04-05T12:00:00Z"},
		TagMessage{Type: "tag", SessionID: "s1", Tag: "important"},
		AgentNameMessage{Type: "agent-name", SessionID: "s1", AgentName: "CodeBot"},
		AgentColorMessage{Type: "agent-color", SessionID: "s1", AgentColor: "#FF0000"},
		AgentSettingMessage{Type: "agent-setting", SessionID: "s1", AgentSetting: "default-agent"},
		PRLinkMessage{Type: "pr-link", SessionID: "s1", PRNumber: 42, PRUrl: "https://github.com/org/repo/pull/42", PRRepository: "org/repo", Timestamp: "2026-04-05T12:00:00Z"},
		SpeculationAcceptMessage{Type: "speculation-accept", Timestamp: "2026-04-05T12:00:00Z", TimeSavedMs: 500},
		ModeEntry{Type: "mode", SessionID: "s1", Mode: "coordinator"},
		ContextCollapseCommitEntry{Type: "marble-origami-commit", SessionID: "s1", CollapseID: "1234567890123456", SummaryUUID: "sum-uuid", SummaryContent: "<collapsed>text</collapsed>", Summary: "Collapsed conversation", FirstArchivedUUID: "first-uuid", LastArchivedUUID: "last-uuid"},
	}

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		require.NoError(t, err, "should marshal %T", entry)

		// Should be parseable as an Entry
		parsed, err := ParseEntry(data)
		require.NoError(t, err, "should parse %T", entry)
		assert.NotEmpty(t, parsed.Type, "parsed %T should have a type", entry)
	}
}
