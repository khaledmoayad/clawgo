package sandbox

import (
	"strings"
	"testing"
)

func TestBwrapSandbox_BuildArgs(t *testing.T) {
	s := &BwrapSandbox{AllowNetwork: true}
	args := s.buildArgs("/home/user/project", "bash", []string{"-c", "echo hello"})

	argStr := strings.Join(args, " ")

	// Must have isolation flags
	if !strings.Contains(argStr, "--unshare-all") {
		t.Error("expected --unshare-all in bwrap args")
	}
	if !strings.Contains(argStr, "--die-with-parent") {
		t.Error("expected --die-with-parent in bwrap args")
	}
	if !strings.Contains(argStr, "--new-session") {
		t.Error("expected --new-session in bwrap args")
	}

	// Must bind the work directory
	if !strings.Contains(argStr, "--bind /home/user/project /home/user/project") {
		t.Error("expected --bind workDir in bwrap args")
	}

	// Must have separator and command
	found := false
	for i, a := range args {
		if a == "--" && i+1 < len(args) && args[i+1] == "bash" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected -- separator followed by command in bwrap args")
	}

	// Virtual filesystems
	if !strings.Contains(argStr, "--dev /dev") {
		t.Error("expected --dev /dev in bwrap args")
	}
	if !strings.Contains(argStr, "--proc /proc") {
		t.Error("expected --proc /proc in bwrap args")
	}
}

func TestBwrapSandbox_BuildArgs_NoNetwork(t *testing.T) {
	s := &BwrapSandbox{AllowNetwork: false}
	args := s.buildArgs("/tmp/work", "ls", []string{"-la"})

	argStr := strings.Join(args, " ")

	if !strings.Contains(argStr, "--unshare-net") {
		t.Error("expected --unshare-net when AllowNetwork=false")
	}

	// Should NOT have resolv.conf bind when network is disabled
	if strings.Contains(argStr, "resolv.conf") {
		t.Error("should not bind resolv.conf when network is disabled")
	}
}

func TestDockerSandbox_BuildArgs(t *testing.T) {
	s := &DockerSandbox{Image: "ubuntu:22.04", AllowNetwork: false}
	args := s.buildArgs("/home/user/project", "bash", []string{"-c", "echo hello"})

	argStr := strings.Join(args, " ")

	// Must have docker run --rm
	if !strings.Contains(argStr, "run --rm") {
		t.Error("expected 'docker run --rm' in docker args")
	}

	// Must have volume mount
	if !strings.Contains(argStr, "-v /home/user/project:/home/user/project") {
		t.Error("expected volume mount for workDir")
	}

	// Must have working directory
	if !strings.Contains(argStr, "-w /home/user/project") {
		t.Error("expected -w workDir")
	}

	// Must have network=none when AllowNetwork is false
	if !strings.Contains(argStr, "--network=none") {
		t.Error("expected --network=none when AllowNetwork=false")
	}

	// Must have resource limits
	if !strings.Contains(argStr, "--memory=512m") {
		t.Error("expected --memory=512m")
	}
	if !strings.Contains(argStr, "--cpus=1") {
		t.Error("expected --cpus=1")
	}

	// Must have image followed by command
	imageIdx := -1
	for i, a := range args {
		if a == "ubuntu:22.04" {
			imageIdx = i
			break
		}
	}
	if imageIdx == -1 {
		t.Fatal("expected ubuntu:22.04 in args")
	}
	if imageIdx+1 >= len(args) || args[imageIdx+1] != "bash" {
		t.Error("expected command after image")
	}
}

func TestDockerSandbox_BuildArgs_WithNetwork(t *testing.T) {
	s := &DockerSandbox{Image: "ubuntu:22.04", AllowNetwork: true}
	args := s.buildArgs("/tmp/work", "ls", nil)

	argStr := strings.Join(args, " ")

	if strings.Contains(argStr, "--network=none") {
		t.Error("should not have --network=none when AllowNetwork=true")
	}
}

func TestNewSandbox_Fallback(t *testing.T) {
	// NewSandbox with TypeNone returns nil when neither bwrap nor docker is available
	// On a test machine this depends on what's installed, but we can verify it doesn't panic
	s := NewSandbox(TypeNone)
	// Result may be nil or a valid sandbox depending on the machine
	if s != nil {
		if s.Type() != TypeBwrap && s.Type() != TypeDocker {
			t.Errorf("unexpected sandbox type: %s", s.Type())
		}
	}
}

func TestDetectAvailable(t *testing.T) {
	available := DetectAvailable()

	// Should return a list (possibly empty) of sandbox types
	for _, st := range available {
		if st != TypeBwrap && st != TypeDocker {
			t.Errorf("unexpected sandbox type in available list: %s", st)
		}
	}
}

func TestDockerSandbox_DefaultImage(t *testing.T) {
	// When Image is empty, buildArgs should use default image
	s := &DockerSandbox{Image: ""}
	args := s.buildArgs("/tmp/work", "echo", []string{"test"})

	found := false
	for _, a := range args {
		if a == defaultDockerImage {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected default image %q in args when Image is empty", defaultDockerImage)
	}
}
