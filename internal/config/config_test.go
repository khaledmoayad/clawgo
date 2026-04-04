package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigDir_Default(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	dir := ConfigDir()
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".claude"), dir)
}

func TestConfigDir_Override(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/tmp/test-claude-config")
	dir := ConfigDir()
	assert.Equal(t, "/tmp/test-claude-config", dir)
}

func TestResolveAPIKey_EnvVar(t *testing.T) {
	t.Setenv(EnvAPIKey, "sk-ant-test-key-123")
	t.Setenv(EnvAuthToken, "")
	cfg := &Config{}
	key := ResolveAPIKey(cfg)
	assert.Equal(t, "sk-ant-test-key-123", key)
}

func TestResolveAPIKey_AuthToken(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvAuthToken, "oauth-token-456")
	cfg := &Config{}
	key := ResolveAPIKey(cfg)
	assert.Equal(t, "oauth-token-456", key)
}

func TestResolveAPIKey_Config(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvAuthToken, "")
	cfg := &Config{PrimaryAPIKey: "config-key-789"}
	key := ResolveAPIKey(cfg)
	assert.Equal(t, "config-key-789", key)
}

func TestResolveAPIKey_CredentialsFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvAuthToken, "")

	// Write a credentials file
	credsPath := filepath.Join(tmpDir, ".credentials.json")
	creds := map[string]string{"apiKey": "file-key-abc"}
	data, err := json.Marshal(creds)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(credsPath, data, 0644))

	cfg := &Config{}
	key := ResolveAPIKey(cfg)
	assert.Equal(t, "file-key-abc", key)
}

func TestResolveAPIKey_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)
	t.Setenv(EnvAPIKey, "")
	t.Setenv(EnvAuthToken, "")
	cfg := &Config{}
	key := ResolveAPIKey(cfg)
	assert.Equal(t, "", key)
}

func TestLoadConfig_Missing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)
	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, false, cfg.HasCompletedOnboarding)
}

func TestLoadConfig_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", tmpDir)
	configPath := filepath.Join(tmpDir, ".config.json")
	data := `{"hasCompletedOnboarding": true, "primaryApiKey": "test-key"}`
	require.NoError(t, os.WriteFile(configPath, []byte(data), 0644))

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.True(t, cfg.HasCompletedOnboarding)
	assert.Equal(t, "test-key", cfg.PrimaryAPIKey)
}

func TestSettingsMerge(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := t.TempDir()

	// Create user settings
	userSettingsDir := tmpDir
	userSettings := Settings{
		Model:          "claude-3-sonnet",
		PermissionMode: "default",
		AllowedTools:   []string{"Read", "Write"},
	}
	data, err := json.Marshal(userSettings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(userSettingsDir, "settings.json"), data, 0644))

	// Create project settings that override
	projectSettingsDir := filepath.Join(projectDir, ".claude")
	require.NoError(t, os.MkdirAll(projectSettingsDir, 0755))
	projectSettings := Settings{
		Model:        "claude-3-opus",
		AllowedTools: []string{"Bash"},
	}
	data, err = json.Marshal(projectSettings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(projectSettingsDir, "settings.json"), data, 0644))

	merged, err := LoadSettings(tmpDir, projectDir)
	require.NoError(t, err)

	// Project model overrides user model
	assert.Equal(t, "claude-3-opus", merged.Model)
	// User permission mode preserved (project didn't set it)
	assert.Equal(t, "default", merged.PermissionMode)
	// Allowed tools are appended
	assert.Contains(t, merged.AllowedTools, "Read")
	assert.Contains(t, merged.AllowedTools, "Write")
	assert.Contains(t, merged.AllowedTools, "Bash")
}

func TestEnv(t *testing.T) {
	t.Setenv("TEST_CLAWGO_VAR", "hello")
	assert.Equal(t, "hello", Env("TEST_CLAWGO_VAR"))
	assert.Equal(t, "", Env("NONEXISTENT_VAR_XYZ"))
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"0", false},
		{"false", false},
		{"", false},
		{"no", false},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("TEST_BOOL_VAR", tt.value)
			assert.Equal(t, tt.expected, EnvBool("TEST_BOOL_VAR"))
		})
	}
}
