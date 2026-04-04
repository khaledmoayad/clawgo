package plugins

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/khaledmoayad/clawgo/internal/hooks"
	"github.com/khaledmoayad/clawgo/internal/skills"
)

// BuiltinMarketplaceName is the sentinel marketplace name for built-in plugins.
const BuiltinMarketplaceName = "builtin"

// BuiltinPluginDefinition describes a plugin that ships with the CLI binary.
// Built-in plugins appear in the /plugin UI and can be enabled/disabled by
// users (persisted to user settings). Plugin IDs use "{name}@builtin".
type BuiltinPluginDefinition struct {
	// Name is used in the "{name}@builtin" identifier.
	Name string

	// Description is shown in the /plugin UI.
	Description string

	// Version is an optional semver string.
	Version string

	// Skills provided by this plugin.
	Skills []*skills.Skill

	// Hooks provided by this plugin.
	Hooks hooks.HooksConfig

	// McpServers provided by this plugin.
	McpServers map[string]json.RawMessage

	// IsAvailable returns whether this plugin is available on the current
	// system. Nil means always available. Unavailable plugins are hidden.
	IsAvailable func() bool

	// DefaultEnabled is the enabled state before the user sets a preference.
	// True if not specified.
	DefaultEnabled bool
}

// Registry manages all loaded plugins (user-installed and built-in) and
// provides thread-safe access to plugin state, merged hooks, and skills.
type Registry struct {
	mu       sync.RWMutex
	plugins  map[string]*LoadedPlugin
	builtins map[string]*BuiltinPluginDefinition
}

// NewRegistry creates a new empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins:  make(map[string]*LoadedPlugin),
		builtins: make(map[string]*BuiltinPluginDefinition),
	}
}

// RegisterBuiltin registers a built-in plugin definition. Call this at
// startup before LoadAll.
func (r *Registry) RegisterBuiltin(def *BuiltinPluginDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.builtins[def.Name] = def
}

// LoadAll installs and loads all configured plugins from repositories,
// merges them with built-in plugins, and stores the result in the registry.
// The enabledPlugins map (from settings) controls which plugins are active.
func (r *Registry) LoadAll(ctx context.Context, config *PluginConfig, enabledPlugins map[string]bool, cacheDir string) *PluginLoadResult {
	result := LoadPlugins(ctx, config, enabledPlugins, cacheDir)

	r.mu.Lock()
	defer r.mu.Unlock()

	// Store loaded plugins
	for _, p := range result.Enabled {
		r.plugins[p.Name] = p
	}
	for _, p := range result.Disabled {
		r.plugins[p.Name] = p
	}

	// Merge builtin plugins
	for name, def := range r.builtins {
		if def.IsAvailable != nil && !def.IsAvailable() {
			continue
		}

		pluginID := name + "@" + BuiltinMarketplaceName

		// Determine enabled state
		userSetting, explicit := enabledPlugins[pluginID]
		enabled := true
		if explicit {
			enabled = userSetting
		} else {
			enabled = def.DefaultEnabled
		}

		plugin := &LoadedPlugin{
			Name: name,
			Manifest: &PluginManifest{
				Name:        name,
				Description: def.Description,
				Version:     def.Version,
			},
			Path:        BuiltinMarketplaceName,
			Source:      pluginID,
			Repository:  pluginID,
			Enabled:     enabled,
			IsBuiltin:   true,
			HooksConfig: def.Hooks,
			McpServers:  def.McpServers,
			Skills:      def.Skills,
		}

		r.plugins[pluginID] = plugin

		if enabled {
			result.Enabled = append(result.Enabled, plugin)
		} else {
			result.Disabled = append(result.Disabled, plugin)
		}
	}

	return result
}

// GetEnabled returns all enabled plugins (user-installed and built-in).
func (r *Registry) GetEnabled() []*LoadedPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []*LoadedPlugin
	for _, p := range r.plugins {
		if p.Enabled {
			out = append(out, p)
		}
	}
	return out
}

// GetDisabled returns all disabled plugins.
func (r *Registry) GetDisabled() []*LoadedPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []*LoadedPlugin
	for _, p := range r.plugins {
		if !p.Enabled {
			out = append(out, p)
		}
	}
	return out
}

// GetAll returns all loaded plugins regardless of enabled state.
func (r *Registry) GetAll() []*LoadedPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*LoadedPlugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p)
	}
	return out
}

// Get looks up a plugin by name or plugin ID.
func (r *Registry) Get(name string) (*LoadedPlugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.plugins[name]
	return p, ok
}

// SetEnabled updates the enabled state of a plugin by name or ID.
func (r *Registry) SetEnabled(pluginID string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if p, ok := r.plugins[pluginID]; ok {
		p.Enabled = enabled
	}
}

// GetMergedHooks collects hooks from all enabled plugins and merges them
// into a single HooksConfig. Matchers for each event are appended in order.
func (r *Registry) GetMergedHooks() hooks.HooksConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	merged := make(hooks.HooksConfig)
	for _, p := range r.plugins {
		if !p.Enabled || p.HooksConfig == nil {
			continue
		}
		for event, matchers := range p.HooksConfig {
			merged[event] = append(merged[event], matchers...)
		}
	}
	return merged
}

// GetMergedSkills collects skills from all enabled plugins.
func (r *Registry) GetMergedSkills() []*skills.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var all []*skills.Skill
	for _, p := range r.plugins {
		if !p.Enabled {
			continue
		}
		all = append(all, p.Skills...)
	}
	return all
}
