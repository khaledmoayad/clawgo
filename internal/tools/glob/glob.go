package glob

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/khaledmoayad/clawgo/internal/git"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// maxGlobResults is the maximum number of files returned by the glob tool.
// Matches Claude Code's 100-file cap to prevent flooding the context window.
const maxGlobResults = 100

// GlobTool finds files matching glob patterns including ** for recursive matching.
type GlobTool struct{}

// New creates a new GlobTool instance.
func New() *GlobTool {
	return &GlobTool{}
}

// Name returns the tool name.
func (t *GlobTool) Name() string { return "Glob" }

// Description returns the tool description.
func (t *GlobTool) Description() string { return toolDescription }

// InputSchema returns the JSON schema for tool input.
func (t *GlobTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsReadOnly returns true because glob only reads the filesystem.
func (t *GlobTool) IsReadOnly() bool { return true }

// IsConcurrencySafe returns true because glob only reads the filesystem, safe for concurrent execution.
func (t *GlobTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// Call executes the glob tool.
// It uses doublestar for ** pattern matching, sorts results by modification time
// (newest first), and filters out gitignored files when in a git repo.
func (t *GlobTool) Call(ctx context.Context, input json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	data, err := tools.ParseRawInput(input)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	pattern, err := tools.RequireString(data, "pattern")
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	searchPath := tools.OptionalString(data, "path", toolCtx.WorkingDir)

	// Make path absolute if needed
	if !filepath.IsAbs(searchPath) {
		searchPath = filepath.Join(toolCtx.WorkingDir, searchPath)
	}

	// Use doublestar to match files using os.DirFS
	fsys := os.DirFS(searchPath)
	matches, err := doublestar.Glob(fsys, pattern)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("glob error: %s", err)), nil
	}

	if len(matches) == 0 {
		return tools.TextResult("No matches found"), nil
	}

	// Filter out gitignored files if we're in a git repo
	inGitRepo := git.IsGitRepo(searchPath)
	if inGitRepo {
		matches = git.FilterIgnored(ctx, searchPath, matches)
	}

	// Filter to only regular files and collect mod times
	type fileEntry struct {
		path    string
		modTime int64
	}

	var entries []fileEntry
	for _, m := range matches {
		fullPath := filepath.Join(searchPath, m)
		info, statErr := os.Stat(fullPath)
		if statErr != nil {
			continue
		}
		if info.IsDir() {
			continue
		}
		entries = append(entries, fileEntry{
			path:    m,
			modTime: info.ModTime().UnixNano(),
		})
	}

	if len(entries) == 0 {
		return tools.TextResult("No matches found"), nil
	}

	// Sort by modification time, newest first
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].modTime > entries[j].modTime
	})

	// Apply max results cap to prevent flooding the context window
	truncated := false
	totalMatches := len(entries)
	if len(entries) > maxGlobResults {
		entries = entries[:maxGlobResults]
		truncated = true
	}

	// Format output: one file per line
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintln(&b, e.path)
	}

	output := strings.TrimSpace(b.String())

	if truncated {
		output += fmt.Sprintf("\n\n[Results truncated: showing %d of %d total matches. Use a more specific pattern to see other files.]", maxGlobResults, totalMatches)
	}

	result := tools.TextResult(output)
	result.Metadata = map[string]any{
		"num_files":     len(entries),
		"total_matches": totalMatches,
		"truncated":     truncated,
	}
	return result, nil
}

// CheckPermissions determines whether the tool should be allowed, denied, or require user prompt.
func (t *GlobTool) CheckPermissions(ctx context.Context, input json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission(t.Name(), t.IsReadOnly(), permCtx), nil
}
