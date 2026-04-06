package swarm

import (
	"testing"
)

func TestTeammateSpawnConfigAgentID(t *testing.T) {
	cfg := TeammateSpawnConfig{
		Name:     "researcher",
		TeamName: "alpha",
	}
	got := cfg.AgentID()
	want := "researcher@alpha"
	if got != want {
		t.Errorf("AgentID() = %q, want %q", got, want)
	}
}

func TestBackendTypeConstants(t *testing.T) {
	if BackendInProcess != "in-process" {
		t.Errorf("BackendInProcess = %q, want %q", BackendInProcess, "in-process")
	}
	if BackendTmux != "tmux" {
		t.Errorf("BackendTmux = %q, want %q", BackendTmux, "tmux")
	}
	if BackendITerm2 != "iterm2" {
		t.Errorf("BackendITerm2 = %q, want %q", BackendITerm2, "iterm2")
	}
}

func TestSystemPromptModeConstants(t *testing.T) {
	if SystemPromptDefault != "default" {
		t.Errorf("SystemPromptDefault = %q, want %q", SystemPromptDefault, "default")
	}
	if SystemPromptReplace != "replace" {
		t.Errorf("SystemPromptReplace = %q, want %q", SystemPromptReplace, "replace")
	}
	if SystemPromptAppend != "append" {
		t.Errorf("SystemPromptAppend = %q, want %q", SystemPromptAppend, "append")
	}
}

func TestIsPaneBackend(t *testing.T) {
	tests := []struct {
		bt   BackendType
		want bool
	}{
		{BackendInProcess, false},
		{BackendTmux, true},
		{BackendITerm2, true},
		{BackendType("unknown"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.bt), func(t *testing.T) {
			got := IsPaneBackend(tt.bt)
			if got != tt.want {
				t.Errorf("IsPaneBackend(%q) = %v, want %v", tt.bt, got, tt.want)
			}
		})
	}
}

func TestTeammateSpawnResult(t *testing.T) {
	result := TeammateSpawnResult{
		Success: true,
		AgentID: "tester@alpha",
		TaskID:  "task-123",
	}
	if !result.Success {
		t.Error("expected Success=true")
	}
	if result.AgentID != "tester@alpha" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "tester@alpha")
	}
	if result.TaskID != "task-123" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-123")
	}
	if result.Error != "" {
		t.Errorf("Error = %q, want empty", result.Error)
	}
}

func TestTeammateSpawnResultFailure(t *testing.T) {
	result := TeammateSpawnResult{
		Success: false,
		AgentID: "tester@alpha",
		Error:   "backend not available",
	}
	if result.Success {
		t.Error("expected Success=false")
	}
	if result.Error != "backend not available" {
		t.Errorf("Error = %q, want %q", result.Error, "backend not available")
	}
}

func TestTeammateMessage(t *testing.T) {
	msg := TeammateMessage{
		Text:      "Fix the null pointer in validate.ts:42",
		From:      "team-lead@alpha",
		Color:     "blue",
		Timestamp: "2024-01-01T00:00:00Z",
		Summary:   "Fix null pointer in validate",
	}
	if msg.Text == "" {
		t.Error("expected non-empty Text")
	}
	if msg.From != "team-lead@alpha" {
		t.Errorf("From = %q, want %q", msg.From, "team-lead@alpha")
	}
}

func TestTeammateSpawnConfigDefaults(t *testing.T) {
	// Verify zero-value fields have expected semantics
	cfg := TeammateSpawnConfig{
		Name:     "worker",
		TeamName: "team",
		Prompt:   "do work",
		Cwd:      "/tmp",
	}

	// Empty SystemPromptMode should be treated as default by consumers
	if cfg.SystemPromptMode != "" {
		t.Errorf("zero-value SystemPromptMode = %q, want empty", cfg.SystemPromptMode)
	}

	// Empty Permissions = all tools
	if cfg.Permissions != nil {
		t.Errorf("zero-value Permissions = %v, want nil", cfg.Permissions)
	}

	// AllowPermissionPrompts defaults to false
	if cfg.AllowPermissionPrompts {
		t.Error("zero-value AllowPermissionPrompts should be false")
	}
}

// mockExecutor verifies the TeammateExecutor interface is implementable.
type mockExecutor struct {
	backendType BackendType
}

func (m *mockExecutor) Type() BackendType                                     { return m.backendType }
func (m *mockExecutor) IsAvailable() (bool, error)                            { return true, nil }
func (m *mockExecutor) Spawn(config TeammateSpawnConfig) (*TeammateSpawnResult, error) {
	return &TeammateSpawnResult{
		Success: true,
		AgentID: config.AgentID(),
		TaskID:  "mock-task",
	}, nil
}
func (m *mockExecutor) SendMessage(agentID string, msg TeammateMessage) error { return nil }
func (m *mockExecutor) Terminate(agentID string, reason string) (bool, error) { return true, nil }
func (m *mockExecutor) Kill(agentID string) (bool, error)                     { return true, nil }
func (m *mockExecutor) IsActive(agentID string) (bool, error)                 { return true, nil }

func TestTeammateExecutorInterface(t *testing.T) {
	var exec TeammateExecutor = &mockExecutor{backendType: BackendInProcess}

	if exec.Type() != BackendInProcess {
		t.Errorf("Type() = %q, want %q", exec.Type(), BackendInProcess)
	}

	avail, err := exec.IsAvailable()
	if err != nil || !avail {
		t.Errorf("IsAvailable() = (%v, %v), want (true, nil)", avail, err)
	}

	result, err := exec.Spawn(TeammateSpawnConfig{
		Name:     "tester",
		TeamName: "alpha",
		Prompt:   "test everything",
		Cwd:      "/tmp",
	})
	if err != nil {
		t.Fatalf("Spawn error: %v", err)
	}
	if result.AgentID != "tester@alpha" {
		t.Errorf("Spawn AgentID = %q, want %q", result.AgentID, "tester@alpha")
	}

	if err := exec.SendMessage("tester@alpha", TeammateMessage{Text: "hello"}); err != nil {
		t.Errorf("SendMessage error: %v", err)
	}

	ok, err := exec.Terminate("tester@alpha", "done")
	if err != nil || !ok {
		t.Errorf("Terminate = (%v, %v), want (true, nil)", ok, err)
	}

	ok, err = exec.Kill("tester@alpha")
	if err != nil || !ok {
		t.Errorf("Kill = (%v, %v), want (true, nil)", ok, err)
	}

	active, err := exec.IsActive("tester@alpha")
	if err != nil || !active {
		t.Errorf("IsActive = (%v, %v), want (true, nil)", active, err)
	}
}
