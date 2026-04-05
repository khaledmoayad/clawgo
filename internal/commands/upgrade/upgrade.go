// Package upgrade implements the /upgrade slash command.
package upgrade

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/commands"
)

const (
	// githubReleasesURL is the GitHub API endpoint for latest release.
	githubReleasesURL = "https://api.github.com/repos/khaledmoayad/clawgo/releases/latest"
	// httpTimeout is the timeout for HTTP requests.
	httpTimeout = 5 * time.Second
)

// UpgradeCommand checks for ClawGo upgrades and provides update instructions.
type UpgradeCommand struct{}

// New creates a new UpgradeCommand.
func New() *UpgradeCommand { return &UpgradeCommand{} }

func (c *UpgradeCommand) Name() string              { return "upgrade" }
func (c *UpgradeCommand) Description() string        { return "Check for ClawGo upgrades" }
func (c *UpgradeCommand) Aliases() []string          { return []string{"update"} }
func (c *UpgradeCommand) Type() commands.CommandType { return commands.CommandTypeLocal }

func (c *UpgradeCommand) Execute(args string, ctx *commands.CommandContext) (*commands.CommandResult, error) {
	currentVersion := ctx.Version
	if currentVersion == "" {
		currentVersion = "dev"
	}

	// Try to fetch latest version from GitHub releases
	latestVersion, downloadURL, err := fetchLatestRelease()
	if err != nil {
		// If we can't reach the API, show current version with manual instructions
		text := fmt.Sprintf("Current version: %s\n\nCould not check for updates: %s\n\nTo update manually:\n  go install github.com/khaledmoayad/clawgo@latest",
			currentVersion, err.Error())
		return &commands.CommandResult{Type: "text", Value: text}, nil
	}

	// Normalize versions for comparison (strip leading 'v')
	current := strings.TrimPrefix(currentVersion, "v")
	latest := strings.TrimPrefix(latestVersion, "v")

	if current == latest || currentVersion == latestVersion {
		text := fmt.Sprintf("Already running latest version (%s)", currentVersion)
		return &commands.CommandResult{Type: "text", Value: text}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Current version: %s\n", currentVersion))
	sb.WriteString(fmt.Sprintf("Latest version:  %s\n\n", latestVersion))
	sb.WriteString("Update available! To upgrade:\n")
	sb.WriteString("  go install github.com/khaledmoayad/clawgo@latest\n")
	if downloadURL != "" {
		sb.WriteString(fmt.Sprintf("\nOr download the binary from:\n  %s", downloadURL))
	}

	return &commands.CommandResult{Type: "text", Value: sb.String()}, nil
}

// githubRelease represents the minimal fields from a GitHub release response.
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// fetchLatestRelease queries GitHub for the latest release version.
func fetchLatestRelease() (version string, url string, err error) {
	client := &http.Client{Timeout: httpTimeout}

	req, err := http.NewRequest("GET", githubReleasesURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "clawgo-upgrade-check")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", fmt.Errorf("failed to parse release info: %w", err)
	}

	if release.TagName == "" {
		return "", "", fmt.Errorf("no release tag found")
	}

	return release.TagName, release.HTMLURL, nil
}
