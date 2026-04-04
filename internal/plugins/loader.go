package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/hooks"
	"github.com/khaledmoayad/clawgo/internal/skills"
)

// DefaultPluginCacheDir returns the default directory for cached plugin
// installations: ~/.claude/plugins/
func DefaultPluginCacheDir(configDir string) string {
	return filepath.Join(configDir, "plugins")
}

// InstallPlugin installs a plugin from the given source into cacheDir.
// Sources can be:
//   - "github:owner/repo" -- cloned via HTTPS from GitHub
//   - "https://..." or "git@..." -- cloned directly
//
// If the plugin is already cached with a valid manifest, the cached copy
// is returned. Returns the local installation path.
func InstallPlugin(ctx context.Context, source, cacheDir string) (string, error) {
	gitURL, dirName := parseSource(source)
	if gitURL == "" {
		return "", fmt.Errorf("unsupported plugin source: %s", source)
	}

	installPath := filepath.Join(cacheDir, "plugins", dirName)

	// Check cache: if directory exists with a valid manifest, use it
	if manifestPath, err := FindManifest(installPath); err == nil && manifestPath != "" {
		return installPath, nil
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(installPath), 0o755); err != nil {
		return "", fmt.Errorf("failed to create plugin cache directory: %w", err)
	}

	// Clone the repository
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", gitURL, installPath)
	cmd.Stderr = nil // suppress stderr output
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git clone failed for %s: %w", source, err)
	}

	return installPath, nil
}

// UpdatePlugin updates a previously installed plugin by doing a git pull.
func UpdatePlugin(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", path, "pull", "--ff-only")
	cmd.Stderr = nil
	cmd.Stdout = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed for %s: %w", path, err)
	}
	return nil
}

// LoadPluginFromPath loads a plugin from a local directory by finding
// and parsing its manifest, loading skills, and parsing hooks.
func LoadPluginFromPath(path, source string) (*LoadedPlugin, error) {
	manifestPath, err := FindManifest(path)
	if err != nil {
		return nil, err
	}

	manifest, err := ParseManifestFile(manifestPath)
	if err != nil {
		return nil, err
	}

	plugin := &LoadedPlugin{
		Name:       manifest.Name,
		Manifest:   manifest,
		Path:       path,
		Source:     source,
		Repository: source,
		McpServers: manifest.McpServers,
	}

	// Load skills from declared skill directories
	if len(manifest.Skills) > 0 {
		for _, skillDir := range manifest.Skills {
			absDir := filepath.Join(path, skillDir)
			loaded, err := skills.LoadSkillsFromDir(absDir, "plugin")
			if err != nil {
				continue // skip missing skill directories
			}
			plugin.Skills = append(plugin.Skills, loaded...)
		}
	} else {
		// Default: check for a skills/ directory in the plugin root
		defaultSkillDir := filepath.Join(path, "skills")
		if info, err := os.Stat(defaultSkillDir); err == nil && info.IsDir() {
			loaded, loadErr := skills.LoadSkillsFromDir(defaultSkillDir, "plugin")
			if loadErr == nil {
				plugin.Skills = append(plugin.Skills, loaded...)
			}
		}
	}

	// Parse hooks from manifest
	if len(manifest.Hooks) > 0 {
		var hooksConfig hooks.HooksConfig
		if err := json.Unmarshal(manifest.Hooks, &hooksConfig); err == nil {
			plugin.HooksConfig = hooksConfig
		}
		// Hooks parse errors are non-fatal; plugin still loads without hooks
	}

	// Read git SHA if this is a git repo
	plugin.SHA = readGitSHA(path)

	return plugin, nil
}

// LoadPlugins orchestrates the full plugin loading pipeline: for each
// configured repository, install/cache it, load it, and split into
// enabled/disabled based on the enabledPlugins map.
func LoadPlugins(ctx context.Context, config *PluginConfig, enabledPlugins map[string]bool, cacheDir string) *PluginLoadResult {
	result := &PluginLoadResult{}

	if config == nil || len(config.Repositories) == 0 {
		return result
	}

	for id, repo := range config.Repositories {
		source := repo.URL
		if source == "" {
			source = id
		}

		installPath, err := InstallPlugin(ctx, source, cacheDir)
		if err != nil {
			result.Errors = append(result.Errors, PluginError{
				Type:    ErrGeneric,
				Source:  source,
				Plugin:  id,
				Message: err.Error(),
			})
			continue
		}

		plugin, err := LoadPluginFromPath(installPath, source)
		if err != nil {
			result.Errors = append(result.Errors, PluginError{
				Type:    ErrManifestParse,
				Source:  source,
				Plugin:  id,
				Message: err.Error(),
			})
			continue
		}

		// Determine enabled state from user settings
		enabled, explicit := enabledPlugins[id]
		if explicit {
			plugin.Enabled = enabled
		} else {
			// Default: enabled
			plugin.Enabled = true
		}

		if plugin.Enabled {
			result.Enabled = append(result.Enabled, plugin)
		} else {
			result.Disabled = append(result.Disabled, plugin)
		}
	}

	return result
}

// parseSource normalizes a plugin source string into a git URL and
// a sanitized directory name for caching.
func parseSource(source string) (gitURL, dirName string) {
	switch {
	case strings.HasPrefix(source, "github:"):
		// "github:owner/repo" -> "https://github.com/owner/repo.git"
		repoPath := strings.TrimPrefix(source, "github:")
		gitURL = "https://github.com/" + repoPath + ".git"
		dirName = sanitizeDirName(repoPath)

	case strings.HasPrefix(source, "https://"):
		gitURL = source
		// Extract a directory name from the URL path
		path := strings.TrimPrefix(source, "https://")
		path = strings.TrimSuffix(path, ".git")
		dirName = sanitizeDirName(path)

	case strings.HasPrefix(source, "git@"):
		gitURL = source
		// "git@github.com:owner/repo.git" -> "github.com/owner/repo"
		path := strings.TrimPrefix(source, "git@")
		path = strings.ReplaceAll(path, ":", "/")
		path = strings.TrimSuffix(path, ".git")
		dirName = sanitizeDirName(path)

	default:
		return "", ""
	}
	return
}

// sanitizeDirName replaces path-unsafe characters for use as a cache
// directory name.
func sanitizeDirName(name string) string {
	name = strings.ReplaceAll(name, "/", "__")
	name = strings.ReplaceAll(name, ":", "__")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

// readGitSHA reads the HEAD commit SHA from a git repository directory.
func readGitSHA(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
