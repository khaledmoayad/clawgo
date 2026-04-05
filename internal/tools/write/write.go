// Package write implements the WriteTool for creating and overwriting files.
// It creates parent directories as needed, detects staleness (mtime-based),
// preserves original file encoding (line endings), computes git diff output,
// updates the file state cache, and notifies LSP servers after writes.
package write

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/filestate"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// LSPNotifyFunc is a hook for LSP file-change notifications. When set, it is
// called (best-effort, fire-and-forget) after a successful write with the
// full file path and the new content. This is wired up at initialization
// time by the application layer.
var LSPNotifyFunc func(filePath string, content string)

// WriteTool creates or overwrites files on the filesystem.
type WriteTool struct{}

// New creates a new WriteTool.
func New() *WriteTool { return &WriteTool{} }

func (t *WriteTool) Name() string                { return "Write" }
func (t *WriteTool) Description() string          { return toolDescription }
func (t *WriteTool) IsReadOnly() bool             { return false }
func (t *WriteTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because writing files modifies the filesystem.
func (t *WriteTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *WriteTool) CheckPermissions(_ context.Context, inp json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	// Check file write glob restrictions first
	var in struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(inp, &in); err == nil && in.FilePath != "" {
		globResult := permissions.CheckFileWritePermission(in.FilePath, permCtx.AllowGlobs, permCtx.DenyGlobs)
		if globResult == permissions.Deny {
			return permissions.Deny, nil
		}
	}
	return permissions.CheckPermission("Write", false, permCtx), nil
}

func (t *WriteTool) Call(_ context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.FilePath) == "" {
		return tools.ErrorResult("required field \"file_path\" is missing or empty"), nil
	}

	fullPath := in.FilePath

	// Staleness detection: check mtime against last read timestamp
	if toolCtx != nil && toolCtx.FileStateCache != nil {
		if err := checkStaleness(fullPath, toolCtx.FileStateCache); err != nil {
			return tools.ErrorResult(err.Error()), nil
		}
	}

	// Read existing file content for encoding detection and diff
	var oldContent []byte
	var isNewFile bool
	oldContent, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			isNewFile = true
			oldContent = nil
		} else {
			return tools.ErrorResult(fmt.Sprintf("Failed to read existing file: %s", err.Error())), nil
		}
	}

	// Preserve line endings from the original file
	finalContent := in.Content
	if !isNewFile && len(oldContent) > 0 {
		origLineEnding := detectLineEnding(string(oldContent))
		finalContent = preserveLineEnding(in.Content, origLineEnding)
	}

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to create directory %s: %s", dir, err.Error())), nil
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(finalContent), 0644); err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to write file: %s", err.Error())), nil
	}

	// Compute git diff (non-fatal -- best effort)
	diffText := computeGitDiff(fullPath, isNewFile)

	// Update file state cache with new content and timestamp
	if toolCtx != nil && toolCtx.FileStateCache != nil {
		toolCtx.FileStateCache.Set(fullPath, filestate.FileState{
			Content:   finalContent,
			Timestamp: time.Now().UnixMilli(),
			Offset:    -1, // full content
			Limit:     -1,
		})
	}

	// Notify LSP server (best-effort, fire-and-forget)
	if LSPNotifyFunc != nil {
		go func() {
			defer func() { _ = recover() }() // never panic from LSP notification
			LSPNotifyFunc(fullPath, finalContent)
		}()
	}

	// Build response message
	msg := buildResponseMessage(fullPath, finalContent, isNewFile, oldContent, diffText)

	result := tools.TextResult(msg)
	result.Metadata = map[string]any{
		"file_path": fullPath,
		"bytes":     len(finalContent),
	}
	if isNewFile {
		result.Metadata["type"] = "create"
	} else {
		result.Metadata["type"] = "update"
	}

	return result, nil
}

// computeGitDiff runs git diff to produce a diff string for the written file.
// Returns empty string if git is unavailable or diff fails.
func computeGitDiff(fullPath string, isNewFile bool) string {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return ""
	}

	dir := filepath.Dir(fullPath)

	if isNewFile {
		// For new files, diff against /dev/null
		cmd := exec.Command("git", "diff", "--no-index", "--", "/dev/null", fullPath)
		cmd.Dir = dir
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out))
	}

	// For existing files, try git diff on the working tree
	cmd := exec.Command("git", "diff", "--", fullPath)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Not in a git repo or other error -- try --no-index with temp
		return ""
	}

	return strings.TrimSpace(string(out))
}

// buildResponseMessage constructs the human-readable response message.
func buildResponseMessage(fullPath, finalContent string, isNewFile bool, oldContent []byte, diffText string) string {
	var msg string
	bytesWritten := len(finalContent)

	if isNewFile {
		msg = fmt.Sprintf("Successfully created %s (%d bytes)", fullPath, bytesWritten)
	} else {
		linesChanged := countLinesChanged(string(oldContent), finalContent)
		msg = fmt.Sprintf("Successfully updated %s (%d bytes, %d lines changed)", fullPath, bytesWritten, linesChanged)
	}

	if diffText != "" {
		msg += "\n\nGit diff:\n" + diffText
	}

	return msg
}

// countLinesChanged counts the number of lines that differ between old and new content.
func countLinesChanged(oldContent, newContent string) int {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Simple count: number of lines added + removed
	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	changed := 0
	for i := 0; i < maxLen; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			changed++
		}
	}
	return changed
}
