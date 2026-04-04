//go:build linux

package sandbox

import (
	"context"
	"os"
	"os/exec"
)

// BwrapSandbox wraps bubblewrap (bwrap) for Linux namespace-based sandboxing.
// It provides isolation via unshared namespaces, read-only bind mounts, and
// optional network restriction.
type BwrapSandbox struct {
	// AllowNetwork controls whether the sandbox has network access.
	// If false, --unshare-net is used to deny network access.
	AllowNetwork bool
}

func (s *BwrapSandbox) Type() SandboxType { return TypeBwrap }

// IsAvailable returns true if the bwrap binary is found in PATH.
func (s *BwrapSandbox) IsAvailable() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

// Execute runs a command inside a bubblewrap sandbox with namespace isolation.
func (s *BwrapSandbox) Execute(ctx context.Context, workDir, cmd string, args []string) (*ExecuteResult, error) {
	bwrapArgs := s.buildArgs(workDir, cmd, args)
	return executeCommand(ctx, "bwrap", bwrapArgs)
}

// buildArgs constructs the bwrap command-line arguments for testing and execution.
func (s *BwrapSandbox) buildArgs(workDir, cmd string, args []string) []string {
	bwrapArgs := []string{
		"--unshare-all",
		"--die-with-parent",
		"--new-session",
	}

	// Read-only system bind mounts
	bwrapArgs = append(bwrapArgs,
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/bin", "/bin",
	)

	// /lib64 may not exist on all systems (e.g., 32-bit only)
	if _, err := os.Stat("/lib64"); err == nil {
		bwrapArgs = append(bwrapArgs, "--ro-bind", "/lib64", "/lib64")
	}

	// /sbin for system utilities
	if _, err := os.Stat("/sbin"); err == nil {
		bwrapArgs = append(bwrapArgs, "--ro-bind", "/sbin", "/sbin")
	}

	// Network: bind resolv.conf only if network is allowed
	if s.AllowNetwork {
		if _, err := os.Stat("/etc/resolv.conf"); err == nil {
			bwrapArgs = append(bwrapArgs, "--ro-bind", "/etc/resolv.conf", "/etc/resolv.conf")
		}
	} else {
		bwrapArgs = append(bwrapArgs, "--unshare-net")
	}

	// Writable work directory and tmp
	bwrapArgs = append(bwrapArgs,
		"--bind", workDir, workDir,
		"--tmpfs", "/tmp",
	)

	// Virtual filesystems
	bwrapArgs = append(bwrapArgs,
		"--dev", "/dev",
		"--proc", "/proc",
	)

	// Separator and command
	bwrapArgs = append(bwrapArgs, "--")
	bwrapArgs = append(bwrapArgs, cmd)
	bwrapArgs = append(bwrapArgs, args...)

	return bwrapArgs
}
