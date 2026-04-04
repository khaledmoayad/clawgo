package sandbox

import (
	"context"
	"os/exec"
)

const defaultDockerImage = "ubuntu:22.04"

// DockerSandbox wraps Docker for container-based sandboxed execution.
// It provides cross-platform isolation via Docker containers with
// resource limits and optional network restriction.
type DockerSandbox struct {
	// Image is the Docker image to use for the container.
	// Defaults to "ubuntu:22.04" if empty.
	Image string

	// AllowNetwork controls whether the container has network access.
	// If false, --network=none is used.
	AllowNetwork bool
}

func (s *DockerSandbox) Type() SandboxType { return TypeDocker }

// IsAvailable returns true if the docker binary is found in PATH.
func (s *DockerSandbox) IsAvailable() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

// Execute runs a command inside a Docker container with resource limits.
func (s *DockerSandbox) Execute(ctx context.Context, workDir, cmd string, args []string) (*ExecuteResult, error) {
	dockerArgs := s.buildArgs(workDir, cmd, args)
	return executeCommand(ctx, "docker", dockerArgs)
}

// buildArgs constructs the docker command-line arguments for testing and execution.
func (s *DockerSandbox) buildArgs(workDir, cmd string, args []string) []string {
	image := s.Image
	if image == "" {
		image = defaultDockerImage
	}

	dockerArgs := []string{
		"run", "--rm",
	}

	// Volume mount for work directory
	dockerArgs = append(dockerArgs, "-v", workDir+":"+workDir)

	// Set working directory
	dockerArgs = append(dockerArgs, "-w", workDir)

	// Network isolation
	if !s.AllowNetwork {
		dockerArgs = append(dockerArgs, "--network=none")
	}

	// Resource limits
	dockerArgs = append(dockerArgs,
		"--memory=512m",
		"--cpus=1",
	)

	// Image
	dockerArgs = append(dockerArgs, image)

	// Command and arguments
	dockerArgs = append(dockerArgs, cmd)
	dockerArgs = append(dockerArgs, args...)

	return dockerArgs
}
