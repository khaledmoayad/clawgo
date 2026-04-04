package git

import (
	"context"
	"os/exec"
	"strings"
)

// IsIgnored checks if a file path is ignored by gitignore rules.
// Runs: git -C <dir> check-ignore -q <path>
// Returns true if the file is ignored (exit code 0), false if not (exit code 1).
func IsIgnored(ctx context.Context, dir, path string) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "check-ignore", "-q", path)
	err := cmd.Run()
	return err == nil // exit code 0 means ignored
}

// FilterIgnored takes a list of paths and returns only those NOT ignored.
// It checks each path against gitignore rules and returns the non-ignored ones.
func FilterIgnored(ctx context.Context, dir string, paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	// Use git check-ignore --stdin for efficiency with multiple paths
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "check-ignore", "--stdin")
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\n"))
	out, _ := cmd.Output() // Errors are expected when some paths aren't ignored

	// Build set of ignored paths
	ignored := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			ignored[line] = true
		}
	}

	// Return non-ignored paths
	result := make([]string, 0, len(paths))
	for _, p := range paths {
		if !ignored[p] {
			result = append(result, p)
		}
	}
	return result
}
