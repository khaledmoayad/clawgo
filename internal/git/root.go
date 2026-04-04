// Package git provides Git CLI wrapper operations for ClawGo.
// It shells out to the git binary for status, diff, log, branch,
// gitignore checking, and repository root discovery.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// FindRoot returns the git repository root for the given directory.
// Runs: git -C <dir> rev-parse --show-toplevel
// Returns error if not in a git repo.
func FindRoot(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsGitRepo returns true if the directory is inside a git repository.
func IsGitRepo(dir string) bool {
	_, err := FindRoot(dir)
	return err == nil
}
