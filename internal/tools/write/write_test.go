package write

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
