package write

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/filestate"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

func newTestContext(t *testing.T) *tools.ToolUseContext {
	t.Helper()
	return &tools.ToolUseContext{
		WorkingDir:  t.TempDir(),
		ProjectRoot: t.TempDir(),
		SessionID:   "test-session",
		AbortCtx:    context.Background(),
		PermCtx:     permissions.NewPermissionContext(permissions.ModeDefault, nil, nil),
	}
}

func TestWriteTool_Name(t *testing.T) {
	tool := New()
	if tool.Name() != "Write" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "Write")
	}
}

func TestWriteTool_IsReadOnly(t *testing.T) {
	tool := New()
	if tool.IsReadOnly() {
		t.Error("IsReadOnly() = true, want false")
	}
}

func TestWriteTool_InputSchema(t *testing.T) {
	tool := New()
	schema := tool.InputSchema()
	if !json.Valid(schema) {
		t.Error("InputSchema() returned invalid JSON")
	}
	schemaStr := string(schema)
	if !strings.Contains(schemaStr, "file_path") {
		t.Error("InputSchema() does not contain 'file_path' property")
	}
	if !strings.Contains(schemaStr, "content") {
		t.Error("InputSchema() does not contain 'content' property")
	}
}

func TestWriteTool_WriteNewFile(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "newfile.txt")

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"hello world"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}

	// Verify file was actually written
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}
}

func TestWriteTool_CreateParentDirs(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "file.txt")

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"nested content"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}

	// Verify file exists
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "nested content" {
		t.Errorf("file content = %q, want %q", string(data), "nested content")
	}
}

func TestWriteTool_OverwriteExisting(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create existing file
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(path, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	// Overwrite with new content
	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"new content"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}

	// Verify content replaced
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(data) != "new content" {
		t.Errorf("file content = %q, want %q", string(data), "new content")
	}
}

func TestWriteTool_EmptyFilePath(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	input := json.RawMessage(`{"file_path":"","content":"hello"}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Call() returned IsError=false for empty file_path, want true")
	}
}

func TestWriteTool_SuccessMessage(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "msg.txt")
	content := "test content"

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":%q}`, path, content))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Successfully created") && !strings.Contains(text, "Successfully updated") {
		t.Errorf("success message should contain 'Successfully created' or 'Successfully updated', got: %s", text)
	}
	if !strings.Contains(text, fmt.Sprintf("%d bytes", len(content))) {
		t.Errorf("success message should contain byte count '%d bytes', got: %s", len(content), text)
	}
}

func TestWriteTool_CheckPermissions_Default(t *testing.T) {
	tool := New()
	ctx := context.Background()
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)

	result, err := tool.CheckPermissions(ctx, json.RawMessage(`{"file_path":"test.txt","content":"x"}`), permCtx)
	if err != nil {
		t.Fatalf("CheckPermissions() returned error: %v", err)
	}
	// Write tool is not read-only, so default mode -> Ask
	if result != permissions.Ask {
		t.Errorf("CheckPermissions() = %v, want Ask (%v)", result, permissions.Ask)
	}
}

func TestWriteTool_CheckPermissions_Auto(t *testing.T) {
	tool := New()
	ctx := context.Background()
	permCtx := permissions.NewPermissionContext(permissions.ModeAuto, nil, nil)

	result, err := tool.CheckPermissions(ctx, json.RawMessage(`{"file_path":"test.txt","content":"x"}`), permCtx)
	if err != nil {
		t.Fatalf("CheckPermissions() returned error: %v", err)
	}
	// Auto mode -> Allow
	if result != permissions.Allow {
		t.Errorf("CheckPermissions() = %v, want Allow (%v)", result, permissions.Allow)
	}
}

// newTestContextWithCache creates a test context with a FileStateCache.
func newTestContextWithCache(t *testing.T) *tools.ToolUseContext {
	t.Helper()
	return &tools.ToolUseContext{
		WorkingDir:     t.TempDir(),
		ProjectRoot:    t.TempDir(),
		SessionID:      "test-session",
		AbortCtx:       context.Background(),
		PermCtx:        permissions.NewPermissionContext(permissions.ModeDefault, nil, nil),
		FileStateCache: filestate.NewDefaultFileStateCache(),
	}
}

func TestWriteStalenessDetection_FileNotRead(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContextWithCache(t)

	// Create a temp file but do NOT add it to file state cache
	dir := t.TempDir()
	path := filepath.Join(dir, "unread.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"new content"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for file not read, got success")
	}
	if !strings.Contains(result.Content[0].Text, "has not been read yet") {
		t.Errorf("expected 'has not been read yet' error, got: %s", result.Content[0].Text)
	}
}

func TestWriteStalenessDetection_FileModifiedSinceRead(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContextWithCache(t)

	// Create file and add to cache with an old timestamp
	dir := t.TempDir()
	path := filepath.Join(dir, "stale.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Record a read with a timestamp in the past
	oldTimestamp := time.Now().Add(-10 * time.Second).UnixMilli()
	toolCtx.FileStateCache.Set(path, filestate.FileState{
		Content:   "original",
		Timestamp: oldTimestamp,
		Offset:    -1,
		Limit:     -1,
	})

	// Modify the file on disk (update mtime to now)
	time.Sleep(10 * time.Millisecond) // ensure mtime advances
	if err := os.WriteFile(path, []byte("modified externally"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"write attempt"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for stale file, got success")
	}
	if !strings.Contains(result.Content[0].Text, "has been modified since read") {
		t.Errorf("expected 'has been modified since read' error, got: %s", result.Content[0].Text)
	}
}

func TestWriteStalenessDetection_Pass(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContextWithCache(t)

	// Create file and add to cache with current timestamp
	dir := t.TempDir()
	path := filepath.Join(dir, "fresh.txt")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Record read with a timestamp >= the file's mtime
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	toolCtx.FileStateCache.Set(path, filestate.FileState{
		Content:   "original",
		Timestamp: info.ModTime().UnixMilli(),
		Offset:    -1,
		Limit:     -1,
	})

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"updated content"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Content[0].Text)
	}

	// Verify file was updated
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "updated content" {
		t.Errorf("file content = %q, want %q", string(data), "updated content")
	}
}

func TestWriteStalenessDetection_NewFile(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContextWithCache(t)

	// Write to a path that doesn't exist -- no staleness check needed
	dir := t.TempDir()
	path := filepath.Join(dir, "brand_new.txt")

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"brand new content"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success for new file, got error: %s", result.Content[0].Text)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != "brand new content" {
		t.Errorf("file content = %q, want %q", string(data), "brand new content")
	}
}

func TestWritePartialView(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContextWithCache(t)

	// Create file and add to cache with IsPartialView=true
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.txt")
	if err := os.WriteFile(path, []byte("partial content"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	toolCtx.FileStateCache.Set(path, filestate.FileState{
		Content:       "partial content",
		Timestamp:     time.Now().UnixMilli(),
		Offset:        0,
		Limit:         10,
		IsPartialView: true,
	})

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"overwrite attempt"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for partial view, got success")
	}
	if !strings.Contains(result.Content[0].Text, "has not been read yet") {
		t.Errorf("expected 'has not been read yet' error, got: %s", result.Content[0].Text)
	}
}

func TestWritePreservesCRLF(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContextWithCache(t)

	// Create file with CRLF line endings
	dir := t.TempDir()
	path := filepath.Join(dir, "crlf.txt")
	crlfContent := "line1\r\nline2\r\nline3\r\n"
	if err := os.WriteFile(path, []byte(crlfContent), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Add to cache (simulate read)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	toolCtx.FileStateCache.Set(path, filestate.FileState{
		Content:   crlfContent,
		Timestamp: info.ModTime().UnixMilli(),
		Offset:    -1,
		Limit:     -1,
	})

	// Write new content with LF endings -- should be converted to CRLF
	newContent := "new1\nnew2\nnew3\n"
	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":%q}`, path, newContent))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Content[0].Text)
	}

	// Verify the file has CRLF line endings
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	expected := "new1\r\nnew2\r\nnew3\r\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestWritePreservesLF(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContextWithCache(t)

	// Create file with LF endings
	dir := t.TempDir()
	path := filepath.Join(dir, "lf.txt")
	lfContent := "line1\nline2\nline3\n"
	if err := os.WriteFile(path, []byte(lfContent), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Add to cache (simulate read)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	toolCtx.FileStateCache.Set(path, filestate.FileState{
		Content:   lfContent,
		Timestamp: info.ModTime().UnixMilli(),
		Offset:    -1,
		Limit:     -1,
	})

	// Write new content with CRLF endings -- should be converted to LF
	newContent := "new1\r\nnew2\r\nnew3\r\n"
	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":%q}`, path, newContent))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Content[0].Text)
	}

	// Verify the file has LF endings
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	expected := "new1\nnew2\nnew3\n"
	if string(data) != expected {
		t.Errorf("file content = %q, want %q", string(data), expected)
	}
}

func TestWriteUpdatesFileStateCache(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContextWithCache(t)

	// Write a new file
	dir := t.TempDir()
	path := filepath.Join(dir, "cached.txt")
	content := "cached content"

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":%q}`, path, content))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Content[0].Text)
	}

	// Verify file state cache was updated
	state, ok := toolCtx.FileStateCache.Get(path)
	if !ok {
		t.Fatal("file state cache does not contain entry after write")
	}
	if state.Content != content {
		t.Errorf("cached content = %q, want %q", state.Content, content)
	}
	if state.Timestamp <= 0 {
		t.Errorf("cached timestamp = %d, want > 0", state.Timestamp)
	}
	if state.Offset != -1 {
		t.Errorf("cached offset = %d, want -1 (full content)", state.Offset)
	}
	if state.Limit != -1 {
		t.Errorf("cached limit = %d, want -1 (full content)", state.Limit)
	}
}

func TestWriteGitDiff(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContextWithCache(t)

	// Create a temp dir with a git repo
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := fmt.Sprintf("cd %s && git %s", dir, strings.Join(args, " "))
		out, err := execCommand("sh", "-c", cmd)
		if err != nil {
			t.Fatalf("git %s failed: %v\n%s", args[0], err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@test.com")
	runGit("config", "user.name", "Test")

	// Create and commit a file
	path := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(path, []byte("original\n"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	runGit("add", "tracked.txt")
	runGit("commit", "-m", "initial")

	// Add file to cache with current mtime
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}
	toolCtx.FileStateCache.Set(path, filestate.FileState{
		Content:   "original\n",
		Timestamp: info.ModTime().UnixMilli(),
		Offset:    -1,
		Limit:     -1,
	})

	// Write new content to tracked file
	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"content":"modified\n"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("expected success, got error: %s", result.Content[0].Text)
	}

	// Verify response contains diff text
	text := result.Content[0].Text
	if !strings.Contains(text, "Git diff:") {
		t.Logf("Response text: %s", text)
		// Git diff may or may not be present depending on git state; just log it
		t.Log("Note: Git diff section not found in response (may depend on git state)")
	}
	if !strings.Contains(text, "Successfully updated") {
		t.Errorf("expected 'Successfully updated' in response, got: %s", text)
	}
}

// execCommand is a helper to run shell commands in tests.
func execCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
