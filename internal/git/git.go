package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// FileStatus represents the status of a single file in git status output.
type FileStatus struct {
	Path     string
	Status   string // "M" modified, "A" added, "D" deleted, "?" untracked, "R" renamed, etc.
	IsStaged bool
}

// StatusResult contains parsed git status output.
type StatusResult struct {
	Files   []FileStatus
	Branch  string
	IsClean bool
}

// Status runs `git -C <dir> status --porcelain=v1 -b` and parses the output.
func Status(ctx context.Context, dir string) (*StatusResult, error) {
	out, err := run(ctx, dir, "status", "--porcelain=v1", "-b")
	if err != nil {
		return nil, err
	}

	result := &StatusResult{}
	lines := strings.Split(out, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Branch line starts with "##"
		if strings.HasPrefix(line, "##") {
			branch := strings.TrimPrefix(line, "## ")
			// Handle "branch...origin/branch" format
			if idx := strings.Index(branch, "..."); idx != -1 {
				branch = branch[:idx]
			}
			result.Branch = branch
			continue
		}

		// Status lines are at least 4 characters: XY <space> <path>
		if len(line) < 4 {
			continue
		}

		x := line[0] // staged status
		y := line[1] // working tree status

		path := line[3:] // skip "XY "

		// Handle renamed files: "R  old -> new"
		if strings.Contains(path, " -> ") {
			parts := strings.SplitN(path, " -> ", 2)
			path = parts[1]
		}

		fs := FileStatus{Path: path}

		// Untracked files
		if x == '?' && y == '?' {
			fs.Status = "?"
		} else if x != ' ' && x != '?' {
			// Staged change
			fs.Status = string(x)
			fs.IsStaged = true
		} else if y != ' ' {
			// Working tree change
			fs.Status = string(y)
		}

		result.Files = append(result.Files, fs)
	}

	result.IsClean = len(result.Files) == 0
	return result, nil
}

// Branch returns the current branch name.
// Runs: git -C <dir> rev-parse --abbrev-ref HEAD
func Branch(ctx context.Context, dir string) (string, error) {
	return run(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// Diff returns the diff output for the given args.
// Runs: git -C <dir> diff <args...>
func Diff(ctx context.Context, dir string, args ...string) (string, error) {
	return run(ctx, dir, append([]string{"diff"}, args...)...)
}

// Log returns recent commit log entries.
// Runs: git -C <dir> log --oneline -n <count>
func Log(ctx context.Context, dir string, count int) (string, error) {
	return run(ctx, dir, "log", "--oneline", "-n", fmt.Sprintf("%d", count))
}

// run is a helper that executes a git command and returns stdout.
func run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimSpace(string(out)), nil
}
