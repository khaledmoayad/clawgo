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
