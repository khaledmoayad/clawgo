package glob

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobTool_Name(t *testing.T) {
	tool := New()
	assert.Equal(t, "Glob", tool.Name())
}

func TestGlobTool_IsReadOnly(t *testing.T) {
	tool := New()
	assert.True(t, tool.IsReadOnly())
}

func TestGlobTool_StarGlob(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError, "result should not be error: %v", result.Content)

	output := result.Content[0].Text
	assert.Contains(t, output, "a.txt")
	assert.Contains(t, output, "c.txt")
	assert.NotContains(t, output, "b.go")
}

func TestGlobTool_DoubleStarGlob(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	deepDir := filepath.Join(dir, "sub", "deep")
	require.NoError(t, os.MkdirAll(deepDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(subDir, "a.go"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(deepDir, "b.go"), []byte("b"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.txt"), []byte("r"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "**/*.go",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError, "result should not be error: %v", result.Content)

	output := result.Content[0].Text
	assert.Contains(t, output, "a.go")
	assert.Contains(t, output, "b.go")
	assert.NotContains(t, output, "root.txt")
}

func TestGlobTool_NoMatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.xyz",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "No matches found")
}

func TestGlobTool_SortByModTime(t *testing.T) {
	dir := t.TempDir()

	// Create files with different modification times
	file1 := filepath.Join(dir, "old.txt")
	file2 := filepath.Join(dir, "new.txt")

	require.NoError(t, os.WriteFile(file1, []byte("old"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("new"), 0644))

	// Set old.txt to be older
	oldTime := time.Now().Add(-1 * time.Hour)
	require.NoError(t, os.Chtimes(file1, oldTime, oldTime))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError, "result should not be error: %v", result.Content)

	output := result.Content[0].Text
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 2)
	// Newest first
	assert.Contains(t, lines[0], "new.txt")
	assert.Contains(t, lines[1], "old.txt")
}

func TestGlobTool_PathParam(t *testing.T) {
	// Working directory has no files, but path param points to a different directory
	workDir := t.TempDir()
	searchDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(searchDir, "found.txt"), []byte("found"), 0644))

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.txt",
		"path":    searchDir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: workDir})
	require.NoError(t, err)
	assert.False(t, result.IsError, "result should not be error: %v", result.Content)
	assert.Contains(t, result.Content[0].Text, "found.txt")
}

func TestGlobTool_CheckPermissions(t *testing.T) {
	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.go",
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

func TestGlobTool_EmptyPattern(t *testing.T) {
	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "",
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: t.TempDir()})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGlobTool_MaxResultsCap(t *testing.T) {
	dir := t.TempDir()

	// Create 150 files, exceeding the 100-file cap
	for i := 0; i < 150; i++ {
		name := fmt.Sprintf("test-%03d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644))
	}

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	output := result.Content[0].Text
	// Count lines before truncation message (file entries only)
	lines := strings.Split(output, "\n")
	fileLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[Results truncated") {
			continue
		}
		fileLines++
	}
	assert.Equal(t, 100, fileLines, "should return exactly 100 files")
	assert.Contains(t, output, "[Results truncated")
	assert.Contains(t, output, "showing 100 of 150 total matches")
}

func TestGlobTool_NoTruncation(t *testing.T) {
	dir := t.TempDir()

	// Create 50 files, under the cap
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("file-%03d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644))
	}

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	output := result.Content[0].Text
	assert.NotContains(t, output, "[Results truncated")

	// Count file lines
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 50, len(lines), "should return all 50 files")
}

func TestGlobTool_ExactlyAtCap(t *testing.T) {
	dir := t.TempDir()

	// Create exactly 100 files
	for i := 0; i < 100; i++ {
		name := fmt.Sprintf("cap-%03d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644))
	}

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	output := result.Content[0].Text
	// No truncation at exactly the cap
	assert.NotContains(t, output, "[Results truncated")

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 100, len(lines), "should return all 100 files without truncation")
}

func TestGlobTool_TruncationMetadata(t *testing.T) {
	dir := t.TempDir()

	// Test with truncation (120 files)
	for i := 0; i < 120; i++ {
		name := fmt.Sprintf("meta-%03d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644))
	}

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	require.NotNil(t, result.Metadata)
	assert.Equal(t, true, result.Metadata["truncated"])
	assert.Equal(t, 100, result.Metadata["num_files"])
	assert.Equal(t, 120, result.Metadata["total_matches"])

	// Test without truncation (30 files in a separate dir)
	dir2 := t.TempDir()
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("small-%03d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir2, name), []byte("x"), 0644))
	}

	input2 := mustJSON(t, map[string]any{
		"pattern": "*.txt",
		"path":    dir2,
	})

	result2, err := tool.Call(context.Background(), input2, &tools.ToolUseContext{WorkingDir: dir2})
	require.NoError(t, err)
	require.NotNil(t, result2.Metadata)
	assert.Equal(t, false, result2.Metadata["truncated"])
	assert.Equal(t, 30, result2.Metadata["num_files"])
	assert.Equal(t, 30, result2.Metadata["total_matches"])
}

func TestGlobTool_TruncationShowsTotal(t *testing.T) {
	dir := t.TempDir()

	// Create 200 files for a clear total
	for i := 0; i < 200; i++ {
		name := fmt.Sprintf("total-%03d.txt", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644))
	}

	tool := New()
	input := mustJSON(t, map[string]any{
		"pattern": "*.txt",
		"path":    dir,
	})

	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)

	output := result.Content[0].Text
	assert.Contains(t, output, "showing 100 of 200 total matches")
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
