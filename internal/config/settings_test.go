package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Test scalar override: later source wins ---

func TestSettingsScalarOverride(t *testing.T) {
	target := &Settings{
		Model:          "claude-3-sonnet",
		PermissionMode: "default",
		Language:        "english",
		EffortLevel:    "medium",
	}
	src := &Settings{
		Model:       "claude-3-opus",
		EffortLevel: "high",
	}

	mergeSettings(target, src)

	// Model overridden by src
	assert.Equal(t, "claude-3-opus", target.Model)
	// PermissionMode preserved (src didn't set it)
	assert.Equal(t, "default", target.PermissionMode)
	// Language preserved (src didn't set it)
	assert.Equal(t, "english", target.Language)
	// EffortLevel overridden by src
	assert.Equal(t, "high", target.EffortLevel)
}

// --- Test bool pointer override: later source wins ---

func TestSettingsBoolPtrOverride(t *testing.T) {
	trueVal := true
	falseVal := false

	target := &Settings{
		VimMode:           &trueVal,
		FastMode:          &falseVal,
		SpinnerTipsEnabled: nil,
	}
	src := &Settings{
		VimMode:            &falseVal,
		SpinnerTipsEnabled: &trueVal,
	}

	mergeSettings(target, src)

	// VimMode overridden to false
	require.NotNil(t, target.VimMode)
	assert.False(t, *target.VimMode)
	// FastMode preserved (src didn't set it)
	require.NotNil(t, target.FastMode)
	assert.False(t, *target.FastMode)
	// SpinnerTipsEnabled set by src
	require.NotNil(t, target.SpinnerTipsEnabled)
	assert.True(t, *target.SpinnerTipsEnabled)
}

// --- Test int pointer override ---

func TestSettingsIntPtrOverride(t *testing.T) {
	days := 30
	target := &Settings{
		CleanupPeriodDays: &days,
	}
	newDays := 7
	src := &Settings{
		CleanupPeriodDays: &newDays,
	}

	mergeSettings(target, src)

	require.NotNil(t, target.CleanupPeriodDays)
	assert.Equal(t, 7, *target.CleanupPeriodDays)
}

// --- Test float64 pointer override ---

func TestSettingsFloat64PtrOverride(t *testing.T) {
	rate := 0.05
	target := &Settings{
		FeedbackSurveyRate: &rate,
	}
	newRate := 0.10
	src := &Settings{
		FeedbackSurveyRate: &newRate,
	}

	mergeSettings(target, src)

	require.NotNil(t, target.FeedbackSurveyRate)
	assert.InDelta(t, 0.10, *target.FeedbackSurveyRate, 0.001)
}

// --- Test array append: AllowedTools from two sources are concatenated ---

func TestSettingsArrayAppend(t *testing.T) {
	target := &Settings{
		AllowedTools:    []string{"Read", "Write"},
		DisallowedTools: []string{"Bash"},
	}
	src := &Settings{
		AllowedTools:    []string{"Glob"},
		DisallowedTools: []string{"Edit"},
	}

	mergeSettings(target, src)

	// AllowedTools concatenated
	assert.Equal(t, []string{"Read", "Write", "Glob"}, target.AllowedTools)
	// DisallowedTools concatenated
	assert.Equal(t, []string{"Bash", "Edit"}, target.DisallowedTools)
}

func TestSettingsArrayAppendHTTPHookURLs(t *testing.T) {
	target := &Settings{
		AllowedHTTPHookURLs:    []string{"https://hooks.example.com/*"},
		HTTPHookAllowedEnvVars: []string{"API_KEY"},
	}
	src := &Settings{
		AllowedHTTPHookURLs:    []string{"https://other.example.com/*"},
		HTTPHookAllowedEnvVars: []string{"SECRET_TOKEN"},
	}

	mergeSettings(target, src)

	assert.Equal(t, []string{"https://hooks.example.com/*", "https://other.example.com/*"}, target.AllowedHTTPHookURLs)
	assert.Equal(t, []string{"API_KEY", "SECRET_TOKEN"}, target.HTTPHookAllowedEnvVars)
}

func TestSettingsArrayAppendCompanyAnnouncements(t *testing.T) {
	target := &Settings{
		CompanyAnnouncements: []string{"Welcome!"},
	}
	src := &Settings{
		CompanyAnnouncements: []string{"New feature available"},
	}

	mergeSettings(target, src)

	assert.Equal(t, []string{"Welcome!", "New feature available"}, target.CompanyAnnouncements)
}

func TestSettingsArrayAppendMCPServerNames(t *testing.T) {
	target := &Settings{
		AllowedMCPServerNames: []string{"server-a"},
		DeniedMCPServerNames:  []string{"blocked-1"},
	}
	src := &Settings{
		AllowedMCPServerNames: []string{"server-b"},
		DeniedMCPServerNames:  []string{"blocked-2"},
	}

	mergeSettings(target, src)

	assert.Equal(t, []string{"server-a", "server-b"}, target.AllowedMCPServerNames)
	assert.Equal(t, []string{"blocked-1", "blocked-2"}, target.DeniedMCPServerNames)
}

// --- Test map merge: Env keys from both sources present ---

func TestSettingsMapMerge(t *testing.T) {
	target := &Settings{
		Env: map[string]string{
			"FOO": "bar",
			"BAZ": "qux",
		},
		KeyBindings: map[string]string{
			"ctrl+s": "save",
		},
	}
	src := &Settings{
		Env: map[string]string{
			"BAZ": "overridden",
			"NEW": "value",
		},
		KeyBindings: map[string]string{
			"ctrl+q": "quit",
		},
	}

	mergeSettings(target, src)

	// Env: FOO preserved, BAZ overridden, NEW added
	assert.Equal(t, "bar", target.Env["FOO"])
	assert.Equal(t, "overridden", target.Env["BAZ"])
	assert.Equal(t, "value", target.Env["NEW"])

	// KeyBindings merged
	assert.Equal(t, "save", target.KeyBindings["ctrl+s"])
	assert.Equal(t, "quit", target.KeyBindings["ctrl+q"])
}

func TestSettingsMapMergeModelOverrides(t *testing.T) {
	target := &Settings{
		ModelOverrides: map[string]string{
			"claude-opus-4-6": "arn:aws:bedrock:us-east-1:123:model/opus",
		},
	}
	src := &Settings{
		ModelOverrides: map[string]string{
			"claude-sonnet-4-6": "arn:aws:bedrock:us-east-1:123:model/sonnet",
		},
	}

	mergeSettings(target, src)

	assert.Equal(t, "arn:aws:bedrock:us-east-1:123:model/opus", target.ModelOverrides["claude-opus-4-6"])
	assert.Equal(t, "arn:aws:bedrock:us-east-1:123:model/sonnet", target.ModelOverrides["claude-sonnet-4-6"])
}

func TestSettingsMapMergeEnabledPlugins(t *testing.T) {
	target := &Settings{
		EnabledPlugins: map[string]json.RawMessage{
			"plugin-a@marketplace": json.RawMessage(`true`),
		},
	}
	src := &Settings{
		EnabledPlugins: map[string]json.RawMessage{
			"plugin-b@marketplace": json.RawMessage(`["v1","v2"]`),
		},
	}

	mergeSettings(target, src)

	assert.Contains(t, target.EnabledPlugins, "plugin-a@marketplace")
	assert.Contains(t, target.EnabledPlugins, "plugin-b@marketplace")
	assert.Equal(t, json.RawMessage(`true`), target.EnabledPlugins["plugin-a@marketplace"])
	assert.Equal(t, json.RawMessage(`["v1","v2"]`), target.EnabledPlugins["plugin-b@marketplace"])
}

func TestSettingsMapMergeNilTarget(t *testing.T) {
	target := &Settings{}
	src := &Settings{
		Env: map[string]string{"KEY": "value"},
	}

	mergeSettings(target, src)

	require.NotNil(t, target.Env)
	assert.Equal(t, "value", target.Env["KEY"])
}

// --- Test json.RawMessage override: Hooks from later source replaces earlier ---

func TestSettingsRawJSONOverride(t *testing.T) {
	target := &Settings{
		Hooks: json.RawMessage(`{"preToolUse": {"command": "echo old"}}`),
	}
	src := &Settings{
		Hooks: json.RawMessage(`{"preToolUse": {"command": "echo new"}}`),
	}

	mergeSettings(target, src)

	// Hooks entirely replaced by src
	assert.JSONEq(t, `{"preToolUse": {"command": "echo new"}}`, string(target.Hooks))
}

func TestSettingsRawJSONOverrideMCPServers(t *testing.T) {
	target := &Settings{
		MCPServers: json.RawMessage(`{"server1": {}}`),
	}
	src := &Settings{
		MCPServers: json.RawMessage(`{"server2": {}}`),
	}

	mergeSettings(target, src)

	// MCPServers entirely replaced by src
	assert.JSONEq(t, `{"server2": {}}`, string(target.MCPServers))
}

func TestSettingsRawJSONOverrideSandbox(t *testing.T) {
	target := &Settings{
		Sandbox: json.RawMessage(`{"enabled": true}`),
	}
	src := &Settings{
		Sandbox: json.RawMessage(`{"enabled": false, "mode": "strict"}`),
	}

	mergeSettings(target, src)

	assert.JSONEq(t, `{"enabled": false, "mode": "strict"}`, string(target.Sandbox))
}

func TestSettingsRawJSONPreservedWhenSrcEmpty(t *testing.T) {
	target := &Settings{
		Hooks:   json.RawMessage(`{"some": "hooks"}`),
		Sandbox: json.RawMessage(`{"enabled": true}`),
	}
	src := &Settings{} // No raw JSON fields set

	mergeSettings(target, src)

	// Target preserved when src is empty
	assert.JSONEq(t, `{"some": "hooks"}`, string(target.Hooks))
	assert.JSONEq(t, `{"enabled": true}`, string(target.Sandbox))
}

// --- Test struct pointer override ---

func TestSettingsStructPtrOverride(t *testing.T) {
	target := &Settings{
		Attribution: &Attribution{Commit: "old-commit-msg"},
	}
	src := &Settings{
		Attribution: &Attribution{Commit: "new-commit-msg", PR: "new-pr-msg"},
	}

	mergeSettings(target, src)

	require.NotNil(t, target.Attribution)
	assert.Equal(t, "new-commit-msg", target.Attribution.Commit)
	assert.Equal(t, "new-pr-msg", target.Attribution.PR)
}

// --- Test .local settings.json loading ---

func TestSettingsLocalFileLoading(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()

	// Create user settings
	userSettings := Settings{
		Model:    "claude-3-sonnet",
		Language: "english",
	}
	writeTestSettingsJSON(t, filepath.Join(configDir, "settings.json"), &userSettings)

	// Create user local settings (overrides user)
	userLocalSettings := Settings{
		Model: "claude-3-opus",
	}
	writeTestSettingsJSON(t, filepath.Join(configDir, "settings.local.json"), &userLocalSettings)

	// Create project settings
	projectClaudeDir := filepath.Join(projectDir, ".claude")
	require.NoError(t, os.MkdirAll(projectClaudeDir, 0755))
	projectSettings := Settings{
		AllowedTools: []string{"Read"},
	}
	writeTestSettingsJSON(t, filepath.Join(projectClaudeDir, "settings.json"), &projectSettings)

	// Create project local settings (overrides project)
	projectLocalSettings := Settings{
		AllowedTools: []string{"Write"},
		Language:     "japanese",
	}
	writeTestSettingsJSON(t, filepath.Join(projectClaudeDir, "settings.local.json"), &projectLocalSettings)

	merged, err := LoadSettingsFull(configDir, projectDir, nil)
	require.NoError(t, err)

	// User local overrides user model
	assert.Equal(t, "claude-3-opus", merged.Model)
	// Project local overrides user's language
	assert.Equal(t, "japanese", merged.Language)
	// AllowedTools appended from both project sources
	assert.Contains(t, merged.AllowedTools, "Read")
	assert.Contains(t, merged.AllowedTools, "Write")
}

func TestSettingsLocalFileMissing(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()

	// Only create user settings (no .local files)
	userSettings := Settings{
		Model: "claude-3-sonnet",
	}
	writeTestSettingsJSON(t, filepath.Join(configDir, "settings.json"), &userSettings)

	merged, err := LoadSettingsFull(configDir, projectDir, nil)
	require.NoError(t, err)

	// Works fine without .local files
	assert.Equal(t, "claude-3-sonnet", merged.Model)
}

// --- Test managed-settings.d directory loading ---

func TestSettingsManagedSettingsDirLoading(t *testing.T) {
	// Create a temp dir to simulate managed-settings.d
	managedDir := t.TempDir()

	// We can't easily test ManagedSettingsDir() since it returns a fixed path,
	// but we can test loadManagedSettingsDir directly
	target := &Settings{
		Model: "base-model",
	}

	// Create multiple JSON files
	settings1 := Settings{
		AllowedTools: []string{"Read"},
	}
	writeTestSettingsJSON(t, filepath.Join(managedDir, "01-policy.json"), &settings1)

	settings2 := Settings{
		AllowedTools: []string{"Write"},
		Model:        "managed-model",
	}
	writeTestSettingsJSON(t, filepath.Join(managedDir, "02-override.json"), &settings2)

	// Simulate loading by reading files and merging
	entries, err := os.ReadDir(managedDir)
	require.NoError(t, err)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			err := mergeSettingsFromFile(target, filepath.Join(managedDir, entry.Name()))
			require.NoError(t, err)
		}
	}

	// Model overridden by second file
	assert.Equal(t, "managed-model", target.Model)
	// AllowedTools appended from both files
	assert.Contains(t, target.AllowedTools, "Read")
	assert.Contains(t, target.AllowedTools, "Write")
}

// --- Test full 7-tier precedence ---

func TestSettingsFull7TierPrecedence(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()

	// Tier 1: User settings (lowest priority)
	writeTestSettingsJSON(t, filepath.Join(configDir, "settings.json"), &Settings{
		Model:       "tier1-model",
		Language:    "tier1-lang",
		OutputStyle: "tier1-style",
	})

	// Tier 2: User local settings
	writeTestSettingsJSON(t, filepath.Join(configDir, "settings.local.json"), &Settings{
		Model: "tier2-model",
	})

	// Tier 3: Project settings
	projectClaudeDir := filepath.Join(projectDir, ".claude")
	require.NoError(t, os.MkdirAll(projectClaudeDir, 0755))
	writeTestSettingsJSON(t, filepath.Join(projectClaudeDir, "settings.json"), &Settings{
		Model:    "tier3-model",
		Language: "tier3-lang",
	})

	// Tier 4: Project local settings
	writeTestSettingsJSON(t, filepath.Join(projectClaudeDir, "settings.local.json"), &Settings{
		Model: "tier4-model",
	})

	// Tier 7: Remote-managed settings (highest)
	remoteSettings := &Settings{
		OutputStyle: "tier7-style",
	}

	merged, err := LoadSettingsFull(configDir, projectDir, remoteSettings)
	require.NoError(t, err)

	// Model: tier4 (project local) beats tier3 (project) beats tier2 (user local) beats tier1 (user)
	// But MDM (tier5) might override -- since we don't mock MDM, tier4 wins
	assert.Equal(t, "tier4-model", merged.Model)
	// Language: tier3 (project) beats tier1 (user)
	assert.Equal(t, "tier3-lang", merged.Language)
	// OutputStyle: tier7 (remote) beats tier1 (user)
	assert.Equal(t, "tier7-style", merged.OutputStyle)
}

// --- Test backward compatibility with LoadSettings (3-tier) ---

func TestSettingsLoadSettings3Tier(t *testing.T) {
	configDir := t.TempDir()
	projectDir := t.TempDir()

	writeTestSettingsJSON(t, filepath.Join(configDir, "settings.json"), &Settings{
		Model:        "user-model",
		AllowedTools: []string{"Read"},
	})

	projectClaudeDir := filepath.Join(projectDir, ".claude")
	require.NoError(t, os.MkdirAll(projectClaudeDir, 0755))
	writeTestSettingsJSON(t, filepath.Join(projectClaudeDir, "settings.json"), &Settings{
		Model:        "project-model",
		AllowedTools: []string{"Bash"},
	})

	merged, err := LoadSettings(configDir, projectDir)
	require.NoError(t, err)

	assert.Equal(t, "project-model", merged.Model)
	assert.Contains(t, merged.AllowedTools, "Read")
	assert.Contains(t, merged.AllowedTools, "Bash")
}

// --- Test empty/missing files are handled gracefully ---

func TestSettingsMergeFromFileMissing(t *testing.T) {
	target := &Settings{}
	err := mergeSettingsFromFile(target, "/nonexistent/path/settings.json")
	assert.NoError(t, err)
}

func TestSettingsMergeFromFileInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))

	target := &Settings{}
	err := mergeSettingsFromFile(target, path)
	assert.Error(t, err)
}

// --- Test complete merge with all field types ---

func TestSettingsCompleteMerge(t *testing.T) {
	trueVal := true
	days := 30
	rate := 0.05

	target := &Settings{}
	src := &Settings{
		// Scalars
		Model:       "claude-3-opus",
		Language:    "japanese",
		Agent:       "code-reviewer",
		EffortLevel: "high",

		// Bool pointers
		VimMode:            &trueVal,
		FastMode:           &trueVal,
		SpinnerTipsEnabled: &trueVal,

		// Int pointer
		CleanupPeriodDays: &days,

		// Float pointer
		FeedbackSurveyRate: &rate,

		// Arrays
		AllowedTools:         []string{"Read", "Write"},
		DisallowedTools:      []string{"Bash"},
		CompanyAnnouncements: []string{"Hello"},
		SSHConfigs:           []SSHConfig{{ID: "ssh1", Name: "dev", SSHHost: "dev.example.com"}},

		// Maps
		Env:            map[string]string{"MY_VAR": "value"},
		KeyBindings:    map[string]string{"ctrl+s": "save"},
		ModelOverrides: map[string]string{"opus": "custom-arn"},
		EnabledPlugins: map[string]json.RawMessage{"plugin@mp": json.RawMessage(`true`)},

		// Raw JSON
		Hooks:      json.RawMessage(`{"hooks": true}`),
		MCPServers: json.RawMessage(`{"server": {}}`),
		Sandbox:    json.RawMessage(`{"mode": "strict"}`),

		// Struct pointers
		Attribution:    &Attribution{Commit: "AI-assisted", PR: "Generated by Claude"},
		FileSuggestion: &FileSuggestion{Type: "command", Command: "find ."},
		WorktreeConfig: &Worktree{SymlinkDirectories: []string{"node_modules"}},
	}

	mergeSettings(target, src)

	// Verify all field types
	assert.Equal(t, "claude-3-opus", target.Model)
	assert.Equal(t, "japanese", target.Language)
	assert.True(t, *target.VimMode)
	assert.True(t, *target.FastMode)
	assert.Equal(t, 30, *target.CleanupPeriodDays)
	assert.InDelta(t, 0.05, *target.FeedbackSurveyRate, 0.001)
	assert.Equal(t, []string{"Read", "Write"}, target.AllowedTools)
	assert.Equal(t, "value", target.Env["MY_VAR"])
	assert.Equal(t, "save", target.KeyBindings["ctrl+s"])
	assert.JSONEq(t, `{"hooks": true}`, string(target.Hooks))
	assert.Equal(t, "AI-assisted", target.Attribution.Commit)
	assert.Len(t, target.SSHConfigs, 1)
	assert.Equal(t, "dev", target.SSHConfigs[0].Name)
}

// --- Helper ---

func writeTestSettingsJSON(t *testing.T, path string, s *Settings) {
	t.Helper()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
}
