// Package sandbox provides sandboxed command execution via bubblewrap (Linux)
// and Docker (cross-platform). It matches the TypeScript sandbox-runtime behavior
// for namespace isolation and container-based execution.
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// SandboxType identifies the sandbox implementation.
type SandboxType string

const (
	TypeBwrap  SandboxType = "bwrap"
	TypeDocker SandboxType = "docker"
	TypeNone   SandboxType = "none"
)

// ExecuteResult holds the output from a sandboxed command execution.
type ExecuteResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Sandbox defines the interface for sandboxed command execution.
type Sandbox interface {
	// Type returns the sandbox implementation type.
	Type() SandboxType

	// IsAvailable returns true if the sandbox runtime binary is present.
	IsAvailable() bool

	// Execute runs a command inside the sandbox.
	Execute(ctx context.Context, workDir string, cmd string, args []string) (*ExecuteResult, error)
}

// NewSandbox creates a sandbox of the preferred type if available.
// Falls back through bwrap -> docker. Returns nil if no sandbox is available.
func NewSandbox(preferred SandboxType) Sandbox {
	if preferred == TypeBwrap {
		s := &BwrapSandbox{}
		if s.IsAvailable() {
			return s
		}
	}
	if preferred == TypeDocker {
		s := &DockerSandbox{Image: defaultDockerImage}
		if s.IsAvailable() {
			return s
		}
	}

	// Fallback chain: bwrap -> docker -> nil
	if preferred == TypeNone || preferred == "" {
		bwrap := &BwrapSandbox{}
		if bwrap.IsAvailable() {
			return bwrap
		}
		docker := &DockerSandbox{Image: defaultDockerImage}
		if docker.IsAvailable() {
			return docker
		}
	}

	return nil
}

// DetectAvailable returns a list of available sandbox types on the current system.
func DetectAvailable() []SandboxType {
	var available []SandboxType
	if _, err := exec.LookPath("bwrap"); err == nil {
		available = append(available, TypeBwrap)
	}
	if _, err := exec.LookPath("docker"); err == nil {
		available = append(available, TypeDocker)
	}
	return available
}

// executeCommand is a shared helper that runs an exec.Cmd and captures output.
func executeCommand(ctx context.Context, name string, args []string) (*ExecuteResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &ExecuteResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return result, fmt.Errorf("sandbox execute failed: %w", err)
		}
	}

	return result, nil
}
