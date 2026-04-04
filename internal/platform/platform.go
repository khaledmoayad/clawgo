// Package platform provides cross-platform detection and helpers.
// It detects OS, architecture, shell, and environment characteristics
// (remote mode, sandbox mode) matching the TypeScript utils/platform.ts.
package platform

import (
	"os"
	"runtime"
)

// Info holds platform-specific information for the current system.
type Info struct {
	OS        string // runtime.GOOS value (linux, darwin, windows)
	Arch      string // runtime.GOARCH value (amd64, arm64, etc.)
	IsLinux   bool
	IsDarwin  bool
	IsWindows bool
	HomeDir   string // User's home directory
	Shell     string // Default shell for the platform
}

// GetInfo returns platform information for the current system.
func GetInfo() Info {
	home, _ := os.UserHomeDir()

	return Info{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		IsLinux:   runtime.GOOS == "linux",
		IsDarwin:  runtime.GOOS == "darwin",
		IsWindows: runtime.GOOS == "windows",
		HomeDir:   home,
		Shell:     platformShell(),
	}
}

// IsRemote returns true if running in a remote/container environment.
// Checks CLAUDE_CODE_REMOTE env var, /.dockerenv file, and CLAUDE_CODE_CONTAINER_ID.
func IsRemote() bool {
	if os.Getenv("CLAUDE_CODE_REMOTE") == "true" {
		return true
	}
	// Check for Docker container indicator
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if os.Getenv("CLAUDE_CODE_CONTAINER_ID") != "" {
		return true
	}
	return false
}

// IsSandboxed returns true if running inside a sandbox environment.
// Checks IS_SANDBOX and CLAUDE_CODE_BUBBLEWRAP env vars.
func IsSandboxed() bool {
	if os.Getenv("IS_SANDBOX") == "true" {
		return true
	}
	if os.Getenv("CLAUDE_CODE_BUBBLEWRAP") == "true" {
		return true
	}
	return false
}

// DefaultShell returns the default shell for the current platform.
func DefaultShell() string {
	return platformShell()
}

// HasPdeathsig returns whether the platform supports PR_SET_PDEATHSIG
// (parent death signal). Only Linux supports this.
func HasPdeathsig() bool {
	return hasPdeathsig()
}
