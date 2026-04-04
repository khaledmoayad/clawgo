package edit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditTool_Name(t *testing.T) {
	tool := New()
	assert.Equal(t, "Edit", tool.Name())
}

func TestEditTool_IsReadOnly(t *testing.T) {
	tool := New()
	assert.False(t, tool.IsReadOnly())
}

func TestEditTool_SimpleReplace(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"file_path": filePath,
		"old_str":   "world",
		"new_str":   "Go",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "hello Go", string(content))
}

func TestEditTool_MultipleMatches(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("aaa\naaa\n"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"file_path": filePath,
		"old_str":   "aaa",
		"new_str":   "bbb",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "matches 2 locations")
}

func TestEditTool_NoMatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("hello"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"file_path": filePath,
		"old_str":   "nonexistent",
		"new_str":   "replacement",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "not found")
}

func TestEditTool_CreateNewFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "subdir", "new.txt")

	tool := New()
	input := mustJSON(t, map[string]any{
		"file_path": filePath,
		"old_str":   "",
		"new_str":   "brand new content",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "brand new content", string(content))
}

func TestEditTool_PreservesPermissions(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "restricted.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("secret data"), 0600))

	tool := New()
	input := mustJSON(t, map[string]any{
		"file_path": filePath,
		"old_str":   "secret",
		"new_str":   "public",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	info, err := os.Stat(filePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestEditTool_MultilineReplace(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "multi.txt")
	original := "line1\nline2\nline3\nline4\n"
	require.NoError(t, os.WriteFile(filePath, []byte(original), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"file_path": filePath,
		"old_str":   "line2\nline3",
		"new_str":   "replaced2\nreplaced3",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, "line1\nreplaced2\nreplaced3\nline4\n", string(content))
}

func TestEditTool_WhitespaceExact(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "ws.txt")
	// File contains "  hello" (2 spaces)
	require.NoError(t, os.WriteFile(filePath, []byte("  hello"), 0644))

	tool := New()
	// Try to match " hello" (1 space) -- should NOT match
	input := mustJSON(t, map[string]any{
		"file_path": filePath,
		"old_str":   " hello",
		"new_str":   "world",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	// " hello" is a substring of "  hello", so it matches once at position 1
	// Actually "  hello" contains " hello" starting at index 1, so it will match.
	// The test should verify exact string matching behavior:
	// " hello" is found in "  hello" once (as a substring), so this WILL replace.
	assert.False(t, result.IsError)

	content, err := os.ReadFile(filePath)
	require.NoError(t, err)
	// " hello" (1 space + hello) found once in "  hello" (2 spaces + hello)
	// Result: " " + "world" = " world"
	assert.Equal(t, " world", string(content))
}

func TestEditTool_EmptyFilePath(t *testing.T) {
	tool := New()
	input := mustJSON(t, map[string]any{
		"file_path": "",
		"old_str":   "something",
		"new_str":   "else",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: t.TempDir()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestEditTool_CheckPermissions(t *testing.T) {
	tool := New()
	input := mustJSON(t, map[string]any{
		"file_path": "/some/file.txt",
		"old_str":   "a",
		"new_str":   "b",
	})

	permCtx := &permissions.PermissionContext{
		Mode:            permissions.ModeDefault,
		AllowedTools:    make(map[string]bool),
		DisallowedTools: make(map[string]bool),
		AlwaysApproved:  make(map[string]bool),
	}

	result, err := tool.CheckPermissions(context.Background(), input, permCtx)
	require.NoError(t, err)
	assert.Equal(t, permissions.Ask, result)
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
