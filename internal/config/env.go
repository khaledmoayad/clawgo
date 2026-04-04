package config

import (
	"os"
	"strings"
)

// Environment variable names matching the TypeScript version.
const (
	EnvAPIKey          = "ANTHROPIC_API_KEY"
	EnvAuthToken       = "ANTHROPIC_AUTH_TOKEN"
	EnvBaseURL         = "ANTHROPIC_BASE_URL"
	EnvAPIBaseURL      = "CLAUDE_CODE_API_BASE_URL"
	EnvModel           = "ANTHROPIC_MODEL"
	EnvSmallModel      = "ANTHROPIC_SMALL_FAST_MODEL"
	EnvConfigDir       = "CLAUDE_CONFIG_DIR"
	EnvUseBedrock      = "CLAUDE_CODE_USE_BEDROCK"
	EnvUseVertex       = "CLAUDE_CODE_USE_VERTEX"
	EnvUseFoundry      = "CLAUDE_CODE_USE_FOUNDRY"
	EnvDisableThinking = "CLAUDE_CODE_DISABLE_THINKING"
	EnvSimple          = "CLAUDE_CODE_SIMPLE"
)

// Env returns the value of the named environment variable.
func Env(name string) string {
	return os.Getenv(name)
}

// EnvBool returns true if the named environment variable is "1" or "true" (case-insensitive).
func EnvBool(name string) bool {
	val := strings.ToLower(os.Getenv(name))
	return val == "1" || val == "true"
}
