package grep

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hasRipgrep returns true if rg is available on the system.
func hasRipgrep() bool {
	_, err := exec.LookPath("rg")
	return err == nil
}

// setupTestDir creates a temporary directory with known test files.
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.go"), []byte(
		"package main\n\nfunc Hello() {\n\treturn\n}\n\nfunc main() {\n\treturn\n}\n",
	), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "world.py"), []byte(
		"def world():\n    return 42\n\ndef hello():\n    return 99\n",
	), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte(
		"# Hello World\n\nThis is a readme file.\n",
	), 0644))

	return dir
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

func callGrep(t *testing.T, dir string, params map[string]any) *tools.ToolResult {
	t.Helper()
	tool := New()
	input := mustJSON(t, params)
	result, err := tool.Call(context.Background(), input, &tools.ToolUseContext{WorkingDir: dir})
	require.NoError(t, err)
	return result
}

func TestGrepTool_Name(t *testing.T) {
	tool := New()
	assert.Equal(t, "Grep", tool.Name())
}

func TestGrepTool_IsReadOnly(t *testing.T) {
	tool := New()
	assert.True(t, tool.IsReadOnly())
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

func TestGrepTool_InvalidRegex(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content\n"), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern": "[invalid",
		"path":    dir,
	})
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
	assert.Equal(t, permissions.Allow, result)
}

// --- Output mode tests ---

func TestGrepOutputModeFilesWithMatches(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "Hello",
		"path":        dir,
		"output_mode": "files_with_matches",
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// Should contain file paths, not line content
	assert.Contains(t, output, "hello.go")
	assert.Contains(t, output, "readme.md")
	// Should NOT contain the actual line content in the path listing
	assert.Contains(t, output, "Found")
}

func TestGrepOutputModeFilesWithMatches_Default(t *testing.T) {
	// When output_mode is not specified, files_with_matches is the default
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern": "Hello",
		"path":    dir,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	assert.Contains(t, output, "Found")
}

func TestGrepOutputModeContent(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "Hello",
		"path":        dir,
		"output_mode": "content",
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// Content mode should show matching lines with line numbers
	assert.Contains(t, output, "Hello")
	// Should have line numbers (default -n=true)
	// Lines should have format like "filename:linenum:content"
	assert.True(t, strings.Contains(output, ":") && (strings.Contains(output, "1:") || strings.Contains(output, "3:")),
		"content mode should include line numbers, got: %s", output)
}

func TestGrepOutputModeCount(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "return",
		"path":        dir,
		"output_mode": "count",
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// Count mode should show file:N format and summary
	assert.Contains(t, output, "Found")
	assert.Contains(t, output, "total")
	assert.Contains(t, output, "occurrences")
	assert.Contains(t, output, "files")
}

// --- Context flag tests ---

func TestGrepContextBefore(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\nTARGET\nline5\nline6\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte(content), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern":     "TARGET",
		"path":        dir,
		"output_mode": "content",
		"-B":          2,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// With -B=2, should see 2 lines before TARGET
	assert.Contains(t, output, "line2")
	assert.Contains(t, output, "line3")
	assert.Contains(t, output, "TARGET")
	// Should NOT have line5 (no after context)
	// (ripgrep won't include after lines; native fallback also shouldn't)
}

func TestGrepContextAfter(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nTARGET\nline4\nline5\nline6\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte(content), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern":     "TARGET",
		"path":        dir,
		"output_mode": "content",
		"-A":          2,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	assert.Contains(t, output, "TARGET")
	assert.Contains(t, output, "line4")
	assert.Contains(t, output, "line5")
}

func TestGrepContextBoth(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nTARGET\nline4\nline5\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte(content), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern":     "TARGET",
		"path":        dir,
		"output_mode": "content",
		"context":     1,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	assert.Contains(t, output, "line2")
	assert.Contains(t, output, "TARGET")
	assert.Contains(t, output, "line4")
}

func TestGrepContextAlias(t *testing.T) {
	// -C should work as alias for context
	dir := t.TempDir()
	content := "line1\nline2\nTARGET\nline4\nline5\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte(content), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern":     "TARGET",
		"path":        dir,
		"output_mode": "content",
		"-C":          1,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	assert.Contains(t, output, "line2")
	assert.Contains(t, output, "TARGET")
	assert.Contains(t, output, "line4")
}

// --- Case insensitive tests ---

func TestGrepCaseInsensitive(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "hello",
		"path":        dir,
		"output_mode": "content",
		"-i":          true,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// "hello" with -i should match "Hello" in hello.go and readme.md
	assert.Contains(t, output, "Hello")
}

func TestGrepCaseInsensitive_Off(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "hello",
		"path":        dir,
		"output_mode": "files_with_matches",
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// "hello" without -i should only match world.py (has "def hello():")
	// Should NOT match hello.go's "func Hello()" (capital H)
	if hasRipgrep() {
		assert.Contains(t, output, "world.py")
		assert.NotContains(t, output, "hello.go")
		assert.NotContains(t, output, "readme.md")
	} else {
		// Native fallback also respects case sensitivity
		assert.Contains(t, output, "world.py")
	}
}

// --- Line number tests ---

func TestGrepLineNumbers(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "Hello",
		"path":        dir,
		"output_mode": "content",
		"-n":          false,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// With -n=false, output should NOT have line number prefixes
	// The content should still contain "Hello" but without number: prefix
	assert.Contains(t, output, "Hello")
	// In rg mode, --no-line-number removes line numbers from output
}

// --- File type tests ---

func TestGrepFileType(t *testing.T) {
	if !hasRipgrep() {
		t.Skip("ripgrep not available, skipping type test")
	}
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "return",
		"path":        dir,
		"output_mode": "files_with_matches",
		"type":        "go",
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// With type=go, only .go files should be searched
	assert.Contains(t, output, "hello.go")
	assert.NotContains(t, output, "world.py")
}

// --- Glob tests ---

func TestGrepGlob(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "return",
		"path":        dir,
		"output_mode": "files_with_matches",
		"glob":        "*.py",
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// With glob=*.py, only .py files should be searched
	assert.Contains(t, output, "world.py")
	assert.NotContains(t, output, "hello.go")
}

// --- Head limit tests ---

func TestGrepHeadLimit(t *testing.T) {
	dir := t.TempDir()
	// Create files with many matches
	for i := 0; i < 10; i++ {
		fname := filepath.Join(dir, strings.Replace("match_NN.txt", "NN", strings.Repeat("x", i+1), 1))
		require.NoError(t, os.WriteFile(fname, []byte("FINDME here\n"), 0644))
	}

	result := callGrep(t, dir, map[string]any{
		"pattern":     "FINDME",
		"path":        dir,
		"output_mode": "files_with_matches",
		"head_limit":  2,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// Should only show 2 files
	assert.Contains(t, output, "Found 2 file")
	// Should indicate pagination/truncation
	assert.Contains(t, output, "limit: 2")
}

func TestGrepHeadLimitZeroUnlimited(t *testing.T) {
	dir := t.TempDir()
	// Create 5 files with matches
	for i := 0; i < 5; i++ {
		fname := filepath.Join(dir, "file_"+strings.Repeat("a", i+1)+".txt")
		require.NoError(t, os.WriteFile(fname, []byte("FINDME\n"), 0644))
	}

	result := callGrep(t, dir, map[string]any{
		"pattern":     "FINDME",
		"path":        dir,
		"output_mode": "files_with_matches",
		"head_limit":  0, // unlimited
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// All 5 files should be returned (no truncation)
	assert.Contains(t, output, "Found 5 file")
	assert.NotContains(t, output, "limit:")
}

// --- Offset tests ---

func TestGrepOffset(t *testing.T) {
	dir := t.TempDir()
	// Create files with content lines
	content := "match1\nmatch2\nmatch3\nmatch4\nmatch5\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.txt"), []byte(content), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern":     "match",
		"path":        dir,
		"output_mode": "content",
		"offset":      2,
		"head_limit":  0, // unlimited after offset
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// With offset=2, first 2 lines should be skipped
	assert.NotContains(t, output, "match1")
	assert.NotContains(t, output, "match2")
	assert.Contains(t, output, "match3")
	assert.Contains(t, output, "match4")
	assert.Contains(t, output, "match5")
}

func TestGrepOffsetWithLimit(t *testing.T) {
	dir := t.TempDir()
	content := "match1\nmatch2\nmatch3\nmatch4\nmatch5\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.txt"), []byte(content), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern":     "match",
		"path":        dir,
		"output_mode": "content",
		"offset":      1,
		"head_limit":  2,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// offset=1 skips first, head_limit=2 takes next 2
	// Should have match2 and match3 (or lines 2-3 in rg output)
	assert.Contains(t, output, "match2")
	assert.Contains(t, output, "match3")
	// Truncation should be noted since there are 2 more after limit
	assert.Contains(t, output, "pagination")
}

// --- Multiline tests ---

func TestGrepMultiline(t *testing.T) {
	if !hasRipgrep() {
		t.Skip("ripgrep not available, skipping multiline test")
	}

	dir := t.TempDir()
	content := "function hello() {\n  return 42\n}\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "multi.js"), []byte(content), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern":     "hello.*return",
		"path":        dir,
		"output_mode": "content",
		"multiline":   true,
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// Multiline should match across lines
	assert.Contains(t, output, "hello")
	assert.Contains(t, output, "return")
}

// --- Default head_limit test ---

func TestGrepDefaultHeadLimit(t *testing.T) {
	// Verify that when head_limit is not specified, default 250 is used
	// We test this by creating more than 250 matches and verifying truncation

	dir := t.TempDir()
	var lines []string
	for i := 0; i < 300; i++ {
		lines = append(lines, "FINDME line content")
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern":     "FINDME",
		"path":        dir,
		"output_mode": "content",
		// head_limit not specified -- should default to 250
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	// Should be truncated since 300 > 250
	assert.Contains(t, output, "pagination")
	assert.Contains(t, output, "limit: 250")
}

// --- No match tests ---

func TestGrepNoMatch_FilesWithMatches(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "xyznonexistent_12345",
		"path":        dir,
		"output_mode": "files_with_matches",
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	assert.Contains(t, output, "No files found")
}

func TestGrepNoMatch_Content(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "xyznonexistent_12345",
		"path":        dir,
		"output_mode": "content",
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	assert.Contains(t, output, "No matches found")
}

func TestGrepNoMatch_Count(t *testing.T) {
	dir := setupTestDir(t)

	result := callGrep(t, dir, map[string]any{
		"pattern":     "xyznonexistent_12345",
		"path":        dir,
		"output_mode": "count",
	})
	assert.False(t, result.IsError)
	output := result.Content[0].Text
	assert.Contains(t, output, "No matches found")
}

// --- applyHeadLimit unit tests ---

func TestApplyHeadLimit_Basic(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}

	// Normal limit
	result, truncated := applyHeadLimit(items, 3, 0)
	assert.Equal(t, []string{"a", "b", "c"}, result)
	assert.True(t, truncated)

	// No truncation needed
	result, truncated = applyHeadLimit(items, 10, 0)
	assert.Equal(t, items, result)
	assert.False(t, truncated)

	// Unlimited (0)
	result, truncated = applyHeadLimit(items, 0, 0)
	assert.Equal(t, items, result)
	assert.False(t, truncated)
}

func TestApplyHeadLimit_WithOffset(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}

	// Offset skips first 2
	result, truncated := applyHeadLimit(items, 0, 2)
	assert.Equal(t, []string{"c", "d", "e"}, result)
	assert.False(t, truncated)

	// Offset + limit
	result, truncated = applyHeadLimit(items, 2, 1)
	assert.Equal(t, []string{"b", "c"}, result)
	assert.True(t, truncated) // 2 more items remain after limit

	// Offset past end
	result, truncated = applyHeadLimit(items, 10, 10)
	assert.Nil(t, result)
	assert.False(t, truncated)
}

// --- Split glob patterns test ---

func TestSplitGlobPatterns(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"*.js", []string{"*.js"}},
		{"*.js *.ts", []string{"*.js", "*.ts"}},
		{"*.{ts,tsx}", []string{"*.{ts,tsx}"}},
		{"*.js,*.ts", []string{"*.js", "*.ts"}},
		{"*.{ts,tsx} *.go", []string{"*.{ts,tsx}", "*.go"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := splitGlobPatterns(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Integration: basic search still works ---

func TestGrepTool_BasicSearch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\ngoodbye world\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("nothing here\n"), 0644))

	result := callGrep(t, dir, map[string]any{
		"pattern":     "hello",
		"path":        dir,
		"output_mode": "content",
	})
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "hello world")
}
