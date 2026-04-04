package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ManifestFileNames lists the filenames searched for in order of preference
// when looking for a plugin manifest. The first one found wins.
var ManifestFileNames = []string{
	"claude-plugin.json",
	"plugin.json",
	"manifest.json",
}

// FindManifest searches a directory for a plugin manifest file.
// It tries claude-plugin.json, plugin.json, and manifest.json in order.
// Returns the full path to the first manifest found, or an error if none exists.
func FindManifest(dir string) (string, error) {
	for _, name := range ManifestFileNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Fallback: check if package.json has a "claude-plugin" key
	pkgPath := filepath.Join(dir, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		var raw map[string]json.RawMessage
		if json.Unmarshal(data, &raw) == nil {
			if _, ok := raw["claude-plugin"]; ok {
				return pkgPath, nil
			}
		}
	}

	return "", fmt.Errorf("no plugin manifest found in %s", dir)
}

// ParseManifest parses JSON data into a PluginManifest.
// If the data comes from a package.json, it extracts the "claude-plugin" section.
func ParseManifest(data []byte) (*PluginManifest, error) {
	// Check if this is a package.json with a "claude-plugin" section first.
	// This must come before direct parse because package.json also has a
	// top-level "name" that would satisfy a direct unmarshal.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		if section, ok := raw["claude-plugin"]; ok {
			var manifest PluginManifest
			if err2 := json.Unmarshal(section, &manifest); err2 != nil {
				return nil, fmt.Errorf("failed to parse claude-plugin section: %w", err2)
			}
			// If name is missing, try to get it from the package name
			if manifest.Name == "" {
				if nameRaw, ok := raw["name"]; ok {
					var name string
					if json.Unmarshal(nameRaw, &name) == nil {
						manifest.Name = name
					}
				}
			}
			return &manifest, ValidateManifest(&manifest)
		}
	}

	// Direct manifest parse
	var manifest PluginManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse plugin manifest: %w", err)
	}

	if err := ValidateManifest(&manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// ValidateManifest checks that a parsed manifest has all required fields.
func ValidateManifest(m *PluginManifest) error {
	if m.Name == "" {
		return errors.New("plugin manifest: name is required")
	}
	return nil
}

// ParseManifestFile reads and parses a manifest file from the given path.
func ParseManifestFile(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	return ParseManifest(data)
}
