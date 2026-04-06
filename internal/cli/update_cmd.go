package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	githubRepo = "khaledmoayad/clawgo"
	githubAPI  = "https://api.github.com"
)

// releaseURL is the GitHub releases API endpoint; overridable for testing.
var releaseURL = githubAPI + "/repos/" + githubRepo + "/releases/latest"

// releaseInfo represents a GitHub release API response.
type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

// releaseAsset represents a downloadable file attached to a release.
type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// newUpdateCmd creates the "update" subcommand for checking and installing updates.
func newUpdateCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"upgrade"},
		Short:   "Check for updates and install if available",
		Long:    "Check the current version against the latest release and install updates if available.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", Version)

			// Fetch latest release
			info, err := getLatestReleaseFromURL(ctx, releaseURL)
			if err != nil {
				return fmt.Errorf("failed to check for updates: %w", err)
			}

			latest := strings.TrimPrefix(info.TagName, "v")
			current := strings.TrimPrefix(Version, "v")

			if !isNewerVersion(info.TagName, Version) {
				fmt.Fprintln(cmd.OutOrStdout(), "Already at latest version.")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "New version available: %s -> %s\n", current, latest)

			// Find asset for current platform
			assetURL := getAssetURL(info.Assets)
			if assetURL == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "No binary available for %s/%s.\n", runtime.GOOS, runtime.GOARCH)
				fmt.Fprintf(cmd.OutOrStdout(), "Download manually from: https://github.com/%s/releases/latest\n", githubRepo)
				return nil
			}

			// Confirm unless --yes
			if !yes {
				fmt.Fprint(cmd.OutOrStdout(), "Download and install? [y/N] ")
				var answer string
				fmt.Fscanln(cmd.InOrStdin(), &answer)
				answer = strings.TrimSpace(strings.ToLower(answer))
				if answer != "y" && answer != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "Update cancelled.")
					return nil
				}
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Downloading update...")
			if err := downloadAndReplace(ctx, assetURL); err != nil {
				return fmt.Errorf("update failed: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully updated to %s\n", latest)
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")

	return cmd
}

// getLatestReleaseFromURL fetches release info from the given URL.
// This is the testable version that accepts a custom URL.
func getLatestReleaseFromURL(ctx context.Context, url string) (*releaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ClawGo/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var info releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}
	return &info, nil
}

// getLatestRelease fetches the latest release from the configured GitHub repo.
func getLatestRelease(ctx context.Context) (*releaseInfo, error) {
	return getLatestReleaseFromURL(ctx, releaseURL)
}

// isNewerVersion returns true if latest > current using semver comparison.
// Strips "v" prefix and handles non-semver "current" values (e.g., "dev")
// by treating them as older than any real version.
func isNewerVersion(latest, current string) bool {
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")

	// Strip any pre-release suffix for comparison (e.g., "1.0.1-beta" -> "1.0.1")
	latest = strings.SplitN(latest, "-", 2)[0]
	current = strings.SplitN(current, "-", 2)[0]

	latestParts := parseSemver(latest)
	currentParts := parseSemver(current)

	if latestParts == nil {
		return false
	}
	if currentParts == nil {
		// Current is non-semver (e.g., "dev", "") -- any real version is newer
		return true
	}

	for i := 0; i < 3; i++ {
		if latestParts[i] > currentParts[i] {
			return true
		}
		if latestParts[i] < currentParts[i] {
			return false
		}
	}
	return false
}

// parseSemver parses "major.minor.patch" into [3]int.
// Returns nil if the string is not valid semver.
func parseSemver(s string) []int {
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	result := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		result[i] = n
	}
	return result
}

// getAssetURL finds the download URL for the current platform binary.
// Asset naming convention: clawgo-{goos}-{goarch} (with .exe for Windows).
func getAssetURL(assets []releaseAsset) string {
	target := "clawgo-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	for _, a := range assets {
		if a.Name == target {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

// downloadAndReplace downloads the binary from url and replaces the current executable.
// Steps:
//  1. Download to a temp file in the same directory as the current binary
//  2. Verify download completed (size > 0)
//  3. Rename current binary to .old, rename temp to current
//  4. Remove .old
func downloadAndReplace(ctx context.Context, url string) error {
	// Find current binary path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("cannot resolve executable symlinks: %w", err)
	}

	dir := filepath.Dir(execPath)
	base := filepath.Base(execPath)

	// Download to temp file in same directory (ensures same filesystem for rename)
	tmpFile, err := os.CreateTemp(dir, base+".update-*")
	if err != nil {
		return fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up on error

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		tmpFile.Close()
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	n, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("download write failed: %w", err)
	}
	tmpFile.Close()

	if n == 0 {
		return fmt.Errorf("downloaded file is empty")
	}

	// Make executable
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("cannot set executable permission: %w", err)
	}

	// Atomic-ish replace: rename current to .old, rename temp to current
	oldPath := execPath + ".old"
	if err := os.Rename(execPath, oldPath); err != nil {
		return fmt.Errorf("cannot backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		// Try to restore the old binary
		os.Rename(oldPath, execPath)
		return fmt.Errorf("cannot install new binary: %w", err)
	}

	// Clean up old binary (best-effort)
	os.Remove(oldPath)

	return nil
}
