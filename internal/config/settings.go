package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings represents the settings from settings.json files.
// Matches the TypeScript Settings type.
type Settings struct {
	Model              string            `json:"model,omitempty"`
	PermissionMode     string            `json:"permissionMode,omitempty"`
	CustomInstructions string            `json:"customInstructions,omitempty"`
	AllowedTools       []string          `json:"allowedTools,omitempty"`
	DisallowedTools    []string          `json:"disallowedTools,omitempty"`
	Env                map[string]string `json:"env,omitempty"`
	KeyBindings        map[string]string `json:"keyBindings,omitempty"`
	VimMode            *bool             `json:"vimMode,omitempty"`

	// EnabledPlugins maps plugin IDs to their enabled state (true/false).
	// Plugin IDs use the format "name@marketplace" or "name@builtin".
	EnabledPlugins map[string]bool `json:"enabledPlugins,omitempty"`

	// Plugins holds the plugin repository configuration (repositories to install).
	// Stored as json.RawMessage to avoid circular imports with the plugins package.
	Plugins json.RawMessage `json:"plugins,omitempty"`

	// Hooks holds user-defined hooks configuration in the same format as
	// the hooks package HooksConfig type.
	Hooks json.RawMessage `json:"hooks,omitempty"`
}

// LoadSettings loads and merges settings from multiple sources in precedence order:
// 1. User: configDir/settings.json (lowest priority)
// 2. Project: projectRoot/.claude/settings.json
// 3. Enterprise/MDM: LoadMDMSettings() (highest priority in this variant)
//
// For full 4-tier precedence including remote-managed settings, use LoadSettingsFull.
// Scalar fields are overridden by higher-priority sources.
// Slice fields (AllowedTools, DisallowedTools) are appended.
// Map fields (Env) are merged.
func LoadSettings(configDir, projectRoot string) (*Settings, error) {
	return LoadSettingsFull(configDir, projectRoot, nil)
}

// LoadSettingsFull loads and merges settings from all 4 tiers in precedence order:
// 1. User: configDir/settings.json (lowest priority)
// 2. Project: projectRoot/.claude/settings.json
// 3. Enterprise/MDM: LoadMDMSettings()
// 4. Remote-managed: remoteSettings parameter (highest priority)
//
// Scalar fields are overridden by higher-priority sources.
// Slice fields (AllowedTools, DisallowedTools) are appended.
// Map fields (Env) are merged.
func LoadSettingsFull(configDir, projectRoot string, remoteSettings *Settings) (*Settings, error) {
	merged := &Settings{}

	// 1. Load user settings
	userPath := filepath.Join(configDir, "settings.json")
	if err := mergeSettingsFromFile(merged, userPath); err != nil {
		return nil, err
	}

	// 2. Load project settings
	if projectRoot != "" {
		projectPath := filepath.Join(projectRoot, ".claude", "settings.json")
		if err := mergeSettingsFromFile(merged, projectPath); err != nil {
			return nil, err
		}
	}

	// 3. Load enterprise/MDM settings
	mdmSettings := LoadMDMSettings()
	mergeSettings(merged, mdmSettings)

	// 4. Load remote-managed settings (highest priority)
	if remoteSettings != nil {
		mergeSettings(merged, remoteSettings)
	}

	return merged, nil
}

// mergeSettingsFromFile reads a settings file and merges it into the target.
// If the file does not exist, it is silently skipped.
func mergeSettingsFromFile(target *Settings, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var src Settings
	if err := json.Unmarshal(data, &src); err != nil {
		return err
	}

	mergeSettings(target, &src)
	return nil
}

// mergeSettings merges src into target. Scalar fields override,
// slice fields append, map fields merge.
func mergeSettings(target, src *Settings) {
	// Override scalar fields if set
	if src.Model != "" {
		target.Model = src.Model
	}
	if src.PermissionMode != "" {
		target.PermissionMode = src.PermissionMode
	}
	if src.CustomInstructions != "" {
		target.CustomInstructions = src.CustomInstructions
	}

	// Append slice fields
	target.AllowedTools = append(target.AllowedTools, src.AllowedTools...)
	target.DisallowedTools = append(target.DisallowedTools, src.DisallowedTools...)

	// Merge map fields
	if src.Env != nil {
		if target.Env == nil {
			target.Env = make(map[string]string)
		}
		for k, v := range src.Env {
			target.Env[k] = v
		}
	}
	if src.KeyBindings != nil {
		if target.KeyBindings == nil {
			target.KeyBindings = make(map[string]string)
		}
		for k, v := range src.KeyBindings {
			target.KeyBindings[k] = v
		}
	}

	// Override VimMode if explicitly set (pointer field)
	if src.VimMode != nil {
		target.VimMode = src.VimMode
	}

	// Merge EnabledPlugins map (higher priority overrides per-key)
	if src.EnabledPlugins != nil {
		if target.EnabledPlugins == nil {
			target.EnabledPlugins = make(map[string]bool)
		}
		for k, v := range src.EnabledPlugins {
			target.EnabledPlugins[k] = v
		}
	}

	// Override Plugins (raw JSON, higher priority wins entirely)
	if len(src.Plugins) > 0 {
		target.Plugins = src.Plugins
	}

	// Override Hooks (raw JSON, higher priority wins entirely)
	if len(src.Hooks) > 0 {
		target.Hooks = src.Hooks
	}
}
