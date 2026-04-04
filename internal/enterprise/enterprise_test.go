package enterprise

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPolicyLimitsManager_Fetch(t *testing.T) {
	limits := PolicyLimits{
		DisabledTools:    []string{"BashTool"},
		DisabledCommands: []string{"model"},
		MaxTurns:         50,
		DisableWebSearch: true,
		CustomMessage:    "Test org policy",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(limits)
	}))
	defer server.Close()

	mgr := NewPolicyLimitsManager(server.URL, func() string { return "test-token" })

	if err := mgr.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	got := mgr.GetLimits()
	if got.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want 50", got.MaxTurns)
	}
	if !got.DisableWebSearch {
		t.Error("DisableWebSearch = false, want true")
	}
	if got.CustomMessage != "Test org policy" {
		t.Errorf("CustomMessage = %q, want %q", got.CustomMessage, "Test org policy")
	}
	if mgr.IsToolAllowed("BashTool") {
		t.Error("IsToolAllowed(BashTool) = true, want false")
	}
	if !mgr.IsToolAllowed("ReadTool") {
		t.Error("IsToolAllowed(ReadTool) = false, want true")
	}
	if mgr.IsCommandAllowed("model") {
		t.Error("IsCommandAllowed(model) = true, want false")
	}
	if !mgr.IsCommandAllowed("help") {
		t.Error("IsCommandAllowed(help) = false, want true")
	}
}

func TestPolicyLimitsManager_ETagCaching(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if etag := r.Header.Get("If-None-Match"); etag == `"etag-v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		limits := PolicyLimits{
			DisabledTools: []string{"AgentTool"},
			MaxTurns:      10,
		}
		w.Header().Set("ETag", `"etag-v1"`)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(limits)
	}))
	defer server.Close()

	mgr := NewPolicyLimitsManager(server.URL, func() string { return "tok" })

	// First fetch: should get 200 with limits
	if err := mgr.Fetch(context.Background()); err != nil {
		t.Fatalf("First fetch failed: %v", err)
	}
	if mgr.IsToolAllowed("AgentTool") {
		t.Error("After first fetch: IsToolAllowed(AgentTool) = true, want false")
	}
	if callCount != 1 {
		t.Errorf("Call count after first fetch = %d, want 1", callCount)
	}

	// Second fetch: should get 304, limits unchanged
	if err := mgr.Fetch(context.Background()); err != nil {
		t.Fatalf("Second fetch failed: %v", err)
	}
	if mgr.IsToolAllowed("AgentTool") {
		t.Error("After second fetch: IsToolAllowed(AgentTool) = true, want false (304 should preserve)")
	}
	if callCount != 2 {
		t.Errorf("Call count after second fetch = %d, want 2", callCount)
	}
}

func TestPolicyLimitsManager_AuthHeader(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(PolicyLimits{})
	}))
	defer server.Close()

	token := "my-oauth-token-123"
	mgr := NewPolicyLimitsManager(server.URL, func() string { return token })

	if err := mgr.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	expected := "Bearer " + token
	if receivedAuth != expected {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, expected)
	}
}

func TestPolicyLimitsManager_IsToolAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limits := PolicyLimits{
			DisabledTools: []string{"BashTool"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(limits)
	}))
	defer server.Close()

	mgr := NewPolicyLimitsManager(server.URL, func() string { return "tok" })
	if err := mgr.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if mgr.IsToolAllowed("BashTool") {
		t.Error("IsToolAllowed(BashTool) = true, want false")
	}
	if !mgr.IsToolAllowed("ReadTool") {
		t.Error("IsToolAllowed(ReadTool) = false, want true")
	}
	if !mgr.IsToolAllowed("EditTool") {
		t.Error("IsToolAllowed(EditTool) = false, want true")
	}
}

func TestPolicyLimitsManager_EmptyLimits(t *testing.T) {
	// No fetch performed -- should return true for everything
	mgr := NewPolicyLimitsManager("http://localhost:0", func() string { return "" })

	if !mgr.IsToolAllowed("BashTool") {
		t.Error("IsToolAllowed(BashTool) = false, want true (no limits fetched)")
	}
	if !mgr.IsToolAllowed("AgentTool") {
		t.Error("IsToolAllowed(AgentTool) = false, want true (no limits fetched)")
	}
	if !mgr.IsCommandAllowed("model") {
		t.Error("IsCommandAllowed(model) = false, want true (no limits fetched)")
	}

	limits := mgr.GetLimits()
	if limits == nil {
		t.Fatal("GetLimits() = nil, want empty PolicyLimits")
	}
	if len(limits.DisabledTools) != 0 {
		t.Errorf("DisabledTools = %v, want empty", limits.DisabledTools)
	}
	if limits.MaxTurns != 0 {
		t.Errorf("MaxTurns = %d, want 0", limits.MaxTurns)
	}
}

// --- Settings Sync Tests ---

func TestSettingsSyncManager_Upload(t *testing.T) {
	var receivedMethod string
	var receivedAuth string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedAuth = r.Header.Get("Authorization")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mgr := NewSettingsSyncManager(server.URL, func() string { return "upload-token" }, false)

	settings := json.RawMessage(`{"model":"claude-3-opus","permissionMode":"default"}`)
	err := mgr.Upload(context.Background(), settings)
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if receivedMethod != http.MethodPut {
		t.Errorf("Method = %s, want PUT", receivedMethod)
	}
	if receivedAuth != "Bearer upload-token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer upload-token")
	}
	if string(receivedBody) != string(settings) {
		t.Errorf("Body = %s, want %s", string(receivedBody), string(settings))
	}
}

func TestSettingsSyncManager_Download(t *testing.T) {
	expectedSettings := `{"model":"claude-3-opus","customInstructions":"team rules"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %s, want GET", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer dl-token" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer dl-token")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(expectedSettings))
	}))
	defer server.Close()

	mgr := NewSettingsSyncManager(server.URL, func() string { return "dl-token" }, true)

	got, err := mgr.Download(context.Background())
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if string(got) != expectedSettings {
		t.Errorf("Downloaded settings = %s, want %s", string(got), expectedSettings)
	}
}

func TestSettingsSyncManager_SyncRemote(t *testing.T) {
	remoteSettings := `{"model":"remote-model"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Sync(remote) should GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(remoteSettings))
	}))
	defer server.Close()

	mgr := NewSettingsSyncManager(server.URL, func() string { return "tok" }, true)

	got, err := mgr.Sync(context.Background(), json.RawMessage(`{"model":"local-model"}`))
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if string(got) != remoteSettings {
		t.Errorf("Sync result = %s, want %s (remote settings)", string(got), remoteSettings)
	}
}

func TestSettingsSyncManager_SyncLocal(t *testing.T) {
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	localSettings := json.RawMessage(`{"model":"local-model"}`)
	mgr := NewSettingsSyncManager(server.URL, func() string { return "tok" }, false)

	got, err := mgr.Sync(context.Background(), localSettings)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if receivedMethod != http.MethodPut {
		t.Errorf("Sync(local) should PUT, got %s", receivedMethod)
	}
	if string(got) != string(localSettings) {
		t.Errorf("Sync result = %s, want %s (local settings pass-through)", string(got), string(localSettings))
	}
}

// --- Team Memory Sync Tests ---

func TestTeamMemorySyncManager_Pull(t *testing.T) {
	entries := []TeamMemoryEntry{
		{Key: "coding-style", Content: "Use tabs", UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Key: "deploy-steps", Content: "Run make deploy", UpdatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
	}

	var receivedRepo string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Pull should GET, got %s", r.Method)
		}
		receivedRepo = r.URL.Query().Get("repo")
		if auth := r.Header.Get("Authorization"); auth != "Bearer pull-tok" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer pull-tok")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(entries)
	}))
	defer server.Close()

	mgr := NewTeamMemorySyncManager(server.URL, func() string { return "pull-tok" }, "git@github.com:org/repo.git")

	got, err := mgr.Pull(context.Background())
	if err != nil {
		t.Fatalf("Pull failed: %v", err)
	}

	if receivedRepo == "" {
		t.Error("repo query param was empty")
	}
	if receivedRepo != mgr.RepoKey() {
		t.Errorf("repo query param = %q, want %q", receivedRepo, mgr.RepoKey())
	}
	if len(got) != 2 {
		t.Fatalf("Pull returned %d entries, want 2", len(got))
	}
	if got[0].Key != "coding-style" {
		t.Errorf("Entry[0].Key = %q, want %q", got[0].Key, "coding-style")
	}
}

func TestTeamMemorySyncManager_Push(t *testing.T) {
	var receivedBody pushPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Push should POST, got %s", r.Method)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer push-tok" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer push-tok")
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mgr := NewTeamMemorySyncManager(server.URL, func() string { return "push-tok" }, "git@github.com:org/repo.git")

	entries := []TeamMemoryEntry{
		{Key: "new-rule", Content: "Always test", UpdatedAt: time.Now()},
	}

	err := mgr.Push(context.Background(), entries)
	if err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	if receivedBody.Repo != mgr.RepoKey() {
		t.Errorf("Push payload repo = %q, want %q", receivedBody.Repo, mgr.RepoKey())
	}
	if len(receivedBody.Entries) != 1 {
		t.Fatalf("Push payload entries = %d, want 1", len(receivedBody.Entries))
	}
	if receivedBody.Entries[0].Key != "new-rule" {
		t.Errorf("Push entry key = %q, want %q", receivedBody.Entries[0].Key, "new-rule")
	}
}

func TestTeamMemorySyncManager_RepoKey(t *testing.T) {
	mgr1 := NewTeamMemorySyncManager("http://localhost", func() string { return "" }, "git@github.com:org/repo.git")
	mgr2 := NewTeamMemorySyncManager("http://localhost", func() string { return "" }, "git@github.com:org/repo.git")
	mgr3 := NewTeamMemorySyncManager("http://localhost", func() string { return "" }, "git@github.com:other/repo.git")

	// Same URL should produce same key
	if mgr1.RepoKey() != mgr2.RepoKey() {
		t.Errorf("Same URL produced different keys: %q vs %q", mgr1.RepoKey(), mgr2.RepoKey())
	}

	// Different URL should produce different key
	if mgr1.RepoKey() == mgr3.RepoKey() {
		t.Errorf("Different URLs produced same key: %q", mgr1.RepoKey())
	}

	// Key should be 16 hex chars
	if len(mgr1.RepoKey()) != 16 {
		t.Errorf("RepoKey length = %d, want 16", len(mgr1.RepoKey()))
	}
}
