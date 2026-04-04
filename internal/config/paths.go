package config

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the Claude config directory.
// Checks CLAUDE_CONFIG_DIR env var first, defaults to ~/.claude.
func ConfigDir() string {
	if dir := os.Getenv(EnvConfigDir); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to a reasonable default if home dir can't be determined
		return filepath.Join("/tmp", ".claude")
	}
	return filepath.Join(home, ".claude")
}

// ProjectConfigDir returns the .claude/ directory relative to the project root.
func ProjectConfigDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".claude")
}

// CredentialsPath returns the path to the credentials file.
func CredentialsPath() string {
	return filepath.Join(ConfigDir(), ".credentials.json")
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() string {
	return filepath.Join(ConfigDir(), ".config.json")
}
