package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Version comparison tests
// ---------------------------------------------------------------------------

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name     string
		latest   string
		current  string
		expected bool
	}{
		{"newer patch", "1.0.1", "1.0.0", true},
		{"same version", "1.0.0", "1.0.0", false},
		{"older version", "1.0.0", "1.0.1", false},
		{"newer minor", "1.1.0", "1.0.0", true},
		{"newer major", "2.0.0", "1.0.0", true},
		{"older major", "1.0.0", "2.0.0", false},
		{"with v prefix latest", "v1.0.1", "1.0.0", true},
		{"with v prefix current", "1.0.1", "v1.0.0", true},
		{"with v prefix both", "v1.0.1", "v1.0.0", true},
		{"dev version", "1.0.0", "dev", true},
		{"empty current", "1.0.0", "", true},
		{"empty latest", "", "1.0.0", false},
		{"pre-release ignored", "1.0.1-beta", "1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNewerVersion(tt.latest, tt.current)
			assert.Equal(t, tt.expected, result, "isNewerVersion(%q, %q)", tt.latest, tt.current)
		})
	}
}

// ---------------------------------------------------------------------------
// getLatestRelease tests (mock HTTP)
// ---------------------------------------------------------------------------

func TestGetLatestRelease(t *testing.T) {
	release := releaseInfo{
		TagName: "v1.2.3",
		Assets: []releaseAsset{
			{Name: "clawgo-linux-amd64", BrowserDownloadURL: "https://example.com/clawgo-linux-amd64"},
			{Name: "clawgo-darwin-arm64", BrowserDownloadURL: "https://example.com/clawgo-darwin-arm64"},
		},
	}
	data, err := json.Marshal(release)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/khaledmoayad/clawgo/releases/latest", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer server.Close()

	info, err := getLatestReleaseFromURL(context.Background(), server.URL+"/repos/khaledmoayad/clawgo/releases/latest")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", info.TagName)
	assert.Len(t, info.Assets, 2)
}

func TestGetLatestReleaseHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := getLatestReleaseFromURL(context.Background(), server.URL+"/repos/khaledmoayad/clawgo/releases/latest")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// getAssetURL tests
// ---------------------------------------------------------------------------

func TestGetAssetURL(t *testing.T) {
	assets := []releaseAsset{
		{Name: "clawgo-linux-amd64", BrowserDownloadURL: "https://example.com/clawgo-linux-amd64"},
		{Name: "clawgo-darwin-arm64", BrowserDownloadURL: "https://example.com/clawgo-darwin-arm64"},
		{Name: "clawgo-windows-amd64.exe", BrowserDownloadURL: "https://example.com/clawgo-windows-amd64.exe"},
	}

	// Build the expected asset name for the current platform
	expectedName := "clawgo-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		expectedName += ".exe"
	}

	url := getAssetURL(assets)
	if url == "" {
		t.Skipf("no matching asset for %s/%s in test data", runtime.GOOS, runtime.GOARCH)
	}

	// The URL should correspond to our current platform
	found := false
	for _, a := range assets {
		if a.Name == expectedName && a.BrowserDownloadURL == url {
			found = true
			break
		}
	}
	assert.True(t, found, "getAssetURL should return URL for %s", expectedName)
}

func TestGetAssetURLNoMatch(t *testing.T) {
	assets := []releaseAsset{
		{Name: "clawgo-freebsd-riscv64", BrowserDownloadURL: "https://example.com/nope"},
	}

	// Unless we are running on freebsd/riscv64, this should return empty
	if runtime.GOOS == "freebsd" && runtime.GOARCH == "riscv64" {
		t.Skip("skipping -- running on freebsd/riscv64")
	}
	url := getAssetURL(assets)
	assert.Empty(t, url, "getAssetURL should return empty for non-matching platform")
}

// ---------------------------------------------------------------------------
// Update command integration test (with --yes flag)
// ---------------------------------------------------------------------------

func TestSelfUpdateCommandYesFlag(t *testing.T) {
	// Create a mock server that returns a release with no newer version
	release := releaseInfo{
		TagName: "v0.0.1", // older than any real version
		Assets:  []releaseAsset{},
	}
	data, err := json.Marshal(release)
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer server.Close()

	// Override the release URL for testing
	origURL := releaseURL
	releaseURL = server.URL + "/repos/khaledmoayad/clawgo/releases/latest"
	defer func() { releaseURL = origURL }()

	// Set a known version
	origVersion := Version
	Version = "1.0.0"
	defer func() { Version = origVersion }()

	cmd := newUpdateCmd()
	cmd.SetArgs([]string{"--yes"})
	err = cmd.Execute()
	assert.NoError(t, err)
}
