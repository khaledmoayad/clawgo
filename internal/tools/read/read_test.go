package read

import (
	"context"
	"encoding/base64"
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

// --- New tests for image, PDF, notebook, encoding, blocked paths ---

func TestReadImageFile(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create a temp .png file with known bytes
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header magic bytes
	if err := os.WriteFile(path, pngData, 0644); err != nil {
		t.Fatalf("failed to write PNG file: %v", err)
	}

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true for image: %s", result.Content[0].Text)
	}

	// Verify we got image metadata
	if result.Metadata == nil {
		t.Fatal("result.Metadata is nil, expected image metadata")
	}

	b64, ok := result.Metadata["base64"].(string)
	if !ok || b64 == "" {
		t.Error("Metadata should contain non-empty 'base64' key")
	}

	mediaType, ok := result.Metadata["media_type"].(string)
	if !ok || mediaType != "image/png" {
		t.Errorf("Metadata media_type = %q, want %q", mediaType, "image/png")
	}

	fileSize, ok := result.Metadata["file_size"].(int)
	if !ok || fileSize != len(pngData) {
		t.Errorf("Metadata file_size = %v, want %d", result.Metadata["file_size"], len(pngData))
	}

	// Verify base64 decodes correctly
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	if len(decoded) != len(pngData) {
		t.Errorf("decoded length = %d, want %d", len(decoded), len(pngData))
	}

	// Verify text content describes the image
	text := result.Content[0].Text
	if !strings.Contains(text, "Image file") {
		t.Errorf("text content should describe image, got: %s", text)
	}
}

func TestReadImageFile_JPEG(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "photo.jpg")
	jpgData := []byte{0xFF, 0xD8, 0xFF, 0xE0} // JPEG magic bytes
	if err := os.WriteFile(path, jpgData, 0644); err != nil {
		t.Fatalf("failed to write JPEG file: %v", err)
	}

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true for JPEG: %s", result.Content[0].Text)
	}
	if result.Metadata["media_type"] != "image/jpeg" {
		t.Errorf("media_type = %v, want image/jpeg", result.Metadata["media_type"])
	}
}

func TestReadPDFFile(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create a temp .pdf file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pdf")
	pdfData := []byte("%PDF-1.4 fake pdf content for testing")
	if err := os.WriteFile(path, pdfData, 0644); err != nil {
		t.Fatalf("failed to write PDF file: %v", err)
	}

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true for PDF: %s", result.Content[0].Text)
	}

	// Verify PDF metadata
	if result.Metadata == nil {
		t.Fatal("result.Metadata is nil, expected PDF metadata")
	}

	b64, ok := result.Metadata["base64"].(string)
	if !ok || b64 == "" {
		t.Error("Metadata should contain non-empty 'base64' key")
	}

	mediaType, ok := result.Metadata["media_type"].(string)
	if !ok || mediaType != "application/pdf" {
		t.Errorf("Metadata media_type = %q, want %q", mediaType, "application/pdf")
	}

	// Verify text content
	text := result.Content[0].Text
	if !strings.Contains(text, "PDF file") {
		t.Errorf("text content should describe PDF, got: %s", text)
	}
}

func TestReadNotebookFile(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create a temp .ipynb with 2 cells (code + markdown)
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ipynb")
	notebookContent := `{
		"cells": [
			{
				"cell_type": "markdown",
				"source": ["# Hello World\n", "This is a test notebook."],
				"metadata": {}
			},
			{
				"cell_type": "code",
				"source": "print('hello')",
				"metadata": {},
				"execution_count": 1,
				"outputs": [
					{
						"output_type": "stream",
						"text": "hello\n"
					}
				]
			}
		],
		"metadata": {
			"language_info": {"name": "python"}
		},
		"nbformat": 4,
		"nbformat_minor": 5
	}`
	if err := os.WriteFile(path, []byte(notebookContent), 0644); err != nil {
		t.Fatalf("failed to write notebook file: %v", err)
	}

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true for notebook: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text

	// Should contain both cells
	if !strings.Contains(text, "Cell [1]") {
		t.Errorf("output should contain 'Cell [1]', got: %s", text)
	}
	if !strings.Contains(text, "Cell [2]") {
		t.Errorf("output should contain 'Cell [2]', got: %s", text)
	}

	// Should contain cell types
	if !strings.Contains(text, "markdown") {
		t.Errorf("output should contain 'markdown' cell type, got: %s", text)
	}
	if !strings.Contains(text, "code") {
		t.Errorf("output should contain 'code' cell type, got: %s", text)
	}

	// Should contain source content
	if !strings.Contains(text, "Hello World") {
		t.Errorf("output should contain markdown source, got: %s", text)
	}
	if !strings.Contains(text, "print('hello')") {
		t.Errorf("output should contain code source, got: %s", text)
	}

	// Should contain output
	if !strings.Contains(text, "hello") {
		t.Errorf("output should contain cell output 'hello', got: %s", text)
	}
}

func TestReadEncodingDetection(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create a file with UTF-8 BOM
	dir := t.TempDir()
	path := filepath.Join(dir, "bom.txt")
	bom := []byte{0xEF, 0xBB, 0xBF} // UTF-8 BOM
	content := append(bom, []byte("Hello UTF-8 BOM\n")...)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write BOM file: %v", err)
	}

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Hello UTF-8 BOM") {
		t.Errorf("output should contain file content without BOM, got: %s", text)
	}
	// BOM bytes should NOT appear in output
	if strings.Contains(text, "\xEF\xBB\xBF") {
		t.Error("output should not contain BOM bytes")
	}
}

func TestReadEncodingDetection_UTF16LE(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create a file with UTF-16 LE BOM
	dir := t.TempDir()
	path := filepath.Join(dir, "utf16le.txt")

	// UTF-16 LE BOM + "Hi\n" encoded as UTF-16 LE
	bom := []byte{0xFF, 0xFE}
	// "Hi\n" in UTF-16 LE: H=0x48,0x00  i=0x69,0x00  \n=0x0A,0x00
	utf16leContent := []byte{0x48, 0x00, 0x69, 0x00, 0x0A, 0x00}
	content := append(bom, utf16leContent...)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write UTF-16 LE file: %v", err)
	}

	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Hi") {
		t.Errorf("output should contain 'Hi' decoded from UTF-16 LE, got: %s", text)
	}
}

func TestReadBlockedDevicePaths(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	blockedPaths := []string{
		"/dev/zero",
		"/dev/random",
		"/dev/urandom",
		"/dev/stdin",
		"/dev/tty",
		"/dev/console",
		"/dev/stdout",
		"/dev/stderr",
		"/dev/fd/0",
		"/dev/fd/1",
		"/dev/fd/2",
	}

	for _, devPath := range blockedPaths {
		t.Run(devPath, func(t *testing.T) {
			input := json.RawMessage(fmt.Sprintf(`{"file_path":%q}`, devPath))
			result, err := tool.Call(ctx, input, toolCtx)
			if err != nil {
				t.Fatalf("Call() returned error for %s: %v", devPath, err)
			}
			if !result.IsError {
				t.Errorf("Call() returned IsError=false for blocked device path %s, want true", devPath)
			}
			text := result.Content[0].Text
			if !strings.Contains(text, "block") && !strings.Contains(text, "infinite") {
				t.Errorf("error for %s should mention blocking, got: %s", devPath, text)
			}
		})
	}
}

func TestReadBlockedDevicePaths_ProcFd(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// /proc/self/fd/0 should also be blocked
	input := json.RawMessage(`{"file_path":"/proc/self/fd/0"}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Call() returned IsError=false for /proc/self/fd/0, want true")
	}
}

func TestReadPagesParameter(t *testing.T) {
	// Test that the pages parameter is parsed and validated
	tests := []struct {
		name    string
		pages   string
		wantErr bool
	}{
		{"single page", "3", false},
		{"range", "1-5", false},
		{"max range", "1-20", false},
		{"too many pages", "1-21", true},
		{"invalid format", "abc", true},
		{"negative start", "-1-5", true},
		{"inverted range", "5-1", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := parsePDFPageRange(tt.pages)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parsePDFPageRange(%q) = (%d, %d, nil), want error", tt.pages, start, end)
				}
			} else {
				if err != nil {
					t.Errorf("parsePDFPageRange(%q) returned error: %v", tt.pages, err)
				}
			}
		})
	}
}

func TestReadPDFWithPagesParam(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// Create a temp PDF file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pdf")
	pdfData := []byte("%PDF-1.4 test content")
	if err := os.WriteFile(path, pdfData, 0644); err != nil {
		t.Fatalf("failed to write PDF file: %v", err)
	}

	// Read with valid pages parameter
	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"pages":"1-5"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true: %s", result.Content[0].Text)
	}

	// Verify pages is recorded in metadata
	if result.Metadata != nil {
		if p, ok := result.Metadata["pages"].(string); ok {
			if p != "1-5" {
				t.Errorf("metadata pages = %q, want %q", p, "1-5")
			}
		}
	}
}

func TestReadPDFWithInvalidPages(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "test.pdf")
	pdfData := []byte("%PDF-1.4 test content")
	if err := os.WriteFile(path, pdfData, 0644); err != nil {
		t.Fatalf("failed to write PDF file: %v", err)
	}

	// Read with invalid pages parameter
	input := json.RawMessage(fmt.Sprintf(`{"file_path":%q,"pages":"abc"}`, path))
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Call() returned IsError=false for invalid pages param, want true")
	}
}

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"test.png", true},
		{"test.PNG", true},
		{"test.jpg", true},
		{"test.jpeg", true},
		{"test.gif", true},
		{"test.webp", true},
		{"test.txt", false},
		{"test.pdf", false},
		{"test.go", false},
	}

	for _, tt := range tests {
		if got := isImageFile(tt.path); got != tt.want {
			t.Errorf("isImageFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsPDFFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"test.pdf", true},
		{"test.PDF", true},
		{"test.txt", false},
		{"test.png", false},
	}

	for _, tt := range tests {
		if got := isPDFFile(tt.path); got != tt.want {
			t.Errorf("isPDFFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsNotebookFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"test.ipynb", true},
		{"test.IPYNB", true},
		{"test.py", false},
		{"test.txt", false},
	}

	for _, tt := range tests {
		if got := isNotebookFile(tt.path); got != tt.want {
			t.Errorf("isNotebookFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
