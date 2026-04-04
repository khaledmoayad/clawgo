package read

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

func writeTestFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return path
}

func TestReadTool_Name(t *testing.T) {
	tool := New()
	if tool.Name() != "Read" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "Read")
	}
}

func TestReadTool_IsReadOnly(t *testing.T) {
	tool := New()
	if !tool.IsReadOnly() {
		t.Error("IsReadOnly() = false, want true")
	}
}

func TestReadTool_InputSchema(t *testing.T) {
	tool := New()
	schema := tool.InputSchema()
	if !json.Valid(schema) {
		t.Error("InputSchema() returned invalid JSON")
	}
	schemaStr := string(schema)
	if !strings.Contains(schemaStr, "file_path") {
		t.Error("InputSchema() does not contain 'file_path' property")
	}
}

func TestReadTool_ReadFile(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	content := "line1\nline2\nline3\nline4\nline5\n"
	path := writeTestFile(t, content)

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	// Verify line numbers are present (cat -n format)
	if !strings.Contains(text, "1\t") {
		t.Errorf("output should contain line number 1, got: %s", text)
	}
	if !strings.Contains(text, "line1") {
		t.Errorf("output should contain 'line1', got: %s", text)
	}
	if !strings.Contains(text, "line5") {
		t.Errorf("output should contain 'line5', got: %s", text)
	}
}

func TestReadTool_OffsetLimit(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create file with 10 lines
	var lines []string
	for i := 1; i <= 10; i++ {
		lines = append(lines, fmt.Sprintf("line%d", i))
	}
	path := writeTestFile(t, strings.Join(lines, "\n")+"\n")

	// offset=3 (0-indexed), limit=2 -> should return lines at index 3,4 (line4, line5)
	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"offset":3,"limit":2}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "line4") {
		t.Errorf("output should contain 'line4' (offset=3), got: %s", text)
	}
	if !strings.Contains(text, "line5") {
		t.Errorf("output should contain 'line5' (offset=3, limit=2), got: %s", text)
	}
	if strings.Contains(text, "line3") {
		t.Errorf("output should NOT contain 'line3' (before offset), got: %s", text)
	}
	if strings.Contains(text, "line6") {
		t.Errorf("output should NOT contain 'line6' (after limit), got: %s", text)
	}
}

func TestReadTool_Nonexistent(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	input := json.RawMessage(`{"file_path":"/tmp/nonexistent_file_12345.txt"}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Call() returned IsError=false for nonexistent file, want true")
	}
}

func TestReadTool_BinaryFile(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create file with null bytes (binary content)
	dir := t.TempDir()
	path := filepath.Join(dir, "binary.bin")
	data := []byte("hello\x00world\x00binary")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write binary file: %v", err)
	}

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Call() returned IsError=false for binary file, want true")
	}
	text := strings.ToLower(result.Content[0].Text)
	if !strings.Contains(text, "binary") {
		t.Errorf("error message should mention 'binary', got: %s", result.Content[0].Text)
	}
}

func TestReadTool_EmptyFile(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	path := writeTestFile(t, "")
	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	// Empty file should not error, but result should indicate emptiness
	if result.IsError {
		t.Errorf("Call() returned IsError=true for empty file: %s", result.Content[0].Text)
	}
}

func TestReadTool_DefaultLimit(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create file with 3000 lines
	var lines []string
	for i := 1; i <= 3000; i++ {
		lines = append(lines, fmt.Sprintf("line%d", i))
	}
	path := writeTestFile(t, strings.Join(lines, "\n")+"\n")

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}
	text := result.Content[0].Text
	// Should contain line 2000 but not line 2001
	if !strings.Contains(text, "line2000") {
		t.Error("output should contain 'line2000' (within default limit)")
	}
	if strings.Contains(text, "line2001") {
		t.Error("output should NOT contain 'line2001' (exceeds default limit of 2000)")
	}
}

func TestReadTool_CheckPermissions_Default(t *testing.T) {
	tool := New()
	ctx := context.Background()
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)

	result, err := tool.CheckPermissions(ctx, json.RawMessage(`{"file_path":"test.txt"}`), permCtx)
	if err != nil {
		t.Fatalf("CheckPermissions() returned error: %v", err)
	}
	// Read-only tools are auto-approved in default mode
	if result != permissions.Allow {
		t.Errorf("CheckPermissions() = %v, want Allow (%v)", result, permissions.Allow)
	}
}
