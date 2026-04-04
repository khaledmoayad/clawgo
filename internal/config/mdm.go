package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// MDM/enterprise settings paths per platform.
const (
	linuxMDMPath  = "/etc/claude-code/managed-settings.json"
	darwinMDMPath = "/Library/Managed Preferences/com.anthropic.claude-code.plist"
	windowsRegKey = `HKLM\SOFTWARE\Policies\Anthropic\ClaudeCode`
)

var (
	cachedMDM *Settings
	mdmOnce   sync.Once
)

// LoadMDMSettings reads enterprise/MDM settings from platform-specific stores.
// Results are cached via sync.Once so subprocess calls only happen once per process.
// All errors are logged to stderr but do not fail -- MDM is best-effort.
func LoadMDMSettings() *Settings {
	mdmOnce.Do(func() {
		cachedMDM = loadMDMSettingsPlatform(runtime.GOOS)
	})
	return cachedMDM
}

// ResetMDMCache resets the cached MDM settings (for testing only).
func ResetMDMCache() {
	mdmOnce = sync.Once{}
	cachedMDM = nil
}

// loadMDMSettingsPlatform dispatches to the correct platform reader.
func loadMDMSettingsPlatform(goos string) *Settings {
	switch goos {
	case "linux":
		return loadMDMLinux()
	case "darwin":
		return loadMDMDarwin()
	case "windows":
		return loadMDMWindows()
	default:
		return &Settings{}
	}
}

// loadMDMLinux reads /etc/claude-code/managed-settings.json.
func loadMDMLinux() *Settings {
	data, err := os.ReadFile(linuxMDMPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "clawgo: failed to read MDM settings from %s: %v\n", linuxMDMPath, err)
		}
		return &Settings{}
	}
	s, err := parseMDMJSON(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clawgo: failed to parse MDM settings from %s: %v\n", linuxMDMPath, err)
		return &Settings{}
	}
	return s
}

// loadMDMDarwin uses plutil to convert the managed preferences plist to JSON.
func loadMDMDarwin() *Settings {
	cmd := exec.Command("plutil", "-convert", "json", "-o", "-", darwinMDMPath)
	out, err := cmd.Output()
	if err != nil {
		// plutil fails if file missing or invalid -- this is expected
		return &Settings{}
	}
	s, err := parseMDMJSON(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clawgo: failed to parse MDM plist output: %v\n", err)
		return &Settings{}
	}
	return s
}

// loadMDMWindows uses reg query to read settings from the Windows registry.
func loadMDMWindows() *Settings {
	cmd := exec.Command("reg", "query", windowsRegKey, "/v", "Settings", "/reg:64")
	out, err := cmd.Output()
	if err != nil {
		// reg query fails if key doesn't exist -- this is expected
		return &Settings{}
	}

	// Parse reg query output: look for the REG_SZ value after "Settings"
	jsonStr := parseRegQueryOutput(string(out))
	if jsonStr == "" {
		return &Settings{}
	}

	s, err := parseMDMJSON([]byte(jsonStr))
	if err != nil {
		fmt.Fprintf(os.Stderr, "clawgo: failed to parse MDM registry settings: %v\n", err)
		return &Settings{}
	}
	return s
}

// parseRegQueryOutput extracts the REG_SZ value from reg query output.
// Output format: "    Settings    REG_SZ    {json value}"
func parseRegQueryOutput(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "REG_SZ") {
			parts := strings.SplitN(line, "REG_SZ", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// parseMDMJSON parses JSON data into a Settings struct.
// Exported for testing.
func parseMDMJSON(data []byte) (*Settings, error) {
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
