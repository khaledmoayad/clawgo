package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/hooks"
	"github.com/khaledmoayad/clawgo/internal/skills"
)

func TestParseManifest_Valid(t *testing.T) {
	data := []byte(`{
		"name": "test-plugin",
		"description": "A test plugin",
		"version": "1.0.0",
		"author": {"name": "Test Author", "url": "https://example.com"},
		"skills": ["./skills"],
		"dependencies": ["other-plugin"]
	}`)

	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "test-plugin" {
		t.Errorf("expected name 'test-plugin', got %q", m.Name)
	}
	if m.Description != "A test plugin" {
		t.Errorf("expected description 'A test plugin', got %q", m.Description)
	}
	if m.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", m.Version)
	}
	if m.Author == nil || m.Author.Name != "Test Author" {
		t.Errorf("expected author name 'Test Author', got %v", m.Author)
	}
	if len(m.Skills) != 1 || m.Skills[0] != "./skills" {
		t.Errorf("expected skills ['./skills'], got %v", m.Skills)
	}
	if len(m.Dependencies) != 1 || m.Dependencies[0] != "other-plugin" {
		t.Errorf("expected dependencies ['other-plugin'], got %v", m.Dependencies)
	}
}

func TestParseManifest_MissingName(t *testing.T) {
	data := []byte(`{"description": "no name"}`)

	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for missing name, got nil")
	}
	if err.Error() != "plugin manifest: name is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestParseManifest_InvalidJSON(t *testing.T) {
	data := []byte(`{not valid json}`)

	_, err := ParseManifest(data)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestParseManifest_PackageJSON(t *testing.T) {
	data := []byte(`{
		"name": "my-package",
		"version": "2.0.0",
		"claude-plugin": {
			"name": "my-plugin",
			"description": "From package.json"
		}
	}`)

	m, err := ParseManifest(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "my-plugin" {
		t.Errorf("expected name 'my-plugin', got %q", m.Name)
	}
	if m.Description != "From package.json" {
		t.Errorf("expected description 'From package.json', got %q", m.Description)
	}
}

func TestFindManifest(t *testing.T) {
	dir := t.TempDir()

	// No manifest initially
	_, err := FindManifest(dir)
	if err == nil {
		t.Fatal("expected error when no manifest exists")
	}

	// Create claude-plugin.json
	manifestPath := filepath.Join(dir, "claude-plugin.json")
	if err := os.WriteFile(manifestPath, []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := FindManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != manifestPath {
		t.Errorf("expected %s, got %s", manifestPath, found)
	}
}

func TestFindManifest_PluginJSON(t *testing.T) {
	dir := t.TempDir()

	// Create plugin.json (second priority)
	manifestPath := filepath.Join(dir, "plugin.json")
	if err := os.WriteFile(manifestPath, []byte(`{"name":"test2"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := FindManifest(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != manifestPath {
		t.Errorf("expected %s, got %s", manifestPath, found)
	}
}

func TestRegistryRegisterBuiltin_GetEnabled(t *testing.T) {
	r := NewRegistry()

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "test-builtin",
		Description:    "A test built-in plugin",
		Version:        "1.0.0",
		DefaultEnabled: true,
		Skills: []*skills.Skill{
			{Name: "builtin-skill", Description: "A builtin skill", Source: "plugin"},
		},
	})

	// LoadAll with nil config to just merge builtins
	result := r.LoadAll(context.Background(), nil, nil, "")

	if len(result.Enabled) != 1 {
		t.Fatalf("expected 1 enabled plugin, got %d", len(result.Enabled))
	}
	if result.Enabled[0].Name != "test-builtin" {
		t.Errorf("expected name 'test-builtin', got %q", result.Enabled[0].Name)
	}
	if !result.Enabled[0].IsBuiltin {
		t.Error("expected IsBuiltin=true")
	}

	// Verify GetEnabled returns it
	enabled := r.GetEnabled()
	if len(enabled) != 1 {
		t.Fatalf("GetEnabled: expected 1, got %d", len(enabled))
	}
}

func TestRegistrySetEnabled(t *testing.T) {
	r := NewRegistry()

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "toggle-test",
		Description:    "Toggleable plugin",
		DefaultEnabled: true,
	})

	r.LoadAll(context.Background(), nil, nil, "")

	pluginID := "toggle-test@builtin"

	// Initially enabled
	enabled := r.GetEnabled()
	if len(enabled) != 1 {
		t.Fatalf("expected 1 enabled, got %d", len(enabled))
	}

	// Disable it
	r.SetEnabled(pluginID, false)
	enabled = r.GetEnabled()
	if len(enabled) != 0 {
		t.Errorf("expected 0 enabled after disable, got %d", len(enabled))
	}

	disabled := r.GetDisabled()
	if len(disabled) != 1 {
		t.Errorf("expected 1 disabled after disable, got %d", len(disabled))
	}

	// Re-enable
	r.SetEnabled(pluginID, true)
	enabled = r.GetEnabled()
	if len(enabled) != 1 {
		t.Errorf("expected 1 enabled after re-enable, got %d", len(enabled))
	}
}

func TestGetMergedHooks(t *testing.T) {
	r := NewRegistry()

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "hook-plugin-a",
		Description:    "Plugin A with hooks",
		DefaultEnabled: true,
		Hooks: hooks.HooksConfig{
			hooks.PreToolUse: {
				{Matcher: "Bash(*)", Hooks: []hooks.HookCommand{
					{Type: hooks.CommandType, Command: "echo pre-a"},
				}},
			},
		},
	})

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "hook-plugin-b",
		Description:    "Plugin B with hooks",
		DefaultEnabled: true,
		Hooks: hooks.HooksConfig{
			hooks.PreToolUse: {
				{Matcher: "FileWrite(*)", Hooks: []hooks.HookCommand{
					{Type: hooks.CommandType, Command: "echo pre-b"},
				}},
			},
			hooks.PostToolUse: {
				{Matcher: "", Hooks: []hooks.HookCommand{
					{Type: hooks.CommandType, Command: "echo post-b"},
				}},
			},
		},
	})

	r.LoadAll(context.Background(), nil, nil, "")

	merged := r.GetMergedHooks()

	// PreToolUse should have matchers from both plugins
	if len(merged[hooks.PreToolUse]) != 2 {
		t.Errorf("expected 2 PreToolUse matchers, got %d", len(merged[hooks.PreToolUse]))
	}

	// PostToolUse should have 1 matcher from plugin B
	if len(merged[hooks.PostToolUse]) != 1 {
		t.Errorf("expected 1 PostToolUse matcher, got %d", len(merged[hooks.PostToolUse]))
	}
}

func TestGetMergedSkills_EnabledOnly(t *testing.T) {
	r := NewRegistry()

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "skills-enabled",
		Description:    "Enabled plugin with skills",
		DefaultEnabled: true,
		Skills: []*skills.Skill{
			{Name: "skill-a", Source: "plugin"},
			{Name: "skill-b", Source: "plugin"},
		},
	})

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "skills-disabled",
		Description:    "Disabled plugin with skills",
		DefaultEnabled: false,
		Skills: []*skills.Skill{
			{Name: "skill-c", Source: "plugin"},
		},
	})

	r.LoadAll(context.Background(), nil, nil, "")

	merged := r.GetMergedSkills()
	if len(merged) != 2 {
		t.Errorf("expected 2 skills from enabled plugin only, got %d", len(merged))
	}

	// Verify only skills from enabled plugin
	names := make(map[string]bool)
	for _, s := range merged {
		names[s.Name] = true
	}
	if !names["skill-a"] || !names["skill-b"] {
		t.Errorf("expected skill-a and skill-b, got %v", names)
	}
	if names["skill-c"] {
		t.Error("skill-c should not be in merged skills (plugin disabled)")
	}
}

func TestLoadPluginFromPath(t *testing.T) {
	dir := t.TempDir()

	// Create manifest
	manifest := PluginManifest{
		Name:        "local-test",
		Description: "A local test plugin",
		Version:     "0.1.0",
		Skills:      []string{"./skills"},
	}
	manifestData, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "claude-plugin.json"), manifestData, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create skills directory with a skill file
	skillsDir := filepath.Join(dir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "test-skill.md"), []byte("# Test Skill\nA test skill for the plugin."), 0o644); err != nil {
		t.Fatal(err)
	}

	plugin, err := LoadPluginFromPath(dir, "test-source")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plugin.Name != "local-test" {
		t.Errorf("expected name 'local-test', got %q", plugin.Name)
	}
	if plugin.Source != "test-source" {
		t.Errorf("expected source 'test-source', got %q", plugin.Source)
	}
	if len(plugin.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(plugin.Skills))
	}
	if plugin.Skills[0].Name != "test-skill" {
		t.Errorf("expected skill name 'test-skill', got %q", plugin.Skills[0].Name)
	}
}

func TestRegistryBuiltin_UserDisable(t *testing.T) {
	r := NewRegistry()

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "user-toggle",
		Description:    "User can disable this",
		DefaultEnabled: true,
	})

	// User has explicitly disabled this plugin
	enabledPlugins := map[string]bool{
		"user-toggle@builtin": false,
	}

	result := r.LoadAll(context.Background(), nil, enabledPlugins, "")

	if len(result.Enabled) != 0 {
		t.Errorf("expected 0 enabled, got %d", len(result.Enabled))
	}
	if len(result.Disabled) != 1 {
		t.Errorf("expected 1 disabled, got %d", len(result.Disabled))
	}
}

func TestRegistryBuiltin_Unavailable(t *testing.T) {
	r := NewRegistry()

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "unavailable",
		Description:    "Not available on this system",
		DefaultEnabled: true,
		IsAvailable:    func() bool { return false },
	})

	result := r.LoadAll(context.Background(), nil, nil, "")

	if len(result.Enabled) != 0 {
		t.Errorf("expected 0 enabled (unavailable), got %d", len(result.Enabled))
	}
	if len(result.Disabled) != 0 {
		t.Errorf("expected 0 disabled (unavailable hidden), got %d", len(result.Disabled))
	}
}

func TestValidateManifest(t *testing.T) {
	tests := []struct {
		name    string
		m       *PluginManifest
		wantErr bool
	}{
		{
			name:    "valid",
			m:       &PluginManifest{Name: "valid-plugin"},
			wantErr: false,
		},
		{
			name:    "empty name",
			m:       &PluginManifest{Name: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateManifest(tt.m)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseSource(t *testing.T) {
	tests := []struct {
		source      string
		wantURL     string
		wantDirName string
	}{
		{
			source:      "github:owner/repo",
			wantURL:     "https://github.com/owner/repo.git",
			wantDirName: "owner__repo",
		},
		{
			source:      "https://github.com/owner/repo.git",
			wantURL:     "https://github.com/owner/repo.git",
			wantDirName: "github.com__owner__repo",
		},
		{
			source:      "git@github.com:owner/repo.git",
			wantURL:     "git@github.com:owner/repo.git",
			wantDirName: "github.com__owner__repo",
		},
		{
			source:      "unsupported-source",
			wantURL:     "",
			wantDirName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			gotURL, gotDir := parseSource(tt.source)
			if gotURL != tt.wantURL {
				t.Errorf("parseSource(%q) URL = %q, want %q", tt.source, gotURL, tt.wantURL)
			}
			if gotDir != tt.wantDirName {
				t.Errorf("parseSource(%q) dir = %q, want %q", tt.source, gotDir, tt.wantDirName)
			}
		})
	}
}

// --- Dependency resolution tests ---

func TestResolveDependencies_MissingDep(t *testing.T) {
	r := NewRegistry()

	// Register plugin A that depends on plugin B
	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "plugin-a",
		Description:    "Depends on plugin-b",
		DefaultEnabled: true,
	})

	enabledPlugins := map[string]bool{
		"plugin-a@builtin": true,
	}
	r.LoadAll(context.Background(), nil, enabledPlugins, "")

	// Manually set dependency on the loaded plugin
	r.mu.Lock()
	for _, p := range r.plugins {
		if p.Name == "plugin-a" {
			p.Manifest.Dependencies = []string{"plugin-b"}
		}
	}
	r.mu.Unlock()

	pluginA, _ := r.Get("plugin-a@builtin")
	if pluginA == nil {
		t.Fatal("plugin-a not found")
	}

	missing := r.ResolveDependencies(pluginA)
	if len(missing) != 1 || missing[0] != "plugin-b" {
		t.Errorf("expected [plugin-b], got %v", missing)
	}
}

func TestResolveDependencies_SatisfiedDep(t *testing.T) {
	r := NewRegistry()

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "plugin-a",
		Description:    "Depends on plugin-b",
		DefaultEnabled: true,
	})
	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "plugin-b",
		Description:    "Required by plugin-a",
		DefaultEnabled: true,
	})

	r.LoadAll(context.Background(), nil, nil, "")

	// Set dependency
	r.mu.Lock()
	for _, p := range r.plugins {
		if p.Name == "plugin-a" {
			p.Manifest.Dependencies = []string{"plugin-b"}
		}
	}
	r.mu.Unlock()

	pluginA, _ := r.Get("plugin-a@builtin")
	if pluginA == nil {
		t.Fatal("plugin-a not found")
	}

	missing := r.ResolveDependencies(pluginA)
	if len(missing) != 0 {
		t.Errorf("expected no missing deps, got %v", missing)
	}
}

// --- Marketplace blocked tests ---

func TestIsMarketplaceBlocked(t *testing.T) {
	policy := &PluginPolicy{
		BlockedMarketplaces: []MarketplaceSource{
			{Source: "github", Owner: "evil", Repo: "malware"},
			{Source: "url", URL: "https://bad-marketplace.com/manifest.json"},
		},
	}

	tests := []struct {
		name    string
		source  MarketplaceSource
		blocked bool
	}{
		{
			name:    "blocked github source",
			source:  MarketplaceSource{Source: "github", Owner: "evil", Repo: "malware"},
			blocked: true,
		},
		{
			name:    "blocked url source",
			source:  MarketplaceSource{Source: "url", URL: "https://bad-marketplace.com/manifest.json"},
			blocked: true,
		},
		{
			name:    "allowed github source",
			source:  MarketplaceSource{Source: "github", Owner: "good", Repo: "plugins"},
			blocked: false,
		},
		{
			name:    "allowed url source",
			source:  MarketplaceSource{Source: "url", URL: "https://good-marketplace.com/manifest.json"},
			blocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMarketplaceBlocked(policy, tt.source)
			if got != tt.blocked {
				t.Errorf("IsMarketplaceBlocked() = %v, want %v", got, tt.blocked)
			}
		})
	}
}

// --- Strict known marketplaces tests ---

func TestIsMarketplaceAllowed_StrictMode(t *testing.T) {
	policy := &PluginPolicy{
		StrictKnownMarketplaces: []MarketplaceSource{
			{Source: "github", Owner: "anthropic", Repo: "official-plugins"},
			{Source: "settings", Name: "internal-marketplace"},
		},
	}

	tests := []struct {
		name    string
		source  MarketplaceSource
		allowed bool
	}{
		{
			name:    "allowed github source",
			source:  MarketplaceSource{Source: "github", Owner: "anthropic", Repo: "official-plugins"},
			allowed: true,
		},
		{
			name:    "allowed settings source",
			source:  MarketplaceSource{Source: "settings", Name: "internal-marketplace"},
			allowed: true,
		},
		{
			name:    "unlisted github source",
			source:  MarketplaceSource{Source: "github", Owner: "random", Repo: "plugins"},
			allowed: false,
		},
		{
			name:    "unlisted url source",
			source:  MarketplaceSource{Source: "url", URL: "https://random.com/market.json"},
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMarketplaceAllowed(policy, tt.source)
			if got != tt.allowed {
				t.Errorf("IsMarketplaceAllowed() = %v, want %v", got, tt.allowed)
			}
		})
	}
}

// --- Plugin customization lock tests ---

func TestIsPluginCustomizationLocked_BoolTrue(t *testing.T) {
	policy := &PluginPolicy{
		StrictPluginOnlyCustomization: json.RawMessage(`true`),
	}

	surfaces := []string{"skills", "agents", "hooks", "mcp"}
	for _, s := range surfaces {
		if !IsPluginCustomizationLocked(policy, s) {
			t.Errorf("expected surface %q to be locked when policy is true", s)
		}
	}
}

func TestIsPluginCustomizationLocked_ArraySpecific(t *testing.T) {
	policy := &PluginPolicy{
		StrictPluginOnlyCustomization: json.RawMessage(`["hooks", "mcp"]`),
	}

	if !IsPluginCustomizationLocked(policy, "hooks") {
		t.Error("expected hooks to be locked")
	}
	if !IsPluginCustomizationLocked(policy, "mcp") {
		t.Error("expected mcp to be locked")
	}
	if IsPluginCustomizationLocked(policy, "skills") {
		t.Error("expected skills to NOT be locked")
	}
	if IsPluginCustomizationLocked(policy, "agents") {
		t.Error("expected agents to NOT be locked")
	}
}

func TestIsPluginCustomizationLocked_NilPolicy(t *testing.T) {
	if IsPluginCustomizationLocked(nil, "hooks") {
		t.Error("expected no lock with nil policy")
	}
}

// --- Enable/disable persistence tests ---

func TestSaveEnabledState(t *testing.T) {
	dir := t.TempDir()

	// Save a plugin as enabled
	if err := SaveEnabledState(dir, "my-plugin@marketplace", true); err != nil {
		t.Fatalf("SaveEnabledState(true) error: %v", err)
	}

	// Read it back
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to parse settings: %v", err)
	}

	var enabledPlugins map[string]json.RawMessage
	if err := json.Unmarshal(settings["enabledPlugins"], &enabledPlugins); err != nil {
		t.Fatalf("failed to parse enabledPlugins: %v", err)
	}

	val := string(enabledPlugins["my-plugin@marketplace"])
	if val != "true" {
		t.Errorf("expected enabled=true, got %q", val)
	}

	// Save as disabled
	if err := SaveEnabledState(dir, "my-plugin@marketplace", false); err != nil {
		t.Fatalf("SaveEnabledState(false) error: %v", err)
	}

	data, _ = os.ReadFile(filepath.Join(dir, "settings.json"))
	json.Unmarshal(data, &settings)
	json.Unmarshal(settings["enabledPlugins"], &enabledPlugins)

	val = string(enabledPlugins["my-plugin@marketplace"])
	if val != "false" {
		t.Errorf("expected enabled=false, got %q", val)
	}
}

func TestSaveEnabledState_PreservesExisting(t *testing.T) {
	dir := t.TempDir()

	// Write initial settings with existing content
	initial := `{"customInstructions": "keep me", "enabledPlugins": {"existing@mp": true}}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	// Save a new plugin
	if err := SaveEnabledState(dir, "new-plugin@mp", true); err != nil {
		t.Fatalf("SaveEnabledState error: %v", err)
	}

	// Verify existing content preserved
	data, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	var settings map[string]json.RawMessage
	json.Unmarshal(data, &settings)

	if string(settings["customInstructions"]) != `"keep me"` {
		t.Errorf("customInstructions lost, got %s", settings["customInstructions"])
	}

	var ep map[string]json.RawMessage
	json.Unmarshal(settings["enabledPlugins"], &ep)

	if string(ep["existing@mp"]) != "true" {
		t.Errorf("existing plugin lost, got %s", ep["existing@mp"])
	}
	if string(ep["new-plugin@mp"]) != "true" {
		t.Errorf("new plugin not added, got %s", ep["new-plugin@mp"])
	}
}

// --- Semver constraint tests ---

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input                      string
		wantMaj, wantMin, wantPat int
		wantErr                    bool
	}{
		{"1.2.3", 1, 2, 3, false},
		{"0.0.1", 0, 0, 1, false},
		{"10.20.30", 10, 20, 30, false},
		{"v1.2.3", 1, 2, 3, false},
		{"1.0", 1, 0, 0, false},
		{"1", 1, 0, 0, false},
		{"1.2.3-beta", 1, 2, 3, false},
		{"abc", 0, 0, 0, true},
		{"1.abc.3", 0, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			maj, min, pat, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if maj != tt.wantMaj || min != tt.wantMin || pat != tt.wantPat {
					t.Errorf("ParseVersion(%q) = %d.%d.%d, want %d.%d.%d",
						tt.input, maj, min, pat, tt.wantMaj, tt.wantMin, tt.wantPat)
				}
			}
		})
	}
}

func TestSatisfiesConstraint(t *testing.T) {
	tests := []struct {
		version    string
		constraint string
		want       bool
	}{
		// Caret (^) -- same major, >= minor.patch
		{"1.3.0", "^1.2.0", true},
		{"1.2.5", "^1.2.0", true},
		{"1.2.0", "^1.2.0", true},
		{"2.0.0", "^1.2.0", false},
		{"0.9.0", "^1.2.0", false},
		// Caret with 0.x
		{"0.2.5", "^0.2.0", true},
		{"0.3.0", "^0.2.0", false},
		{"0.2.0", "^0.2.0", true},
		// Tilde (~) -- same major.minor, >= patch
		{"1.2.5", "~1.2.3", true},
		{"1.2.3", "~1.2.3", true},
		{"1.2.2", "~1.2.3", false},
		{"1.3.0", "~1.2.3", false},
		// Greater than or equal
		{"2.0.0", ">=1.0.0", true},
		{"1.0.0", ">=1.0.0", true},
		{"0.9.0", ">=1.0.0", false},
		// Less than or equal
		{"0.9.0", "<=1.0.0", true},
		{"1.0.0", "<=1.0.0", true},
		{"1.0.1", "<=1.0.0", false},
		// Greater than
		{"2.0.0", ">1.0.0", true},
		{"1.0.0", ">1.0.0", false},
		// Less than
		{"0.9.0", "<1.0.0", true},
		{"1.0.0", "<1.0.0", false},
		// Exact match
		{"1.2.3", "1.2.3", true},
		{"1.2.4", "1.2.3", false},
		// Wildcard
		{"1.0.0", "*", true},
		{"0.0.1", "", true},
	}

	for _, tt := range tests {
		name := tt.version + " " + tt.constraint
		t.Run(name, func(t *testing.T) {
			got := SatisfiesConstraint(tt.version, tt.constraint)
			if got != tt.want {
				t.Errorf("SatisfiesConstraint(%q, %q) = %v, want %v",
					tt.version, tt.constraint, got, tt.want)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.9.9", "2.0.0", -1},
		{"0.1.0", "0.0.9", 1},
	}

	for _, tt := range tests {
		t.Run(tt.a+" vs "+tt.b, func(t *testing.T) {
			got := CompareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// --- Marketplace tests ---

func TestMarketplaceListPlugins(t *testing.T) {
	mk1 := &KnownMarketplace{
		Manifest: &MarketplaceManifest{
			Name: "test-marketplace",
			Plugins: []MarketplacePlugin{
				{Name: "plugin-a", Version: "1.0.0"},
				{Name: "plugin-b", Version: "2.0.0"},
			},
		},
	}
	mk2 := &KnownMarketplace{
		Manifest: &MarketplaceManifest{
			Name: "other-marketplace",
			Plugins: []MarketplacePlugin{
				{Name: "plugin-c", Version: "0.1.0"},
			},
		},
	}

	all := ListPlugins([]*KnownMarketplace{mk1, mk2})
	if len(all) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(all))
	}
}

func TestMarketplaceFindPlugin(t *testing.T) {
	mk := &KnownMarketplace{
		Manifest: &MarketplaceManifest{
			Name: "test-marketplace",
			Plugins: []MarketplacePlugin{
				{Name: "my-plugin", Version: "1.0.0", Description: "found it"},
			},
		},
	}

	plugin, marketplace, found := FindPlugin([]*KnownMarketplace{mk}, "my-plugin")
	if !found {
		t.Fatal("expected to find plugin")
	}
	if plugin.Name != "my-plugin" {
		t.Errorf("expected 'my-plugin', got %q", plugin.Name)
	}
	if marketplace.Manifest.Name != "test-marketplace" {
		t.Errorf("expected 'test-marketplace', got %q", marketplace.Manifest.Name)
	}

	// Case-insensitive search
	_, _, found = FindPlugin([]*KnownMarketplace{mk}, "MY-PLUGIN")
	if !found {
		t.Error("expected case-insensitive find")
	}

	// Not found
	_, _, found = FindPlugin([]*KnownMarketplace{mk}, "nonexistent")
	if found {
		t.Error("expected not found")
	}
}

// --- UninstallPlugin test ---

func TestUninstallPlugin(t *testing.T) {
	cacheDir := t.TempDir()
	configDir := t.TempDir()

	// sanitizeDirName replaces / : and spaces but not @
	// so "test-plugin@test-marketplace" becomes "test-plugin@test-marketplace"
	pluginID := "test-plugin@test-marketplace"
	dirName := "test-plugin@test-marketplace" // sanitizeDirName preserves @
	pluginDir := filepath.Join(cacheDir, "plugins", dirName)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(`{"name":"test"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create initial settings with plugin enabled
	initial := `{"enabledPlugins": {"test-plugin@test-marketplace": true}}`
	if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	// Uninstall
	if err := UninstallPlugin(pluginID, cacheDir, configDir); err != nil {
		t.Fatalf("UninstallPlugin error: %v", err)
	}

	// Verify cache dir removed
	if _, err := os.Stat(pluginDir); !os.IsNotExist(err) {
		t.Error("expected plugin cache directory to be removed")
	}

	// Verify disabled in settings
	data, _ := os.ReadFile(filepath.Join(configDir, "settings.json"))
	var settings map[string]json.RawMessage
	json.Unmarshal(data, &settings)
	var ep map[string]json.RawMessage
	json.Unmarshal(settings["enabledPlugins"], &ep)

	if string(ep[pluginID]) != "false" {
		t.Errorf("expected plugin disabled in settings, got %s", ep[pluginID])
	}
}

// --- LoadAll with policy enforcement test ---

func TestLoadAll_PolicyEnforcement(t *testing.T) {
	r := NewRegistry()

	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "allowed-builtin",
		Description:    "Always allowed",
		DefaultEnabled: true,
	})

	// Policy with strict known marketplaces
	policy := &PluginPolicy{
		StrictKnownMarketplaces: []MarketplaceSource{
			{Source: "settings", Name: "approved-marketplace"},
		},
	}

	result := r.LoadAll(context.Background(), nil, nil, "", LoadAllOptions{
		Policy: policy,
	})

	// Built-in plugins should still be enabled (bypass policy)
	if len(result.Enabled) != 1 {
		t.Errorf("expected 1 enabled (builtin), got %d", len(result.Enabled))
	}
}

// --- LoadAll dependency check integration test ---

func TestLoadAll_DependencyErrors(t *testing.T) {
	r := NewRegistry()

	// Register a builtin plugin with an unsatisfied dependency
	r.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "dep-checker",
		Description:    "Has missing dependency",
		DefaultEnabled: true,
	})

	result := r.LoadAll(context.Background(), nil, nil, "")

	// Now set the dependency on the loaded plugin and re-check
	r.mu.Lock()
	for _, p := range r.plugins {
		if p.Name == "dep-checker" {
			p.Manifest.Dependencies = []string{"nonexistent-dep"}
		}
	}
	r.mu.Unlock()

	// Run LoadAll again to trigger dependency resolution
	r2 := NewRegistry()
	r2.RegisterBuiltin(&BuiltinPluginDefinition{
		Name:           "dep-checker",
		Description:    "Has missing dependency",
		DefaultEnabled: true,
	})
	// We need to verify ResolveDependencies directly since LoadAll
	// sets the dependency from manifest (which we can't change before LoadAll)
	_ = result

	r2.LoadAll(context.Background(), nil, nil, "")

	// Set dependency after load and check via ResolveDependencies
	r2.mu.Lock()
	for _, p := range r2.plugins {
		if p.Name == "dep-checker" {
			p.Manifest.Dependencies = []string{"nonexistent-dep"}
		}
	}
	r2.mu.Unlock()

	pluginDC, ok := r2.Get("dep-checker@builtin")
	if !ok {
		t.Fatal("dep-checker not found")
	}

	missing := r2.ResolveDependencies(pluginDC)
	if len(missing) != 1 || missing[0] != "nonexistent-dep" {
		t.Errorf("expected [nonexistent-dep], got %v", missing)
	}
}

// --- EnforcePluginPolicy test ---

func TestEnforcePluginPolicy_BuiltinAlwaysAllowed(t *testing.T) {
	policy := &PluginPolicy{
		BlockedMarketplaces: []MarketplaceSource{
			{Source: "settings", Name: "builtin"},
		},
	}

	plugin := &LoadedPlugin{
		Name:      "test",
		Source:    "test@builtin",
		IsBuiltin: true,
	}

	err := EnforcePluginPolicy(policy, "enable", plugin)
	if err != nil {
		t.Errorf("expected builtin plugin to be allowed, got: %v", err)
	}
}

// --- Marketplace manifest loading test (settings source) ---

func TestLoadMarketplace_SettingsSource(t *testing.T) {
	cacheDir := t.TempDir()

	// Create a marketplace manifest file
	manifestDir := filepath.Join(cacheDir, "marketplaces", "local-market")
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		t.Fatal(err)
	}

	manifest := MarketplaceManifest{
		Name: "local-market",
		Plugins: []MarketplacePlugin{
			{Name: "local-plugin", Version: "1.0.0", Description: "A local plugin"},
		},
	}
	data, _ := json.Marshal(manifest)
	manifestPath := filepath.Join(manifestDir, "marketplace.json")
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	source := MarketplaceSource{
		Source: "settings",
		Name:   "local-market",
		Path:   manifestPath,
	}

	mk, err := LoadMarketplace(context.Background(), source, cacheDir)
	if err != nil {
		t.Fatalf("LoadMarketplace error: %v", err)
	}

	if mk.Manifest == nil {
		t.Fatal("expected manifest to be loaded")
	}
	if mk.Manifest.Name != "local-market" {
		t.Errorf("expected name 'local-market', got %q", mk.Manifest.Name)
	}
	if len(mk.Manifest.Plugins) != 1 {
		t.Errorf("expected 1 plugin, got %d", len(mk.Manifest.Plugins))
	}
}
