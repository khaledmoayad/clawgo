// Package plugins -- marketplace discovery, manifest loading, and plugin search.
//
// A marketplace is a JSON manifest listing available plugins. Manifests can be
// loaded from local settings files, GitHub raw URLs, git clones, or plain HTTP
// URLs. Loaded manifests are cached locally for offline use.
package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MarketplaceSource identifies where to fetch a marketplace manifest.
type MarketplaceSource struct {
	Source string `json:"source"` // "settings", "github", "git", "url"
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
	Owner  string `json:"owner,omitempty"`
	Repo   string `json:"repo,omitempty"`
	Branch string `json:"branch,omitempty"`
	Path   string `json:"path,omitempty"` // subdirectory or file path
}

// MarketplacePlugin describes a plugin available in a marketplace.
type MarketplacePlugin struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Version     string   `json:"version"`
	Source      string   `json:"source"` // git URL or local path
	Keywords    []string `json:"keywords,omitempty"`
	Author      string   `json:"author,omitempty"`
	License     string   `json:"license,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
}

// MarketplaceManifest is the top-level structure of a marketplace.json file.
type MarketplaceManifest struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Plugins     []MarketplacePlugin `json:"plugins"`
	Version     string              `json:"version,omitempty"`
	LastUpdated string              `json:"lastUpdated,omitempty"`
}

// KnownMarketplace tracks a registered marketplace with its cached manifest.
type KnownMarketplace struct {
	Source          MarketplaceSource
	Manifest        *MarketplaceManifest
	InstallLocation string
	LastUpdated     string
	AutoUpdate      bool
}

// LoadMarketplace fetches and parses a marketplace manifest from a source.
// The manifest is cached locally in cacheDir under marketplaces/{source-name}.json.
func LoadMarketplace(ctx context.Context, source MarketplaceSource, cacheDir string) (*KnownMarketplace, error) {
	var data []byte
	var err error

	switch source.Source {
	case "settings":
		data, err = loadFromSettings(source, cacheDir)
	case "github":
		data, err = loadFromGitHub(ctx, source)
	case "git":
		data, err = loadFromGit(ctx, source, cacheDir)
	case "url":
		data, err = loadFromURL(ctx, source)
	default:
		return nil, fmt.Errorf("unsupported marketplace source type: %q", source.Source)
	}

	if err != nil {
		// Try the cache as fallback
		cached, cacheErr := readCachedManifest(source.Name, cacheDir)
		if cacheErr == nil {
			return &KnownMarketplace{
				Source:          source,
				Manifest:        cached,
				InstallLocation: cacheDir,
				LastUpdated:     time.Now().UTC().Format(time.RFC3339),
			}, nil
		}
		return nil, fmt.Errorf("failed to load marketplace %q (%s): %w", source.Name, source.Source, err)
	}

	var manifest MarketplaceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse marketplace manifest for %q: %w", source.Name, err)
	}

	// Cache the manifest for offline use
	_ = writeCachedManifest(source.Name, cacheDir, data)

	return &KnownMarketplace{
		Source:          source,
		Manifest:        &manifest,
		InstallLocation: cacheDir,
		LastUpdated:     time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// ListPlugins returns all plugins available across all loaded marketplaces.
func ListPlugins(marketplaces []*KnownMarketplace) []MarketplacePlugin {
	var all []MarketplacePlugin
	for _, m := range marketplaces {
		if m.Manifest != nil {
			all = append(all, m.Manifest.Plugins...)
		}
	}
	return all
}

// FindPlugin searches marketplaces for a plugin by name.
// Returns the plugin, its marketplace, and whether it was found.
func FindPlugin(marketplaces []*KnownMarketplace, name string) (*MarketplacePlugin, *KnownMarketplace, bool) {
	name = strings.ToLower(name)
	for _, m := range marketplaces {
		if m.Manifest == nil {
			continue
		}
		for i := range m.Manifest.Plugins {
			if strings.ToLower(m.Manifest.Plugins[i].Name) == name {
				return &m.Manifest.Plugins[i], m, true
			}
		}
	}
	return nil, nil, false
}

// loadFromSettings reads a marketplace manifest from a local file in the
// plugin cache directory.
func loadFromSettings(source MarketplaceSource, cacheDir string) ([]byte, error) {
	path := source.Path
	if path == "" {
		// Default: look for marketplace.json in the cache dir
		path = filepath.Join(cacheDir, "marketplaces", source.Name, "marketplace.json")
	}
	return os.ReadFile(path)
}

// loadFromGitHub fetches a marketplace manifest from GitHub raw content.
func loadFromGitHub(ctx context.Context, source MarketplaceSource) ([]byte, error) {
	owner := source.Owner
	repo := source.Repo
	branch := source.Branch
	if branch == "" {
		branch = "main"
	}

	if owner == "" || repo == "" {
		// Try to extract from Repo field if it's in "owner/repo" format
		if parts := strings.SplitN(source.Repo, "/", 2); len(parts) == 2 {
			owner = parts[0]
			repo = parts[1]
		} else {
			return nil, fmt.Errorf("github source requires owner and repo")
		}
	}

	manifestPath := "marketplace.json"
	if source.Path != "" {
		manifestPath = source.Path
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, manifestPath)
	return httpGet(ctx, url)
}

// loadFromGit clones a git repo and reads the marketplace manifest.
func loadFromGit(ctx context.Context, source MarketplaceSource, cacheDir string) ([]byte, error) {
	if source.URL == "" {
		return nil, fmt.Errorf("git source requires url")
	}

	cloneDir := filepath.Join(cacheDir, "marketplaces", sanitizeDirName(source.Name))

	// If already cloned, pull instead
	if _, err := os.Stat(filepath.Join(cloneDir, ".git")); err == nil {
		cmd := exec.CommandContext(ctx, "git", "-C", cloneDir, "pull", "--ff-only")
		cmd.Stdout = nil
		cmd.Stderr = nil
		_ = cmd.Run() // Best effort update
	} else {
		if err := os.MkdirAll(filepath.Dir(cloneDir), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create marketplace cache dir: %w", err)
		}

		args := []string{"clone", "--depth", "1"}
		if source.Branch != "" {
			args = append(args, "--branch", source.Branch)
		}
		args = append(args, source.URL, cloneDir)

		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("git clone failed for marketplace %q: %w", source.Name, err)
		}
	}

	manifestPath := filepath.Join(cloneDir, "marketplace.json")
	if source.Path != "" {
		manifestPath = filepath.Join(cloneDir, source.Path)
	}

	return os.ReadFile(manifestPath)
}

// loadFromURL fetches a marketplace manifest from an HTTP(S) URL.
func loadFromURL(ctx context.Context, source MarketplaceSource) ([]byte, error) {
	if source.URL == "" {
		return nil, fmt.Errorf("url source requires url")
	}
	return httpGet(ctx, source.URL)
}

// httpGet performs an HTTP GET with context and timeout.
func httpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	// Limit read to 10MB to prevent abuse
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return data, nil
}

// readCachedManifest reads a previously cached marketplace manifest.
func readCachedManifest(name, cacheDir string) (*MarketplaceManifest, error) {
	path := filepath.Join(cacheDir, "marketplaces", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest MarketplaceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

// writeCachedManifest stores a marketplace manifest in the cache directory.
func writeCachedManifest(name, cacheDir string, data []byte) error {
	dir := filepath.Join(cacheDir, "marketplaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".json"), data, 0o644)
}
