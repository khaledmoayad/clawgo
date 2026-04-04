package attribution

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const coAuthorTrailer = "Co-Authored-By: Claude <noreply@anthropic.com>"

// FormatTrailer returns a git commit trailer for AI attribution.
// If no files were modified by AI, returns an empty string.
func FormatTrailer(tracker *Tracker) string {
	files := tracker.GetAIModifiedFiles()
	if len(files) == 0 {
		return ""
	}
	return coAuthorTrailer
}

// FormatGitNote builds a git note summarizing AI-assisted changes.
// The note lists each AI-modified file with its content hash.
func FormatGitNote(tracker *Tracker) string {
	attrs := tracker.GetAttribution()
	aiFiles := tracker.GetAIModifiedFiles()

	if len(aiFiles) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("AI-assisted changes:\n")
	for _, path := range aiFiles {
		fs := attrs[path]
		if fs != nil {
			fmt.Fprintf(&b, "  %s [%s]\n", path, fs.Hash[:12])
		}
	}
	return b.String()
}

// WriteGitNote adds a git note to the specified commit.
// It executes `git notes add -m <note> <commitHash>` via the system git.
func WriteGitNote(ctx context.Context, commitHash string, note string) error {
	cmd := exec.CommandContext(ctx, "git", "notes", "add", "-m", note, commitHash)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git notes add: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}
