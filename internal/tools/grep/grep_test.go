package grep

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

func TestGrepTool_Name(t *testing.T) {
	tool := New()
	assert.Equal(t, "Grep", tool.Name())
}

func TestGrepTool_IsReadOnly(t *testing.T) {
	tool := New()
	assert.True(t, tool.IsReadOnly())
}

func TestGrepTool_BasicSearch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\ngoodbye world\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("nothing here\n"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "hello",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError, "result should not be error: %v", result.Content)
	assert.Contains(t, result.Content[0].Text, "hello world")
}

func TestGrepTool_RegexPattern(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("func NewTool() *Tool {\n\treturn &Tool{}\n}\n\nfunc main() {}\n"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "func.*New",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError, "result should not be error: %v", result.Content)
	assert.Contains(t, result.Content[0].Text, "func NewTool")
}

func TestGrepTool_IncludeFilter(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.go"), []byte("package main\nfunc hello() {}\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello from readme\n"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "hello",
		"path":    dir,
		"include": "*.go",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError, "result should not be error: %v", result.Content)
	output := result.Content[0].Text
	assert.Contains(t, output, "app.go")
	assert.NotContains(t, output, "readme.txt")
}

func TestGrepTool_ContextLines(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nTARGET\nline4\nline5\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte(content), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "TARGET",
		"path":    dir,
		"context": 2,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError, "result should not be error: %v", result.Content)
	output := result.Content[0].Text
	// With 2 context lines, we should see line1, line2, TARGET, line4, line5
	assert.Contains(t, output, "line1")
	assert.Contains(t, output, "TARGET")
	assert.Contains(t, output, "line5")
}

func TestGrepTool_NoMatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("some content\n"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "nonexistent_pattern_xyz",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	// No match should not be an error, just empty/no results message
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// Should indicate no results found
	assert.Contains(t, output, "No matches found")
}

func TestGrepTool_InvalidRegex(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content\n"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "[invalid",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGrepTool_CheckPermissions(t *testing.T) {
	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "test",
	})

	permCtx := &permissions.PermissionContext{
		Mode:            permissions.ModeDefault,
		AllowedTools:    make(map[string]bool),
		DisallowedTools: make(map[string]bool),
		AlwaysApproved:  make(map[string]bool),
	}

	result, err := tool.CheckPermissions(context.Background(), input, permCtx)
	require.NoError(t, err)
	// Read-only tools get Allow in ModeDefault
	assert.Equal(t, permissions.Allow, result)
}

func TestGrepTool_EmptyPattern(t *testing.T) {
	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: t.TempDir()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
