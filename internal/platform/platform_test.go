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
