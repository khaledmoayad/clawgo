package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMDMJSON(t *testing.T) {
	t.Run("valid settings JSON", func(t *testing.T) {
		data := []byte(`{"model": "claude-opus-4-20250514", "permissionMode": "plan"}`)
		s, err := parseMDMJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "claude-opus-4-20250514", s.Model)
		assert.Equal(t, "plan", s.PermissionMode)
	})

	t.Run("empty JSON", func(t *testing.T) {
		data := []byte(`{}`)
		s, err := parseMDMJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "", s.Model)
	})

	t.Run("settings with env and tools", func(t *testing.T) {
		data := []byte(`{
			"model": "claude-sonnet-4-20250514",
			"allowedTools": ["bash", "read"],
			"disallowedTools": ["write"],
			"env": {"FOO": "bar"}
		}`)
		s, err := parseMDMJSON(data)
		require.NoError(t, err)
		assert.Equal(t, "claude-sonnet-4-20250514", s.Model)
		assert.Equal(t, []string{"bash", "read"}, s.AllowedTools)
		assert.Equal(t, []string{"write"}, s.DisallowedTools)
		assert.Equal(t, "bar", s.Env["FOO"])
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		_, err := parseMDMJSON([]byte(`not json`))
		assert.Error(t, err)
	})
}

func TestParseRegQueryOutput(t *testing.T) {
	t.Run("standard reg query output", func(t *testing.T) {
		output := `
HKEY_LOCAL_MACHINE\SOFTWARE\Policies\Anthropic\ClaudeCode
    Settings    REG_SZ    {"model": "claude-opus-4-20250514"}
`
		result := parseRegQueryOutput(output)
		assert.Equal(t, `{"model": "claude-opus-4-20250514"}`, result)
	})

	t.Run("empty output", func(t *testing.T) {
		result := parseRegQueryOutput("")
		assert.Equal(t, "", result)
	})

	t.Run("no REG_SZ line", func(t *testing.T) {
		result := parseRegQueryOutput("some other output\nwithout registry data")
		assert.Equal(t, "", result)
	})
}

func TestLoadMDMSettingsPlatform(t *testing.T) {
	t.Run("unknown platform returns empty settings", func(t *testing.T) {
		s := loadMDMSettingsPlatform("freebsd")
		assert.NotNil(t, s)
		assert.Equal(t, "", s.Model)
	})
}

func TestGetConfigVersion(t *testing.T) {
	t.Run("version present as float64", func(t *testing.T) {
		data := map[string]interface{}{"configVersion": float64(3)}
		assert.Equal(t, 3, GetConfigVersion(data))
	})

	t.Run("version not present", func(t *testing.T) {
		data := map[string]interface{}{"foo": "bar"}
		assert.Equal(t, 0, GetConfigVersion(data))
	})

	t.Run("version is string (invalid)", func(t *testing.T) {
		data := map[string]interface{}{"configVersion": "two"}
		assert.Equal(t, 0, GetConfigVersion(data))
	})

	t.Run("empty map", func(t *testing.T) {
		assert.Equal(t, 0, GetConfigVersion(map[string]interface{}{}))
	})
}

func TestMigrateConfig(t *testing.T) {
	t.Run("no migrations needed", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		original := map[string]interface{}{
			"hasCompletedOnboarding": true,
		}
		data, _ := json.Marshal(original)
		require.NoError(t, os.WriteFile(configPath, data, 0644))

		err := MigrateConfig(configPath)
		require.NoError(t, err)

		// File should be unchanged since no migrations were applied
		after, _ := os.ReadFile(configPath)
		var result map[string]interface{}
		require.NoError(t, json.Unmarshal(after, &result))
		assert.Equal(t, true, result["hasCompletedOnboarding"])
		// No configVersion should be added since no migrations ran
		_, hasVersion := result["configVersion"]
		assert.False(t, hasVersion)
	})

	t.Run("missing file is no-op", func(t *testing.T) {
		err := MigrateConfig("/nonexistent/path/config.json")
		assert.NoError(t, err)
	})
}

func TestLoadSettingsFull(t *testing.T) {
	// Reset MDM cache before test to ensure clean state
	ResetMDMCache()

	t.Run("4-tier merge precedence", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		projectRoot := filepath.Join(tmpDir, "project")

		// Create directories
		require.NoError(t, os.MkdirAll(configDir, 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".claude"), 0755))

		// Tier 1: user settings -- sets model and custom instructions
		userSettings := Settings{
			Model:              "user-model",
			CustomInstructions: "user-instructions",
			PermissionMode:     "default",
		}
		writeSettingsJSON(t, filepath.Join(configDir, "settings.json"), userSettings)

		// Tier 2: project settings -- overrides model
		projectSettings := Settings{
			Model: "project-model",
		}
		writeSettingsJSON(t, filepath.Join(projectRoot, ".claude", "settings.json"), projectSettings)

		// Tier 3: MDM -- on Linux this reads /etc/claude-code/managed-settings.json
		// which doesn't exist in test env, so MDM returns empty (no override)
		// We test MDM JSON parsing separately above

		// Tier 4: remote settings -- overrides model again
		remoteSettings := &Settings{
			Model:          "remote-model",
			PermissionMode: "plan",
		}

		result, err := LoadSettingsFull(configDir, projectRoot, remoteSettings)
		require.NoError(t, err)

		// Remote model (tier 4) should win over project (tier 2) and user (tier 1)
		assert.Equal(t, "remote-model", result.Model)
		// Remote permission mode (tier 4) should win over user (tier 1)
		assert.Equal(t, "plan", result.PermissionMode)
		// Custom instructions from user (tier 1) preserved since no override
		assert.Equal(t, "user-instructions", result.CustomInstructions)
	})

	t.Run("nil remote settings skips tier 4", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		require.NoError(t, os.MkdirAll(configDir, 0755))

		userSettings := Settings{Model: "user-model"}
		writeSettingsJSON(t, filepath.Join(configDir, "settings.json"), userSettings)

		result, err := LoadSettingsFull(configDir, "", nil)
		require.NoError(t, err)
		assert.Equal(t, "user-model", result.Model)
	})

	t.Run("slice fields append across tiers", func(t *testing.T) {
		tmpDir := t.TempDir()
		configDir := filepath.Join(tmpDir, "config")
		projectRoot := filepath.Join(tmpDir, "project")

		require.NoError(t, os.MkdirAll(configDir, 0755))
		require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".claude"), 0755))

		userSettings := Settings{AllowedTools: []string{"bash"}}
		writeSettingsJSON(t, filepath.Join(configDir, "settings.json"), userSettings)

		projectSettings := Settings{AllowedTools: []string{"read"}}
		writeSettingsJSON(t, filepath.Join(projectRoot, ".claude", "settings.json"), projectSettings)

		remoteSettings := &Settings{AllowedTools: []string{"write"}}

		result, err := LoadSettingsFull(configDir, projectRoot, remoteSettings)
		require.NoError(t, err)
		// All slices should be appended
		assert.Contains(t, result.AllowedTools, "bash")
		assert.Contains(t, result.AllowedTools, "read")
		assert.Contains(t, result.AllowedTools, "write")
	})
}

// writeSettingsJSON is a test helper that writes a Settings struct as JSON.
func writeSettingsJSON(t *testing.T, path string, s Settings) {
	t.Helper()
	data, err := json.Marshal(s)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
}
