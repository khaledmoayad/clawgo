package config

import (
	"encoding/json"
	"os"
	"strings"
)

// Config is a type alias for GlobalConfig for backward compatibility.
// New code should use GlobalConfig directly.
type Config = GlobalConfig

// Credentials represents the credentials file at ~/.claude/.credentials.json.
type Credentials struct {
	APIKey string `json:"apiKey,omitempty"`
}

// LoadConfig reads the global config from ConfigDir()/.config.json.
// Returns a zero-value Config if the file does not exist.
func LoadConfig() (*Config, error) {
	path := GlobalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveConfig writes the config to ConfigDir()/.config.json.
func SaveConfig(cfg *Config) error {
	path := GlobalConfigPath()

	// Ensure directory exists
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// AuthResult holds the resolved authentication credentials.
type AuthResult struct {
	Token   string // The API key or OAuth access token
	IsOAuth bool   // True if this is a Claude.ai OAuth token (use as authToken, not apiKey)
}

// ResolveAuth resolves authentication from multiple sources in priority order.
// Returns an AuthResult indicating whether the token is an API key or OAuth token.
func ResolveAuth(cfg *Config) AuthResult {
	result := ResolveAPIKey(cfg)
	if result == "" {
		return AuthResult{}
	}
	// OAuth tokens from Claude.ai start with "sk-ant-oat"
	return AuthResult{
		Token:   result,
		IsOAuth: strings.HasPrefix(result, "sk-ant-oat"),
	}
}

// ResolveAPIKey resolves the API key from multiple sources in priority order:
// 1. ANTHROPIC_API_KEY env var
// 2. ANTHROPIC_AUTH_TOKEN env var
// 3. Config.PrimaryAPIKey
// 4. Credentials file (~/.claude/.credentials.json)
func ResolveAPIKey(cfg *Config) string {
	// 1. Check ANTHROPIC_API_KEY env var
	if key := Env(EnvAPIKey); key != "" {
		return key
	}

	// 2. Check ANTHROPIC_AUTH_TOKEN env var
	if token := Env(EnvAuthToken); token != "" {
		return token
	}

	// 3. Check config primary key
	if cfg != nil && cfg.PrimaryAPIKey != "" {
		return cfg.PrimaryAPIKey
	}

	// 4. Check credentials file (apiKey field)
	credsPath := CredentialsPath()
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return ""
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return ""
	}
	if creds.APIKey != "" {
		return creds.APIKey
	}

	// 5. Check claudeAiOauth in credentials file
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	if oauthData, ok := raw["claudeAiOauth"]; ok {
		var oauth struct {
			AccessToken string `json:"accessToken"`
		}
		if err := json.Unmarshal(oauthData, &oauth); err == nil && oauth.AccessToken != "" {
			return oauth.AccessToken
		}
	}

	return ""
}

const defaultAPIBaseURL = "https://api.anthropic.com"

// ResolveAPIBaseURL resolves the API base URL from multiple sources in priority order:
// 1. CLAUDE_CODE_API_BASE_URL env var
// 2. ANTHROPIC_BASE_URL env var
// 3. Default "https://api.anthropic.com"
func ResolveAPIBaseURL(cfg *Config) string {
	if url := Env(EnvAPIBaseURL); url != "" {
		return url
	}
	if url := Env(EnvBaseURL); url != "" {
		return url
	}
	return defaultAPIBaseURL
}
