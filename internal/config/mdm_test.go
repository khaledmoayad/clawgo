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

func TestLoadMDMLinuxWithDropIn(t *testing.T) {
	t.Run("merges base and drop-in files", func(t *testing.T) {
		// Create temp dirs for both base and drop-in
		tmpDir := t.TempDir()
		basePath := filepath.Join(tmpDir, "managed-settings.json")
		dropInDir := filepath.Join(tmpDir, "managed-settings.d")
		require.NoError(t, os.MkdirAll(dropInDir, 0755))

		// Base settings: model and permission
		baseJSON := `{"model": "base-model", "permissionMode": "default"}`
		require.NoError(t, os.WriteFile(basePath, []byte(baseJSON), 0644))

		// Drop-in 01: overrides model
		drop1 := `{"model": "dropin-model"}`
		require.NoError(t, os.WriteFile(filepath.Join(dropInDir, "01-org.json"), []byte(drop1), 0644))

		// Drop-in 02: adds custom instructions (later file alphabetically)
		drop2 := `{"customInstructions": "team rules"}`
		require.NoError(t, os.WriteFile(filepath.Join(dropInDir, "02-team.json"), []byte(drop2), 0644))

		s := loadMDMLinuxFromPaths(basePath, dropInDir)
		assert.Equal(t, "dropin-model", s.Model, "drop-in should override base model")
		assert.Equal(t, "default", s.PermissionMode, "base permission preserved")
		assert.Equal(t, "team rules", s.CustomInstructions, "drop-in 02 adds instructions")
	})

	t.Run("later drop-in alphabetically wins on conflict", func(t *testing.T) {
		tmpDir := t.TempDir()
		basePath := filepath.Join(tmpDir, "managed-settings.json")
		dropInDir := filepath.Join(tmpDir, "managed-settings.d")
		require.NoError(t, os.MkdirAll(dropInDir, 0755))

		require.NoError(t, os.WriteFile(basePath, []byte(`{}`), 0644))

		// Two drop-ins that set model differently
		require.NoError(t, os.WriteFile(
			filepath.Join(dropInDir, "01-first.json"),
			[]byte(`{"model": "first-model"}`), 0644))
		require.NoError(t, os.WriteFile(
			filepath.Join(dropInDir, "02-second.json"),
			[]byte(`{"model": "second-model"}`), 0644))

		s := loadMDMLinuxFromPaths(basePath, dropInDir)
		assert.Equal(t, "second-model", s.Model, "later alphabetically should win")
	})

	t.Run("invalid JSON in drop-in is skipped", func(t *testing.T) {
		tmpDir := t.TempDir()
		basePath := filepath.Join(tmpDir, "managed-settings.json")
		dropInDir := filepath.Join(tmpDir, "managed-settings.d")
		require.NoError(t, os.MkdirAll(dropInDir, 0755))

		require.NoError(t, os.WriteFile(basePath, []byte(`{"model": "base"}`), 0644))

		// Invalid JSON file
		require.NoError(t, os.WriteFile(
			filepath.Join(dropInDir, "01-bad.json"),
			[]byte(`not valid json`), 0644))

		// Valid file after bad one
		require.NoError(t, os.WriteFile(
			filepath.Join(dropInDir, "02-good.json"),
			[]byte(`{"permissionMode": "plan"}`), 0644))

		s := loadMDMLinuxFromPaths(basePath, dropInDir)
		assert.Equal(t, "base", s.Model, "base model preserved")
		assert.Equal(t, "plan", s.PermissionMode, "valid drop-in still applied")
	})

	t.Run("missing base file uses drop-ins only", func(t *testing.T) {
		tmpDir := t.TempDir()
		basePath := filepath.Join(tmpDir, "nonexistent.json")
		dropInDir := filepath.Join(tmpDir, "managed-settings.d")
		require.NoError(t, os.MkdirAll(dropInDir, 0755))

		require.NoError(t, os.WriteFile(
			filepath.Join(dropInDir, "01-only.json"),
			[]byte(`{"model": "dropin-only"}`), 0644))

		s := loadMDMLinuxFromPaths(basePath, dropInDir)
		assert.Equal(t, "dropin-only", s.Model)
	})

	t.Run("missing drop-in dir uses base only", func(t *testing.T) {
		tmpDir := t.TempDir()
		basePath := filepath.Join(tmpDir, "managed-settings.json")
		require.NoError(t, os.WriteFile(basePath, []byte(`{"model": "base-only"}`), 0644))

		s := loadMDMLinuxFromPaths(basePath, filepath.Join(tmpDir, "nonexistent-dir"))
		assert.Equal(t, "base-only", s.Model)
	})
}

func TestLoadMDMWindowsHKCUFallback(t *testing.T) {
	t.Run("HKLM result used when available", func(t *testing.T) {
		hklmJSON := `{"model": "hklm-model"}`
		hkcuJSON := `{"model": "hkcu-model"}`
		s := loadMDMWindowsFromValues(hklmJSON, hkcuJSON)
		assert.Equal(t, "hklm-model", s.Model, "HKLM should take priority over HKCU")
	})

	t.Run("HKCU used when HKLM is empty", func(t *testing.T) {
		hkcuJSON := `{"model": "hkcu-model"}`
		s := loadMDMWindowsFromValues("", hkcuJSON)
		assert.Equal(t, "hkcu-model", s.Model, "HKCU should be used as fallback")
	})

	t.Run("empty when both are empty", func(t *testing.T) {
		s := loadMDMWindowsFromValues("", "")
		assert.Equal(t, "", s.Model)
	})
}

func TestMergeSettingsHelper(t *testing.T) {
	t.Run("non-empty fields in override replace base", func(t *testing.T) {
		base := &Settings{Model: "base", PermissionMode: "default"}
		override := &Settings{Model: "override"}
		mergeSettings(base, override)
		assert.Equal(t, "override", base.Model)
		assert.Equal(t, "default", base.PermissionMode)
	})

	t.Run("empty fields in override do not replace base", func(t *testing.T) {
		base := &Settings{Model: "base"}
		override := &Settings{}
		mergeSettings(base, override)
		assert.Equal(t, "base", base.Model)
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
