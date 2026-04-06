package platform

import (
	"runtime"
	"testing"
)

func TestGetInfo(t *testing.T) {
	info := GetInfo()

	if info.OS == "" {
		t.Error("OS should not be empty")
	}
	if info.Arch == "" {
		t.Error("Arch should not be empty")
	}
	if info.HomeDir == "" {
		t.Error("HomeDir should not be empty")
	}
	if info.Shell == "" {
		t.Error("Shell should not be empty")
	}
	if info.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", info.OS, runtime.GOOS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", info.Arch, runtime.GOARCH)
	}
}

func TestGetInfo_BooleanConsistency(t *testing.T) {
	info := GetInfo()

	// Exactly one of IsLinux/IsDarwin/IsWindows should be true
	trueCount := 0
	if info.IsLinux {
		trueCount++
	}
	if info.IsDarwin {
		trueCount++
	}
	if info.IsWindows {
		trueCount++
	}

	// On unsupported platforms (e.g., freebsd), all might be false,
	// but on standard platforms exactly one should be true.
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		if trueCount != 1 {
			t.Errorf("expected exactly one of IsLinux/IsDarwin/IsWindows to be true, got %d", trueCount)
		}
	default:
		if trueCount > 1 {
			t.Errorf("at most one of IsLinux/IsDarwin/IsWindows should be true, got %d", trueCount)
		}
	}
}

func TestIsRemote_EnvVar(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REMOTE", "true")
	t.Setenv("CLAUDE_CODE_CONTAINER_ID", "")

	if !IsRemote() {
		t.Error("IsRemote() should return true when CLAUDE_CODE_REMOTE=true")
	}
}

func TestIsRemote_Default(t *testing.T) {
	t.Setenv("CLAUDE_CODE_REMOTE", "")
	t.Setenv("CLAUDE_CODE_CONTAINER_ID", "")

	// Note: this test may still return true if /.dockerenv exists on the host
	// but that's expected behavior in a container environment.
	result := IsRemote()
	_ = result // Just verify it doesn't panic
}

func TestIsSandboxed_EnvVar(t *testing.T) {
	t.Setenv("IS_SANDBOX", "true")
	t.Setenv("CLAUDE_CODE_BUBBLEWRAP", "")

	if !IsSandboxed() {
		t.Error("IsSandboxed() should return true when IS_SANDBOX=true")
	}
}

func TestIsSandboxed_BubblewrapEnv(t *testing.T) {
	t.Setenv("IS_SANDBOX", "")
	t.Setenv("CLAUDE_CODE_BUBBLEWRAP", "true")

	if !IsSandboxed() {
		t.Error("IsSandboxed() should return true when CLAUDE_CODE_BUBBLEWRAP=true")
	}
}

func TestDefaultShell(t *testing.T) {
	shell := DefaultShell()
	if shell == "" {
		t.Error("DefaultShell() should return a non-empty string")
	}
}

func TestIsCI_CIEnvVar(t *testing.T) {
	t.Setenv("CI", "true")
	// Clear other CI vars to isolate
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("JENKINS_URL", "")
	t.Setenv("CIRCLECI", "")
	t.Setenv("TRAVIS", "")
	t.Setenv("BUILDKITE", "")
	t.Setenv("CODEBUILD_BUILD_ID", "")

	if !IsCI() {
		t.Error("IsCI() should return true when CI=true")
	}
}

func TestIsCI_GitHubActions(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("JENKINS_URL", "")
	t.Setenv("CIRCLECI", "")
	t.Setenv("TRAVIS", "")
	t.Setenv("BUILDKITE", "")
	t.Setenv("CODEBUILD_BUILD_ID", "")

	if !IsCI() {
		t.Error("IsCI() should return true when GITHUB_ACTIONS=true")
	}
}

func TestIsCI_GitLabCI(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("JENKINS_URL", "")
	t.Setenv("CIRCLECI", "")
	t.Setenv("TRAVIS", "")
	t.Setenv("BUILDKITE", "")
	t.Setenv("CODEBUILD_BUILD_ID", "")

	if !IsCI() {
		t.Error("IsCI() should return true when GITLAB_CI=true")
	}
}

func TestIsCI_NoCIVars(t *testing.T) {
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("JENKINS_URL", "")
	t.Setenv("CIRCLECI", "")
	t.Setenv("TRAVIS", "")
	t.Setenv("BUILDKITE", "")
	t.Setenv("CODEBUILD_BUILD_ID", "")

	if IsCI() {
		t.Error("IsCI() should return false when no CI env vars are set")
	}
}

func TestIsCI_FalseValue(t *testing.T) {
	// "false" should not count as CI
	t.Setenv("CI", "false")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("JENKINS_URL", "")
	t.Setenv("CIRCLECI", "")
	t.Setenv("TRAVIS", "")
	t.Setenv("BUILDKITE", "")
	t.Setenv("CODEBUILD_BUILD_ID", "")

	if IsCI() {
		t.Error("IsCI() should return false when CI=false")
	}
}

func TestTerminalType_Set(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")

	result := TerminalType()
	if result != "xterm-256color" {
		t.Errorf("TerminalType() = %q, want %q", result, "xterm-256color")
	}
}

func TestTerminalType_Unset(t *testing.T) {
	t.Setenv("TERM", "")

	result := TerminalType()
	if result != "dumb" {
		t.Errorf("TerminalType() = %q, want %q", result, "dumb")
	}
}

func TestIsColorTerminal_XTerm256(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("COLORTERM", "")

	if !IsColorTerminal() {
		t.Error("IsColorTerminal() should return true for xterm-256color")
	}
}

func TestIsColorTerminal_Dumb(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("COLORTERM", "")

	if IsColorTerminal() {
		t.Error("IsColorTerminal() should return false for dumb terminal without COLORTERM")
	}
}

func TestIsColorTerminal_ColorTermEnv(t *testing.T) {
	t.Setenv("TERM", "dumb")
	t.Setenv("COLORTERM", "truecolor")

	if !IsColorTerminal() {
		t.Error("IsColorTerminal() should return true when COLORTERM is set")
	}
}

func TestHasGit(t *testing.T) {
	// Git is expected to be available in the test environment
	if !HasGit() {
		t.Error("HasGit() should return true when git is on PATH")
	}
}

func TestGetInfo_NewFields(t *testing.T) {
	// Set known env vars so we can verify the new fields
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("CI", "")
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("GITLAB_CI", "")
	t.Setenv("JENKINS_URL", "")
	t.Setenv("CIRCLECI", "")
	t.Setenv("TRAVIS", "")
	t.Setenv("BUILDKITE", "")
	t.Setenv("CODEBUILD_BUILD_ID", "")
	t.Setenv("COLORTERM", "")

	info := GetInfo()

	if info.TerminalType != "xterm-256color" {
		t.Errorf("info.TerminalType = %q, want %q", info.TerminalType, "xterm-256color")
	}
	if info.IsCI {
		t.Error("info.IsCI should be false when no CI env vars set")
	}
	if !info.HasGit {
		t.Error("info.HasGit should be true when git is on PATH")
	}
	// xterm-256color contains both "xterm" and "256color"
	if !info.IsColorTerm {
		t.Error("info.IsColorTerm should be true for xterm-256color")
	}
}
