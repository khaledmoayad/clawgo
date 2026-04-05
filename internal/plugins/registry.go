package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// LoadAllOptions holds optional parameters for LoadAll.
type LoadAllOptions struct {
	// Policy is the enterprise plugin policy. Nil means no policy enforcement.
	Policy *PluginPolicy

	// MarketplaceSources lists marketplace sources to load plugins from.
	// These are loaded in addition to repository-based plugins.
	MarketplaceSources []MarketplaceSource
}

// LoadAll installs and loads all configured plugins from repositories,
// merges them with built-in plugins, and stores the result in the registry.
// The enabledPlugins map (from settings) controls which plugins are active.
func (r *Registry) LoadAll(ctx context.Context, config *PluginConfig, enabledPlugins map[string]bool, cacheDir string, opts ...LoadAllOptions) *PluginLoadResult {
	result := LoadPlugins(ctx, config, enabledPlugins, cacheDir)

	// Extract options if provided
	var opt LoadAllOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	// Load marketplace-sourced plugins
	if len(opt.MarketplaceSources) > 0 {
		for _, source := range opt.MarketplaceSources {
			// Check policy before loading marketplace
			if opt.Policy != nil {
				if IsMarketplaceBlocked(opt.Policy, source) {
					continue
				}
				if !IsMarketplaceAllowed(opt.Policy, source) {
					continue
				}
			}

			mk, err := LoadMarketplace(ctx, source, cacheDir)
			if err != nil {
				result.Errors = append(result.Errors, PluginError{
					Type:    ErrNetworkError,
					Source:  source.Name,
					Message: fmt.Sprintf("failed to load marketplace %q: %v", source.Name, err),
				})
				continue
			}

			if mk.Manifest != nil {
				for _, mp := range mk.Manifest.Plugins {
					pluginID := mp.Name + "@" + source.Name

					// Check if already loaded from repositories
					alreadyLoaded := false
					for _, p := range result.Enabled {
						if p.Name == mp.Name {
							alreadyLoaded = true
							break
						}
					}
					for _, p := range result.Disabled {
						if p.Name == mp.Name {
							alreadyLoaded = true
							break
						}
					}
					if alreadyLoaded {
						continue
					}

					// Create a LoadedPlugin from marketplace entry
					plugin := &LoadedPlugin{
						Name: mp.Name,
						Manifest: &PluginManifest{
							Name:        mp.Name,
							Description: mp.Description,
							Version:     mp.Version,
							Homepage:    mp.Homepage,
							License:     mp.License,
							Keywords:    mp.Keywords,
						},
						Source:     pluginID,
						Repository: mp.Source,
					}

					// Determine enabled state
					enabled, explicit := enabledPlugins[pluginID]
					if explicit {
						plugin.Enabled = enabled
					}
					// Marketplace plugins default to disabled until explicitly installed

					if plugin.Enabled {
						result.Enabled = append(result.Enabled, plugin)
					} else {
						result.Disabled = append(result.Disabled, plugin)
					}
				}
			}
		}
	}

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

	// Enforce policy on all non-builtin plugins
	if opt.Policy != nil {
		var stillEnabled []*LoadedPlugin
		for _, p := range result.Enabled {
			if err := EnforcePluginPolicy(opt.Policy, "enable", p); err != nil {
				p.Enabled = false
				result.Disabled = append(result.Disabled, p)
				result.Errors = append(result.Errors, PluginError{
					Type:    ErrGeneric,
					Source:  p.Source,
					Plugin:  p.Name,
					Message: err.Error(),
				})
			} else {
				stillEnabled = append(stillEnabled, p)
			}
		}
		result.Enabled = stillEnabled
	}

	// Check dependencies after all plugins are loaded
	for _, p := range result.Enabled {
		missing := r.resolveDepsUnlocked(p)
		for _, dep := range missing {
			result.Errors = append(result.Errors, PluginError{
				Type:    ErrGeneric,
				Source:  p.Source,
				Plugin:  p.Name,
				Message: fmt.Sprintf("missing dependency: %s", dep),
			})
		}
	}

	return result
}

// resolveDepsUnlocked checks dependencies without acquiring the lock.
// Must be called while holding r.mu (read or write).
func (r *Registry) resolveDepsUnlocked(plugin *LoadedPlugin) []string {
	if plugin == nil || plugin.Manifest == nil || len(plugin.Manifest.Dependencies) == 0 {
		return nil
	}

	var missing []string
	for _, dep := range plugin.Manifest.Dependencies {
		found := false
		for _, p := range r.plugins {
			if !p.Enabled {
				continue
			}
			if p.Name == dep || p.Source == dep {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, dep)
		}
	}
	return missing
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

// ResolveDependencies checks that all plugin dependencies are met.
// Returns a list of missing dependency identifiers. An empty list means
// all dependencies are satisfied.
func (r *Registry) ResolveDependencies(plugin *LoadedPlugin) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if plugin == nil || plugin.Manifest == nil || len(plugin.Manifest.Dependencies) == 0 {
		return nil
	}

	var missing []string
	for _, dep := range plugin.Manifest.Dependencies {
		found := false
		for _, p := range r.plugins {
			if !p.Enabled {
				continue
			}
			// Match by plugin name (with or without @marketplace suffix)
			if p.Name == dep || p.Source == dep {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, dep)
		}
	}
	return missing
}

// SaveEnabledState persists the enabled state of a plugin to user settings.
// It reads the current settings.json, updates enabledPlugins[pluginID], and
// writes back atomically.
func SaveEnabledState(configDir, pluginID string, enabled bool) error {
	settingsPath := filepath.Join(configDir, "settings.json")

	// Read existing settings
	var settings map[string]json.RawMessage
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			settings = make(map[string]json.RawMessage)
		} else {
			return fmt.Errorf("failed to read settings: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("failed to parse settings: %w", err)
		}
	}

	// Parse or create enabledPlugins map
	enabledPlugins := make(map[string]json.RawMessage)
	if raw, ok := settings["enabledPlugins"]; ok {
		if err := json.Unmarshal(raw, &enabledPlugins); err != nil {
			// If parsing fails, start fresh
			enabledPlugins = make(map[string]json.RawMessage)
		}
	}

	// Update the plugin state
	val, _ := json.Marshal(enabled)
	enabledPlugins[pluginID] = val

	// Marshal back
	epRaw, err := json.Marshal(enabledPlugins)
	if err != nil {
		return fmt.Errorf("failed to marshal enabledPlugins: %w", err)
	}
	settings["enabledPlugins"] = epRaw

	// Write settings atomically
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(settingsPath, out, 0o644)
}

// UninstallPlugin removes a plugin from the cache and disables it in settings.
func UninstallPlugin(pluginID, cacheDir, configDir string) error {
	// Remove plugin cache directory
	// Plugin ID might be "name@marketplace" or a git source
	dirName := sanitizeDirName(pluginID)
	pluginCacheDir := filepath.Join(cacheDir, "plugins", dirName)
	if _, err := os.Stat(pluginCacheDir); err == nil {
		if err := os.RemoveAll(pluginCacheDir); err != nil {
			return fmt.Errorf("failed to remove plugin cache: %w", err)
		}
	}

	// Remove from enabledPlugins in settings
	return SaveEnabledState(configDir, pluginID, false)
}
