package bash

import (
	"context"
	"encoding/json"
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

func TestBashTool_Name(t *testing.T) {
	tool := New()
	if tool.Name() != "Bash" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "Bash")
	}
}

func TestBashTool_IsReadOnly(t *testing.T) {
	tool := New()
	if tool.IsReadOnly() {
		t.Error("IsReadOnly() = true, want false")
	}
}

func TestBashTool_InputSchema(t *testing.T) {
	tool := New()
	schema := tool.InputSchema()
	if !json.Valid(schema) {
		t.Error("InputSchema() returned invalid JSON")
	}
	schemaStr := string(schema)
	if !strings.Contains(schemaStr, "command") {
		t.Error("InputSchema() does not contain 'command' property")
	}
	if !strings.Contains(schemaStr, "timeout") {
		t.Error("InputSchema() does not contain 'timeout' property")
	}
}

func TestBashTool_EchoHello(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	input := json.RawMessage(`{"command":"echo hello"}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true, want false")
	}
	if len(result.Content) == 0 {
		t.Fatal("Call() returned empty Content")
	}
	if !strings.Contains(result.Content[0].Text, "hello") {
		t.Errorf("Call() output = %q, want to contain 'hello'", result.Content[0].Text)
	}
}

func TestBashTool_ExitCode(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	input := json.RawMessage(`{"command":"exit 42"}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Call() returned IsError=false for exit 42, want true")
	}
	if len(result.Content) == 0 {
		t.Fatal("Call() returned empty Content")
	}
	if !strings.Contains(result.Content[0].Text, "42") {
		t.Errorf("Call() output = %q, want to contain '42'", result.Content[0].Text)
	}
}

func TestBashTool_Stderr(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	input := json.RawMessage(`{"command":"echo err >&2"}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("Call() returned empty Content")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "STDERR:") {
		t.Errorf("Call() output = %q, want to contain 'STDERR:'", text)
	}
	if !strings.Contains(text, "err") {
		t.Errorf("Call() output = %q, want to contain 'err'", text)
	}
}

func TestBashTool_StdoutAndStderr(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	input := json.RawMessage(`{"command":"echo out; echo err >&2"}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("Call() returned empty Content")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "out") {
		t.Errorf("Call() output = %q, want to contain 'out'", text)
	}
	if !strings.Contains(text, "err") {
		t.Errorf("Call() output = %q, want to contain 'err'", text)
	}
}

func TestBashTool_Timeout(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	// 100ms timeout with a sleep 10 command
	input := json.RawMessage(`{"command":"sleep 10","timeout":100}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Call() returned IsError=false for timeout, want true")
	}
	if len(result.Content) == 0 {
		t.Fatal("Call() returned empty Content")
	}
	text := strings.ToLower(result.Content[0].Text)
	if !strings.Contains(text, "timed out") && !strings.Contains(text, "timeout") {
		t.Errorf("Call() output = %q, want to contain timeout message", result.Content[0].Text)
	}
}

func TestBashTool_EmptyCommand(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	input := json.RawMessage(`{"command":""}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Call() returned IsError=false for empty command, want true")
	}
	text := strings.ToLower(result.Content[0].Text)
	if !strings.Contains(text, "required") && !strings.Contains(text, "empty") && !strings.Contains(text, "missing") {
		t.Errorf("Call() output = %q, want to contain error about required field", result.Content[0].Text)
	}
}

func TestBashTool_InvalidJSON(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := newTestContext(t)

	input := json.RawMessage(`not valid json`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if !result.IsError {
		t.Error("Call() returned IsError=false for invalid JSON, want true")
	}
}

func TestBashTool_WorkingDir(t *testing.T) {
	tool := New()
	ctx := context.Background()
	tmpDir := t.TempDir()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  tmpDir,
		ProjectRoot: tmpDir,
		SessionID:   "test-session",
		AbortCtx:    context.Background(),
		PermCtx:     permissions.NewPermissionContext(permissions.ModeDefault, nil, nil),
	}

	input := json.RawMessage(`{"command":"pwd"}`)
	result, err := tool.Call(ctx, input, toolCtx)
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.IsError {
		t.Errorf("Call() returned IsError=true, want false")
	}
	if !strings.Contains(result.Content[0].Text, tmpDir) {
		t.Errorf("Call() output = %q, want to contain %q", result.Content[0].Text, tmpDir)
	}
}

func TestBashTool_CheckPermissions_SafeCommand(t *testing.T) {
	tool := New()
	ctx := context.Background()
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)

	// "echo hi" is classified as safe by the bash classifier, so it is
	// treated as read-only and auto-approved in default mode.
	result, err := tool.CheckPermissions(ctx, json.RawMessage(`{"command":"echo hi"}`), permCtx)
	if err != nil {
		t.Fatalf("CheckPermissions() returned error: %v", err)
	}
	if result != permissions.Allow {
		t.Errorf("CheckPermissions() = %v, want Allow (%v)", result, permissions.Allow)
	}
}

func TestBashTool_CheckPermissions_WriteCommand(t *testing.T) {
	tool := New()
	ctx := context.Background()
	permCtx := permissions.NewPermissionContext(permissions.ModeDefault, nil, nil)

	// "curl http://example.com" is not classified as safe/read-only,
	// so it requires user permission in default mode.
	result, err := tool.CheckPermissions(ctx, json.RawMessage(`{"command":"curl http://example.com"}`), permCtx)
	if err != nil {
		t.Fatalf("CheckPermissions() returned error: %v", err)
	}
	if result != permissions.Ask {
		t.Errorf("CheckPermissions() = %v, want Ask (%v)", result, permissions.Ask)
	}
}

func TestBashTool_CheckPermissions_Auto(t *testing.T) {
	tool := New()
	ctx := context.Background()
	permCtx := permissions.NewPermissionContext(permissions.ModeAuto, nil, nil)

	result, err := tool.CheckPermissions(ctx, json.RawMessage(`{"command":"echo hi"}`), permCtx)
	if err != nil {
		t.Fatalf("CheckPermissions() returned error: %v", err)
	}
	if result != permissions.Allow {
		t.Errorf("CheckPermissions() = %v, want Allow (%v)", result, permissions.Allow)
	}
}
