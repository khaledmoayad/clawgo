package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// MDM/enterprise settings paths per platform.
const (
	linuxMDMPath       = "/etc/claude-code/managed-settings.json"
	linuxMDMDropInDir  = "/etc/claude-code/managed-settings.d"
	darwinMDMPath      = "/Library/Managed Preferences/com.anthropic.claude-code.plist"
	windowsRegKey      = `HKLM\SOFTWARE\Policies\Anthropic\ClaudeCode`
	windowsRegKeyHKCU  = `HKCU\SOFTWARE\Policies\Anthropic\ClaudeCode`
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

// loadMDMLinux reads /etc/claude-code/managed-settings.json and merges
// drop-in files from /etc/claude-code/managed-settings.d/*.json.
func loadMDMLinux() *Settings {
	return loadMDMLinuxFromPaths(linuxMDMPath, linuxMDMDropInDir)
}

// loadMDMLinuxFromPaths is the testable implementation.
// It reads the base settings file and then merges drop-in files
// from the drop-in directory in alphabetical order.
func loadMDMLinuxFromPaths(basePath, dropInDir string) *Settings {
	result := &Settings{}

	// Load base settings file
	data, err := os.ReadFile(basePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "clawgo: failed to read MDM settings from %s: %v\n", basePath, err)
		}
		// Continue -- drop-in files may still exist
	} else {
		s, err := parseMDMJSON(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "clawgo: failed to parse MDM settings from %s: %v\n", basePath, err)
		} else {
			result = s
		}
	}

	// Load drop-in directory files in alphabetical order
	entries, err := os.ReadDir(dropInDir)
	if err != nil {
		// Drop-in dir missing is expected -- silently skip
		return result
	}

	var jsonFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".json" {
			jsonFiles = append(jsonFiles, filepath.Join(dropInDir, entry.Name()))
		}
	}
	sort.Strings(jsonFiles)

	for _, path := range jsonFiles {
		dropData, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "clawgo: failed to read MDM drop-in %s: %v\n", path, err)
			continue
		}
		s, err := parseMDMJSON(dropData)
		if err != nil {
			// Invalid JSON in drop-in: log warning but do not fail
			fmt.Fprintf(os.Stderr, "clawgo: failed to parse MDM drop-in %s: %v\n", path, err)
			continue
		}
		mergeSettings(result, s)
	}

	return result
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
// Tries HKLM first (higher priority); falls back to HKCU if HKLM is empty.
func loadMDMWindows() *Settings {
	hklmJSON := queryWindowsReg(windowsRegKey)
	hkcuJSON := queryWindowsReg(windowsRegKeyHKCU)
	return loadMDMWindowsFromValues(hklmJSON, hkcuJSON)
}

// queryWindowsReg executes reg query and returns the JSON string value, or "".
func queryWindowsReg(regKey string) string {
	cmd := exec.Command("reg", "query", regKey, "/v", "Settings", "/reg:64")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return parseRegQueryOutput(string(out))
}

// loadMDMWindowsFromValues is the testable implementation.
// HKLM takes priority; HKCU is used as fallback if HKLM is empty.
func loadMDMWindowsFromValues(hklmJSON, hkcuJSON string) *Settings {
	// Try HKLM first (higher priority)
	if hklmJSON != "" {
		s, err := parseMDMJSON([]byte(hklmJSON))
		if err != nil {
			fmt.Fprintf(os.Stderr, "clawgo: failed to parse MDM HKLM registry settings: %v\n", err)
		} else {
			return s
		}
	}

	// Fall back to HKCU (lower priority)
	if hkcuJSON != "" {
		s, err := parseMDMJSON([]byte(hkcuJSON))
		if err != nil {
			fmt.Fprintf(os.Stderr, "clawgo: failed to parse MDM HKCU registry settings: %v\n", err)
		} else {
			return s
		}
	}

	return &Settings{}
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
