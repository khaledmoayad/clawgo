// Package platform provides cross-platform detection and helpers.
// It detects OS, architecture, shell, and environment characteristics
// (remote mode, sandbox mode) matching the TypeScript utils/platform.ts.
package platform

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Info holds platform-specific information for the current system.
type Info struct {
	OS           string // runtime.GOOS value (linux, darwin, windows)
	Arch         string // runtime.GOARCH value (amd64, arm64, etc.)
	IsLinux      bool
	IsDarwin     bool
	IsWindows    bool
	HomeDir      string // User's home directory
	Shell        string // Default shell for the platform
	IsCI         bool   // Running in a CI environment
	TerminalType string // TERM env var value, or "dumb" if unset
	HasGit       bool   // Git binary available on PATH
	IsColorTerm  bool   // Terminal supports color output
}

// GetInfo returns platform information for the current system.
func GetInfo() Info {
	home, _ := os.UserHomeDir()

	return Info{
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		IsLinux:      runtime.GOOS == "linux",
		IsDarwin:     runtime.GOOS == "darwin",
		IsWindows:    runtime.GOOS == "windows",
		HomeDir:      home,
		Shell:        platformShell(),
		IsCI:         IsCI(),
		TerminalType: TerminalType(),
		HasGit:       HasGit(),
		IsColorTerm:  IsColorTerminal(),
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

// ciEnvVars lists environment variables that indicate a CI environment.
var ciEnvVars = []string{
	"CI",
	"GITHUB_ACTIONS",
	"GITLAB_CI",
	"JENKINS_URL",
	"CIRCLECI",
	"TRAVIS",
	"BUILDKITE",
	"CODEBUILD_BUILD_ID",
}

// IsCI returns true if running in a CI environment.
// Checks standard CI env vars: CI, GITHUB_ACTIONS, GITLAB_CI, JENKINS_URL,
// CIRCLECI, TRAVIS, BUILDKITE, CODEBUILD_BUILD_ID.
func IsCI() bool {
	for _, v := range ciEnvVars {
		if val := os.Getenv(v); val != "" && val != "false" {
			return true
		}
	}
	return false
}

// TerminalType returns the TERM environment variable value, or "dumb" if unset.
func TerminalType() string {
	if t := os.Getenv("TERM"); t != "" {
		return t
	}
	return "dumb"
}

// IsColorTerminal returns true if the terminal supports color output.
// Checks for common color-capable TERM values and COLORTERM env var.
func IsColorTerminal() bool {
	term := TerminalType()
	if strings.Contains(term, "color") || strings.Contains(term, "xterm") {
		return true
	}
	return os.Getenv("COLORTERM") != ""
}

// HasGit returns true if the git binary is available on PATH.
func HasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
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
