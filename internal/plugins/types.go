// Package plugins implements plugin installation, management, and loading
// for ClawGo. Plugins extend the CLI with third-party skills, hooks, and
// MCP server configurations, installed from git repositories and managed
// through the registry.
package plugins

import (
	"encoding/json"

	"github.com/khaledmoayad/clawgo/internal/hooks"
	"github.com/khaledmoayad/clawgo/internal/skills"
)

// PluginAuthor holds optional metadata about the plugin creator.
type PluginAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// PluginManifest represents the parsed plugin.json or claude-plugin.json
// manifest file that describes a plugin's metadata and components.
type PluginManifest struct {
	// Name is the unique identifier for the plugin (kebab-case preferred).
	Name string `json:"name"`

	// Description is a brief, user-facing explanation of what the plugin provides.
	Description string `json:"description,omitempty"`

	// Version is a semantic version string (e.g., "1.2.3").
	Version string `json:"version,omitempty"`

	// Author holds optional creator/maintainer information.
	Author *PluginAuthor `json:"author,omitempty"`

	// Skills lists relative paths to skill directories within the plugin.
	Skills []string `json:"skills,omitempty"`

	// Hooks is the raw JSON hooks configuration. Parsed lazily to avoid
	// coupling manifest parsing to the hooks package schema.
	Hooks json.RawMessage `json:"hooks,omitempty"`

	// McpServers maps server names to their raw JSON configurations.
	McpServers map[string]json.RawMessage `json:"mcpServers,omitempty"`

	// Dependencies lists other plugin IDs that must be enabled for this
	// plugin to function.
	Dependencies []string `json:"dependencies,omitempty"`

	// Homepage is the plugin homepage or documentation URL.
	Homepage string `json:"homepage,omitempty"`

	// Repository is the source code repository URL.
	Repository string `json:"repository,omitempty"`

	// License is the SPDX license identifier.
	License string `json:"license,omitempty"`

	// Keywords are tags for plugin discovery and categorization.
	Keywords []string `json:"keywords,omitempty"`
}

// LoadedPlugin represents a fully loaded plugin with parsed components.
type LoadedPlugin struct {
	// Name is the plugin identifier (from manifest).
	Name string

	// Manifest holds the parsed manifest metadata.
	Manifest *PluginManifest

	// Path is the local filesystem path to the plugin directory.
	Path string

	// Source identifies where the plugin came from (e.g., "github:owner/repo").
	Source string

	// Repository is the repository identifier, usually same as Source.
	Repository string

	// Enabled indicates whether the plugin is active in the current session.
	Enabled bool

	// IsBuiltin is true for built-in plugins that ship with the CLI.
	IsBuiltin bool

	// SHA is the git commit hash for version pinning.
	SHA string

	// HooksConfig holds the parsed hooks from the manifest.
	HooksConfig hooks.HooksConfig

	// McpServers maps server names to raw JSON configurations.
	McpServers map[string]json.RawMessage

	// Skills holds loaded skill objects from this plugin.
	Skills []*skills.Skill
}

// PluginConfig represents the plugin configuration stored in settings.json.
// The "plugins" field maps repository identifiers to their installation metadata.
type PluginConfig struct {
	Repositories map[string]PluginRepository `json:"repositories"`
}

// PluginRepository holds installation metadata for a single plugin source.
type PluginRepository struct {
	URL         string `json:"url"`
	Branch      string `json:"branch,omitempty"`
	LastUpdated string `json:"lastUpdated,omitempty"`
	CommitSHA   string `json:"commitSha,omitempty"`
}

// PluginErrorType identifies the category of a plugin error.
type PluginErrorType string

const (
	ErrGeneric            PluginErrorType = "generic-error"
	ErrPluginNotFound     PluginErrorType = "plugin-not-found"
	ErrGitAuthFailed      PluginErrorType = "git-auth-failed"
	ErrGitTimeout         PluginErrorType = "git-timeout"
	ErrManifestParse      PluginErrorType = "manifest-parse-error"
	ErrManifestValidation PluginErrorType = "manifest-validation-error"
	ErrComponentLoad      PluginErrorType = "component-load-failed"
	ErrNetworkError       PluginErrorType = "network-error"
)

// PluginError represents a structured error that occurred during plugin
// loading or installation. The Type field enables discriminated handling.
type PluginError struct {
	Type    PluginErrorType `json:"type"`
	Source  string          `json:"source"`
	Plugin  string          `json:"plugin,omitempty"`
	Message string          `json:"message"`
}

// Error implements the error interface for PluginError.
func (e PluginError) Error() string {
	if e.Plugin != "" {
		return e.Plugin + ": " + e.Message
	}
	return e.Message
}

// PluginLoadResult captures the outcome of loading all configured plugins.
type PluginLoadResult struct {
	Enabled  []*LoadedPlugin
	Disabled []*LoadedPlugin
	Errors   []PluginError
}
