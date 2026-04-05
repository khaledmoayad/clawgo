// Package plugins -- enterprise policy enforcement for plugin operations.
//
// PluginPolicy holds settings from policySettings (managed/enterprise)
// that control which marketplaces and customization surfaces are allowed.
// Policy checks run before any plugin install, enable, or marketplace load.
package plugins

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CustomizationSurface identifies a customization area that can be locked
// to plugin-only sources by enterprise policy.
type CustomizationSurface string

const (
	SurfaceSkills CustomizationSurface = "skills"
	SurfaceAgents CustomizationSurface = "agents"
	SurfaceHooks  CustomizationSurface = "hooks"
	SurfaceMCP    CustomizationSurface = "mcp"
)

// PluginPolicy holds enterprise policy settings affecting plugins.
// Fields are parsed from the policySettings source in settings.json.
type PluginPolicy struct {
	// StrictPluginOnlyCustomization can be:
	//   - true (bool): locks all four customization surfaces
	//   - []string: locks only the listed surfaces
	//   - nil/absent: nothing locked (default)
	StrictPluginOnlyCustomization json.RawMessage

	// StrictKnownMarketplaces is an allowlist of marketplace sources.
	// If non-nil, only listed marketplaces can be used.
	StrictKnownMarketplaces []MarketplaceSource

	// BlockedMarketplaces is a blocklist of marketplace sources.
	// Blocked marketplaces cannot be loaded or used.
	BlockedMarketplaces []MarketplaceSource
}

// ParsePluginPolicy extracts PluginPolicy from raw JSON settings fields.
func ParsePluginPolicy(
	strictPluginOnly json.RawMessage,
	strictKnown json.RawMessage,
	blocked json.RawMessage,
) (*PluginPolicy, error) {
	policy := &PluginPolicy{
		StrictPluginOnlyCustomization: strictPluginOnly,
	}

	if len(strictKnown) > 0 {
		if err := json.Unmarshal(strictKnown, &policy.StrictKnownMarketplaces); err != nil {
			return nil, fmt.Errorf("failed to parse strictKnownMarketplaces: %w", err)
		}
	}

	if len(blocked) > 0 {
		if err := json.Unmarshal(blocked, &policy.BlockedMarketplaces); err != nil {
			return nil, fmt.Errorf("failed to parse blockedMarketplaces: %w", err)
		}
	}

	return policy, nil
}

// EnforcePluginPolicy checks if a plugin operation is allowed by enterprise policy.
// Operations: "install", "enable", "disable", "uninstall".
// Returns an error describing the policy violation, or nil if allowed.
func EnforcePluginPolicy(policy *PluginPolicy, operation string, plugin *LoadedPlugin) error {
	if policy == nil || plugin == nil {
		return nil
	}

	// Built-in plugins are always allowed
	if plugin.IsBuiltin {
		return nil
	}

	// Check if the plugin's marketplace source is blocked
	if plugin.Source != "" {
		source := sourceFromPluginID(plugin.Source)
		if source != nil {
			if IsMarketplaceBlocked(policy, *source) {
				return fmt.Errorf("plugin %q is from a blocked marketplace source", plugin.Name)
			}
			if !IsMarketplaceAllowed(policy, *source) {
				return fmt.Errorf("plugin %q is from a marketplace not in the organization allowlist", plugin.Name)
			}
		}
	}

	return nil
}

// IsMarketplaceAllowed checks if a marketplace source is allowed by policy.
// Returns true if no policy restrictions exist or the source is in the allowlist.
func IsMarketplaceAllowed(policy *PluginPolicy, source MarketplaceSource) bool {
	if policy == nil {
		return true
	}

	// Check blocklist first
	if IsMarketplaceBlocked(policy, source) {
		return false
	}

	// If no allowlist, everything not blocked is allowed
	if len(policy.StrictKnownMarketplaces) == 0 {
		return true
	}

	// Check if the source matches any entry in the allowlist
	for _, allowed := range policy.StrictKnownMarketplaces {
		if marketplaceSourcesMatch(source, allowed) {
			return true
		}
	}

	return false
}

// IsMarketplaceBlocked checks if a marketplace source is blocked by policy.
func IsMarketplaceBlocked(policy *PluginPolicy, source MarketplaceSource) bool {
	if policy == nil || len(policy.BlockedMarketplaces) == 0 {
		return false
	}

	for _, blocked := range policy.BlockedMarketplaces {
		if marketplaceSourcesMatch(source, blocked) {
			return true
		}
	}

	return false
}

// IsPluginCustomizationLocked checks if a customization surface is locked to
// plugins only by enterprise policy.
// surface is one of: "skills", "agents", "hooks", "mcp"
func IsPluginCustomizationLocked(policy *PluginPolicy, surface string) bool {
	if policy == nil || len(policy.StrictPluginOnlyCustomization) == 0 {
		return false
	}

	// Try parsing as bool first
	var boolVal bool
	if err := json.Unmarshal(policy.StrictPluginOnlyCustomization, &boolVal); err == nil {
		return boolVal
	}

	// Try parsing as string array
	var surfaces []string
	if err := json.Unmarshal(policy.StrictPluginOnlyCustomization, &surfaces); err == nil {
		for _, s := range surfaces {
			if strings.EqualFold(s, surface) {
				return true
			}
		}
	}

	return false
}

// marketplaceSourcesMatch compares two marketplace sources for policy matching.
// Matching is by source type and relevant identifiers (URL, repo, name).
func marketplaceSourcesMatch(a, b MarketplaceSource) bool {
	if a.Source != b.Source {
		return false
	}

	switch a.Source {
	case "github":
		return strings.EqualFold(a.Owner, b.Owner) && strings.EqualFold(a.Repo, b.Repo)
	case "url":
		return a.URL == b.URL
	case "git":
		return a.URL == b.URL
	case "settings":
		return strings.EqualFold(a.Name, b.Name)
	default:
		return a.Name == b.Name
	}
}

// sourceFromPluginID attempts to extract a MarketplaceSource from a plugin
// source identifier. Returns nil if the source cannot be parsed into a
// marketplace source (e.g., it's just a name or git URL).
func sourceFromPluginID(pluginSource string) *MarketplaceSource {
	// Check for "name@marketplace" format
	if idx := strings.LastIndex(pluginSource, "@"); idx > 0 {
		marketplace := pluginSource[idx+1:]
		if marketplace == BuiltinMarketplaceName {
			return nil // Built-in plugins bypass policy
		}
		return &MarketplaceSource{
			Source: "settings",
			Name:   marketplace,
		}
	}

	// Check for GitHub source
	if strings.HasPrefix(pluginSource, "github:") {
		parts := strings.SplitN(strings.TrimPrefix(pluginSource, "github:"), "/", 2)
		if len(parts) == 2 {
			return &MarketplaceSource{
				Source: "github",
				Owner:  parts[0],
				Repo:   parts[1],
			}
		}
	}

	// Check for URL source
	if strings.HasPrefix(pluginSource, "https://") || strings.HasPrefix(pluginSource, "http://") {
		return &MarketplaceSource{
			Source: "url",
			URL:    pluginSource,
		}
	}

	return nil
}
