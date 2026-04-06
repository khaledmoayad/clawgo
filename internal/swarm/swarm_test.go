package swarm

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/tools/tasks"
)

func TestCreateTeamAndDelete(t *testing.T) {
	store := tasks.NewStore()
	m := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")
	defer m.Close()

	// Create a team
	team := m.CreateTeam("alpha")
	if team.Name != "alpha" {
		t.Errorf("expected team name 'alpha', got %q", team.Name)
	}

	// Creating same team returns existing
	team2 := m.CreateTeam("alpha")
	if team2 != team {
		t.Error("expected same team instance for duplicate name")
	}

	// Get team
	got, ok := m.GetTeam("alpha")
	if !ok || got.Name != "alpha" {
		t.Error("GetTeam failed to find 'alpha'")
	}

	// Delete team
	if err := m.DeleteTeam("alpha"); err != nil {
		t.Errorf("DeleteTeam error: %v", err)
	}

	// Should not exist after delete
	_, ok = m.GetTeam("alpha")
	if ok {
		t.Error("expected team to be deleted")
	}

	// Delete non-existent team should error
	if err := m.DeleteTeam("nonexistent"); err == nil {
		t.Error("expected error deleting non-existent team")
	}
}

func TestSpawnWorkerRegistersInStore(t *testing.T) {
	store := tasks.NewStore()
	// We pass nil client/registry -- the worker goroutine will fail on RunLoop
	// but we only need to test that the worker and task are created properly.
	m := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")
	defer m.Close()

	m.CreateTeam("beta")

	// Use a real context so context.WithCancel doesn't panic
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := m.SpawnWorker(ctx, "beta", "researcher", "test worker", "do something")
	if err != nil {
		t.Fatalf("SpawnWorker error: %v", err)
	}

	expectedID := "researcher@beta"
	if w.ID != expectedID {
		t.Errorf("expected worker ID %q, got %q", expectedID, w.ID)
	}
	if w.AgentName != "researcher" {
		t.Errorf("expected AgentName 'researcher', got %q", w.AgentName)
	}
	if w.TeamName != "beta" {
		t.Errorf("expected TeamName 'beta', got %q", w.TeamName)
	}
	if w.Status != WorkerRunning {
		t.Errorf("expected worker status 'running', got %q", w.Status)
	}
	if w.Description != "test worker" {
		t.Errorf("expected description 'test worker', got %q", w.Description)
	}

	// Verify worker is in the manager's map
	got, ok := m.GetWorker(w.ID)
	if !ok {
		t.Error("GetWorker failed to find spawned worker")
	}
	if got.TaskID == "" {
		t.Error("expected worker to have a TaskID from the task store")
	}

	// Verify task exists in store
	task, ok := store.Get(w.TaskID)
	if !ok {
		t.Error("expected task in store for worker")
	}
	if task.Type != "local_agent" {
		t.Errorf("expected task type 'local_agent', got %q", task.Type)
	}

	// Wait briefly for the goroutine to attempt RunLoop and fail (nil client)
	time.Sleep(100 * time.Millisecond)

	// Worker should have failed because client is nil
	m.mu.RLock()
	status := w.Status
	m.mu.RUnlock()
	// Status should be either "failed" (RunLoop error) or "running" (hasn't updated yet)
	if status != WorkerFailed && status != WorkerRunning {
		t.Errorf("expected status 'failed' or 'running', got %q", status)
	}
}

func TestSpawnWorkerTeamNotFound(t *testing.T) {
	store := tasks.NewStore()
	m := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")
	defer m.Close()

	ctx := context.Background()
	_, err := m.SpawnWorker(ctx, "nonexistent", "worker", "test", "prompt")
	if err == nil {
		t.Error("expected error when team does not exist")
	}
}

func TestSendMessageDelivers(t *testing.T) {
	store := tasks.NewStore()
	m := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")
	defer m.Close()

	// Manually create a worker with an InputCh to test SendMessage
	w := &Worker{
		ID:      "agent-test01",
		InputCh: make(chan string, 10),
		Status:  WorkerRunning,
	}
	m.mu.Lock()
	m.workers["agent-test01"] = w
	m.mu.Unlock()

	// Send a message
	if err := m.SendMessage("agent-test01", "hello worker"); err != nil {
		t.Errorf("SendMessage error: %v", err)
	}

	// Verify it was received
	select {
	case msg := <-w.InputCh:
		if msg != "hello worker" {
			t.Errorf("expected 'hello worker', got %q", msg)
		}
	default:
		t.Error("expected message in InputCh")
	}

	// Send to non-existent worker
	if err := m.SendMessage("nonexistent", "hello"); err == nil {
		t.Error("expected error sending to non-existent worker")
	}
}

func TestStopWorkerCancels(t *testing.T) {
	store := tasks.NewStore()
	m := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")
	defer m.Close()

	cancelled := false
	task := store.Create("test", "local_agent")
	w := &Worker{
		ID:     "agent-stop01",
		TaskID: task.ID,
		Cancel: func() { cancelled = true },
		Status: WorkerRunning,
	}
	m.mu.Lock()
	m.workers["agent-stop01"] = w
	m.mu.Unlock()

	if err := m.StopWorker("agent-stop01"); err != nil {
		t.Errorf("StopWorker error: %v", err)
	}

	if !cancelled {
		t.Error("expected cancel function to be called")
	}
	if w.Status != WorkerStopped {
		t.Errorf("expected status 'stopped', got %q", w.Status)
	}

	// Stop non-existent
	if err := m.StopWorker("nonexistent"); err == nil {
		t.Error("expected error stopping non-existent worker")
	}
}

func TestTaskNotificationToXML(t *testing.T) {
	notif := &TaskNotification{
		TaskID:      "agent-abc123",
		Status:      "completed",
		Summary:     `Agent "Fix bug" completed`,
		Result:      "Fixed the null pointer in validate.ts:42",
		TotalTokens: 1500,
		ToolUses:    5,
		DurationMs:  3200,
	}

	xml := notif.ToXML()

	// Verify required XML elements
	expectedElements := []string{
		"<task-notification>",
		"</task-notification>",
		"<task-id>agent-abc123</task-id>",
		"<status>completed</status>",
		`<summary>Agent "Fix bug" completed</summary>`,
		"<result>Fixed the null pointer in validate.ts:42</result>",
		"<total_tokens>1500</total_tokens>",
		"<tool_uses>5</tool_uses>",
		"<duration_ms>3200</duration_ms>",
		"<usage>",
		"</usage>",
	}

	for _, expected := range expectedElements {
		if !strings.Contains(xml, expected) {
			t.Errorf("XML missing expected element %q\nGot:\n%s", expected, xml)
		}
	}
}

func TestTaskNotificationToXMLNoResult(t *testing.T) {
	notif := &TaskNotification{
		TaskID:  "agent-def456",
		Status:  "failed",
		Summary: "Agent failed with error",
		Result:  "",
	}

	xml := notif.ToXML()

	// Result should be omitted when empty
	if strings.Contains(xml, "<result>") {
		t.Error("expected no <result> element when Result is empty")
	}
}

func TestGetNotificationsChannel(t *testing.T) {
	store := tasks.NewStore()
	m := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")

	ch := m.GetNotifications()
	if ch == nil {
		t.Fatal("GetNotifications returned nil channel")
	}

	// Send a notification directly (simulate worker completion)
	go func() {
		m.notifyCh <- TaskNotification{
			TaskID:  "agent-test",
			Status:  "completed",
			Summary: "test completed",
		}
	}()

	select {
	case notif := <-ch:
		if notif.TaskID != "agent-test" {
			t.Errorf("expected TaskID 'agent-test', got %q", notif.TaskID)
		}
		if notif.Status != "completed" {
			t.Errorf("expected status 'completed', got %q", notif.Status)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for notification")
	}

	m.Close()
}

func TestLeaderProcessNotification(t *testing.T) {
	store := tasks.NewStore()
	mgr := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")
	defer mgr.Close()

	leader := NewLeader(mgr)

	notif := TaskNotification{
		TaskID:      "agent-xyz789",
		Status:      "completed",
		Summary:     `Agent "Research auth" completed`,
		Result:      "Found issue in auth module",
		TotalTokens: 1000,
		ToolUses:    3,
		DurationMs:  2000,
	}

	msg := leader.ProcessNotification(notif)

	// Should be a user-role message
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %q", msg.Role)
	}

	// Content should contain the XML
	if len(msg.Content) == 0 {
		t.Fatal("expected at least one content block")
	}

	text := msg.Content[0].Text
	if !strings.Contains(text, "<task-notification>") {
		t.Error("expected XML task-notification in message content")
	}
	if !strings.Contains(text, "agent-xyz789") {
		t.Error("expected agent ID in message content")
	}
}

func TestGenerateRandomName(t *testing.T) {
	names := make(map[string]bool)
	for i := 0; i < 100; i++ {
		name := generateRandomName()
		if !strings.HasPrefix(name, "agent-") {
			t.Errorf("expected prefix 'agent-', got %q", name)
		}
		if len(name) != 12 { // "agent-" (6) + 6 hex chars
			t.Errorf("expected length 12, got %d for %q", len(name), name)
		}
		if names[name] {
			t.Errorf("duplicate name generated: %s", name)
		}
		names[name] = true
	}
}

func TestSpawnWorkerDuplicateID(t *testing.T) {
	store := tasks.NewStore()
	m := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")
	defer m.Close()

	m.CreateTeam("gamma")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := m.SpawnWorker(ctx, "gamma", "tester", "first worker", "prompt1")
	if err != nil {
		t.Fatalf("first SpawnWorker error: %v", err)
	}

	// Same name in same team should fail
	_, err = m.SpawnWorker(ctx, "gamma", "tester", "second worker", "prompt2")
	if err == nil {
		t.Error("expected error for duplicate agent name in same team")
	}
}

func TestSpawnWorkerEmptyName(t *testing.T) {
	store := tasks.NewStore()
	m := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")
	defer m.Close()

	m.CreateTeam("delta")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w, err := m.SpawnWorker(ctx, "delta", "", "auto-named worker", "prompt")
	if err != nil {
		t.Fatalf("SpawnWorker error: %v", err)
	}

	// Should have a random name in agent-{hex6} format
	if !strings.HasPrefix(w.AgentName, "agent-") {
		t.Errorf("expected auto-generated name with 'agent-' prefix, got %q", w.AgentName)
	}
	// ID should be agentName@teamName
	if !strings.HasSuffix(w.ID, "@delta") {
		t.Errorf("expected ID to end with '@delta', got %q", w.ID)
	}
}

func TestDeleteTeamCancelsWorkers(t *testing.T) {
	store := tasks.NewStore()
	m := NewManager(store, nil, nil, nil, "/tmp", "/tmp", "test-session")
	defer m.Close()

	m.CreateTeam("gamma")

	// Add workers manually
	cancelled1, cancelled2 := false, false
	w1 := &Worker{ID: "agent-w1", Cancel: func() { cancelled1 = true }, Status: WorkerRunning}
	w2 := &Worker{ID: "agent-w2", Cancel: func() { cancelled2 = true }, Status: WorkerRunning}

	m.mu.Lock()
	m.workers["agent-w1"] = w1
	m.workers["agent-w2"] = w2
	m.teams["gamma"].Workers["agent-w1"] = w1
	m.teams["gamma"].Workers["agent-w2"] = w2
	m.mu.Unlock()

	if err := m.DeleteTeam("gamma"); err != nil {
		t.Errorf("DeleteTeam error: %v", err)
	}

	if !cancelled1 || !cancelled2 {
		t.Error("expected both workers to be cancelled")
	}

	// Workers should be removed from global map
	if _, ok := m.GetWorker("agent-w1"); ok {
		t.Error("worker w1 should be removed after team deletion")
	}
	if _, ok := m.GetWorker("agent-w2"); ok {
		t.Error("worker w2 should be removed after team deletion")
	}
}

// TestUserMessageFormat verifies api.UserMessage produces the expected format.
func TestUserMessageFormat(t *testing.T) {
	msg := api.UserMessage("test content")
	if msg.Role != "user" {
		t.Errorf("expected role 'user', got %q", msg.Role)
	}
	if len(msg.Content) != 1 || msg.Content[0].Text != "test content" {
		t.Error("unexpected message content")
	}
}

// --- Coordinator mode tests ---

func TestIsCoordinatorMode(t *testing.T) {
	// Save and restore env
	orig := os.Getenv("CLAUDE_CODE_COORDINATOR_MODE")
	defer os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", orig)

	// Not set
	os.Unsetenv("CLAUDE_CODE_COORDINATOR_MODE")
	if IsCoordinatorMode() {
		t.Error("expected false when env not set")
	}

	// Set to "1"
	os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	if !IsCoordinatorMode() {
		t.Error("expected true when env is '1'")
	}

	// Set to "true"
	os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "true")
	if !IsCoordinatorMode() {
		t.Error("expected true when env is 'true'")
	}

	// Set to "yes"
	os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "yes")
	if !IsCoordinatorMode() {
		t.Error("expected true when env is 'yes'")
	}

	// Set to "0" (falsy)
	os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "0")
	if IsCoordinatorMode() {
		t.Error("expected false when env is '0'")
	}

	// Set to empty string
	os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "")
	if IsCoordinatorMode() {
		t.Error("expected false when env is empty")
	}
}

func TestMatchSessionMode(t *testing.T) {
	orig := os.Getenv("CLAUDE_CODE_COORDINATOR_MODE")
	defer os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", orig)

	// Empty session mode -- no switch
	os.Unsetenv("CLAUDE_CODE_COORDINATOR_MODE")
	if msg := MatchSessionMode(""); msg != "" {
		t.Errorf("expected empty message for empty session mode, got %q", msg)
	}

	// Session is coordinator but env is not -- should switch to coordinator
	os.Unsetenv("CLAUDE_CODE_COORDINATOR_MODE")
	msg := MatchSessionMode("coordinator")
	if msg == "" {
		t.Error("expected switch message")
	}
	if !strings.Contains(msg, "Entered coordinator mode") {
		t.Errorf("expected 'Entered coordinator mode', got %q", msg)
	}
	if !IsCoordinatorMode() {
		t.Error("expected coordinator mode to be active after switch")
	}

	// Session is normal but env is coordinator -- should switch to normal
	os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	msg = MatchSessionMode("normal")
	if msg == "" {
		t.Error("expected switch message")
	}
	if !strings.Contains(msg, "Exited coordinator mode") {
		t.Errorf("expected 'Exited coordinator mode', got %q", msg)
	}
	if IsCoordinatorMode() {
		t.Error("expected coordinator mode to be inactive after switch")
	}

	// Same mode -- no switch
	os.Unsetenv("CLAUDE_CODE_COORDINATOR_MODE")
	if msg := MatchSessionMode("normal"); msg != "" {
		t.Errorf("expected empty message when modes match, got %q", msg)
	}
}

func TestGetCoordinatorSystemPrompt(t *testing.T) {
	orig := os.Getenv("CLAUDE_CODE_SIMPLE")
	defer os.Setenv("CLAUDE_CODE_SIMPLE", orig)
	os.Unsetenv("CLAUDE_CODE_SIMPLE")

	prompt := GetCoordinatorSystemPrompt()

	// Verify key sections are present
	expectedPhrases := []string{
		"You are Claude Code",
		"coordinator",
		"## 1. Your Role",
		"## 2. Your Tools",
		"## 3. Workers",
		"## 4. Task Workflow",
		"## 5. Writing Worker Prompts",
		"## 6. Example Session",
		"<task-notification>",
		"Agent",
		"SendMessage",
		"TaskStop",
		"subagent_type",
	}
	for _, phrase := range expectedPhrases {
		if !strings.Contains(prompt, phrase) {
			t.Errorf("system prompt missing expected phrase: %q", phrase)
		}
	}
}

func TestGetCoordinatorSystemPromptSimpleMode(t *testing.T) {
	orig := os.Getenv("CLAUDE_CODE_SIMPLE")
	defer os.Setenv("CLAUDE_CODE_SIMPLE", orig)

	os.Setenv("CLAUDE_CODE_SIMPLE", "1")
	prompt := GetCoordinatorSystemPrompt()

	if !strings.Contains(prompt, "Bash, Read, and Edit") {
		t.Error("simple mode prompt should mention Bash, Read, and Edit tools")
	}
}

func TestGetCoordinatorUserContext(t *testing.T) {
	orig := os.Getenv("CLAUDE_CODE_COORDINATOR_MODE")
	defer os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", orig)

	// Not in coordinator mode -- should return empty
	os.Unsetenv("CLAUDE_CODE_COORDINATOR_MODE")
	ctx := GetCoordinatorUserContext(nil, "")
	if len(ctx) != 0 {
		t.Error("expected empty context when not in coordinator mode")
	}

	// In coordinator mode
	os.Setenv("CLAUDE_CODE_COORDINATOR_MODE", "1")
	ctx = GetCoordinatorUserContext(nil, "")
	if _, ok := ctx["workerToolsContext"]; !ok {
		t.Error("expected workerToolsContext key")
	}
	if !strings.Contains(ctx["workerToolsContext"], "Agent") {
		t.Error("expected worker tools to include Agent")
	}

	// With MCP clients
	ctx = GetCoordinatorUserContext([]string{"github", "filesystem"}, "")
	if !strings.Contains(ctx["workerToolsContext"], "github, filesystem") {
		t.Error("expected MCP client names in context")
	}

	// With scratchpad
	ctx = GetCoordinatorUserContext(nil, "/tmp/scratch")
	if !strings.Contains(ctx["workerToolsContext"], "/tmp/scratch") {
		t.Error("expected scratchpad dir in context")
	}
	if !strings.Contains(ctx["workerToolsContext"], "Scratchpad directory") {
		t.Error("expected scratchpad label in context")
	}
}

func TestCoordinatorConfig(t *testing.T) {
	cfg := CoordinatorConfig{
		Enabled:        true,
		ScratchpadDir:  "/tmp/scratch",
		MCPClientNames: []string{"github"},
	}

	if !cfg.Enabled {
		t.Error("expected Enabled to be true")
	}
	if cfg.ScratchpadDir != "/tmp/scratch" {
		t.Errorf("expected scratchpad '/tmp/scratch', got %q", cfg.ScratchpadDir)
	}
	if len(cfg.MCPClientNames) != 1 || cfg.MCPClientNames[0] != "github" {
		t.Error("unexpected MCP client names")
	}
}

func TestIsEnvTruthy(t *testing.T) {
	tests := []struct {
		val    string
		expect bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"yes", true},
		{"YES", true},
		{"0", false},
		{"false", false},
		{"no", false},
		{"", false},
		{"  ", false},
		{" 1 ", true},
		{" true ", true},
	}

	for _, tt := range tests {
		got := isEnvTruthy(tt.val)
		if got != tt.expect {
			t.Errorf("isEnvTruthy(%q) = %v, want %v", tt.val, got, tt.expect)
		}
	}
}
