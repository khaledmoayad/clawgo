package attribution

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracker_RecordChange(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("src/main.go", []byte("package main"), true)

	attrs := tracker.GetAttribution()
	require.Contains(t, attrs, "src/main.go")
	assert.Equal(t, "src/main.go", attrs["src/main.go"].Path)
	assert.True(t, attrs["src/main.go"].ModifiedByAI)
	assert.NotEmpty(t, attrs["src/main.go"].Hash)
	assert.False(t, attrs["src/main.go"].Timestamp.IsZero())
}

func TestTracker_RecordChange_HashConsistency(t *testing.T) {
	tracker1 := NewTracker("session-1")
	tracker2 := NewTracker("session-2")

	content := []byte("identical content")
	tracker1.RecordChange("file.go", content, true)
	tracker2.RecordChange("file.go", content, false)

	attrs1 := tracker1.GetAttribution()
	attrs2 := tracker2.GetAttribution()

	// Same content must produce same hash regardless of session or AI flag
	assert.Equal(t, attrs1["file.go"].Hash, attrs2["file.go"].Hash)
}

func TestTracker_GetAIModifiedFiles(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("ai-file1.go", []byte("ai1"), true)
	tracker.RecordChange("human-file.go", []byte("human"), false)
	tracker.RecordChange("ai-file2.go", []byte("ai2"), true)

	aiFiles := tracker.GetAIModifiedFiles()
	assert.Len(t, aiFiles, 2)
	assert.Contains(t, aiFiles, "ai-file1.go")
	assert.Contains(t, aiFiles, "ai-file2.go")
	assert.NotContains(t, aiFiles, "human-file.go")
}

func TestTracker_Snapshot_UseLastOnly(t *testing.T) {
	tracker := NewTracker("test-session")

	// Record multiple changes to the same file
	tracker.RecordChange("file.go", []byte("version1"), true)
	tracker.RecordChange("file.go", []byte("version2"), true)
	tracker.RecordChange("file.go", []byte("version3"), true)

	snap := tracker.Snapshot()

	// Parse the snapshot to verify only latest state exists
	var states map[string]*FileState
	err := json.Unmarshal(snap, &states)
	require.NoError(t, err)

	// Only one entry for the file (latest state, not delta history)
	require.Len(t, states, 1)
	require.Contains(t, states, "file.go")

	// Hash should match version3 content
	tracker2 := NewTracker("verify")
	tracker2.RecordChange("file.go", []byte("version3"), true)
	expected := tracker2.GetAttribution()["file.go"].Hash
	assert.Equal(t, expected, states["file.go"].Hash)
}

func TestFormatTrailer_WithAIChanges(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("main.go", []byte("package main"), true)

	trailer := FormatTrailer(tracker)
	assert.Contains(t, trailer, "Co-Authored-By")
	assert.Contains(t, trailer, "Claude")
	assert.Contains(t, trailer, "noreply@anthropic.com")
}

func TestFormatTrailer_NoAIChanges(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("main.go", []byte("package main"), false)

	trailer := FormatTrailer(tracker)
	assert.Empty(t, trailer)
}

func TestFormatGitNote(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("src/api.go", []byte("api code"), true)
	tracker.RecordChange("src/util.go", []byte("util code"), true)
	tracker.RecordChange("README.md", []byte("readme"), false)

	note := FormatGitNote(tracker)
	assert.Contains(t, note, "AI-assisted changes:")
	assert.Contains(t, note, "src/api.go")
	assert.Contains(t, note, "src/util.go")
	assert.NotContains(t, note, "README.md")
	// Should contain truncated hashes
	assert.Contains(t, note, "[")
	assert.Contains(t, note, "]")
}

func TestFormatGitNote_NoAIChanges(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("file.go", []byte("data"), false)

	note := FormatGitNote(tracker)
	assert.Empty(t, note)
}

func TestFormatCommitMessage_WithAIFiles(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("main.go", []byte("package main"), true)

	msg := FormatCommitMessage("feat: add feature", tracker)
	assert.Contains(t, msg, "feat: add feature")
	assert.Contains(t, msg, "Co-Authored-By: Claude <noreply@anthropic.com>")
	// Should have two newlines between message and trailer
	assert.Contains(t, msg, "\n\nCo-Authored-By")
}

func TestFormatCommitMessage_NoAIFiles(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("main.go", []byte("package main"), false)

	msg := FormatCommitMessage("feat: add feature", tracker)
	assert.Equal(t, "feat: add feature", msg)
	assert.NotContains(t, msg, "Co-Authored-By")
}

func TestFormatCommitMessage_NoDuplicate(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("main.go", []byte("package main"), true)

	original := "feat: add feature\n\nCo-Authored-By: Claude <noreply@anthropic.com>"
	msg := FormatCommitMessage(original, tracker)
	assert.Equal(t, original, msg, "should not duplicate trailer")
}

func TestFormatCommitMessage_EmptyTracker(t *testing.T) {
	tracker := NewTracker("test-session")

	msg := FormatCommitMessage("feat: add feature", tracker)
	assert.Equal(t, "feat: add feature", msg)
}

func TestTracker_GetAttribution_ReturnsCopy(t *testing.T) {
	tracker := NewTracker("test-session")
	tracker.RecordChange("file.go", []byte("data"), true)

	attrs := tracker.GetAttribution()
	// Mutating the returned map should not affect the tracker
	delete(attrs, "file.go")

	attrs2 := tracker.GetAttribution()
	assert.Contains(t, attrs2, "file.go", "tracker state should not be affected by external mutation")
}
