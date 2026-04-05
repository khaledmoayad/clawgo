package edit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// EditTool performs string replacement in files with uniqueness validation.
type EditTool struct{}

// New creates a new EditTool instance.
func New() *EditTool {
	return &EditTool{}
}

// Name returns the tool name.
func (t *EditTool) Name() string { return "Edit" }

// Description returns the tool description.
func (t *EditTool) Description() string { return toolDescription }

// InputSchema returns the JSON schema for tool input.
func (t *EditTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsReadOnly returns false because this tool modifies files.
func (t *EditTool) IsReadOnly() bool { return false }

// IsConcurrencySafe returns false because editing files modifies the filesystem.
func (t *EditTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

// Call executes the edit tool.
// It validates input, then performs string replacement with uniqueness checking,
// or creates a new file when old_str is empty.
func (t *EditTool) Call(ctx context.Context, input json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	data, err := tools.ParseRawInput(input)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	filePath, err := tools.RequireString(data, "file_path")
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	// old_str and new_str can be empty strings, so extract them directly
	oldStr, _ := data["old_str"].(string)
	newStr, _ := data["new_str"].(string)

	// Make path absolute relative to working directory if not already absolute
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(toolCtx.WorkingDir, filePath)
	}

	// Enforce read-before-edit: file must have been read first (unless creating new)
	if oldStr != "" && toolCtx != nil && toolCtx.FileStateCache != nil {
		if !toolCtx.FileStateCache.Has(filePath) {
			return tools.ErrorResult(fmt.Sprintf("You must read the file before editing it. Use the Read tool to read %s first.", filePath)), nil
		}
	}

	// Special case: empty old_str means create/overwrite file with new_str
	if oldStr == "" {
		return t.createFile(filePath, newStr)
	}

	return t.replaceInFile(filePath, oldStr, newStr)
}

// createFile creates or overwrites a file with the given content.
func (t *EditTool) createFile(filePath, content string) (*tools.ToolResult, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to create directory %s: %s", dir, err)), nil
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to write file %s: %s", filePath, err)), nil
	}

	return tools.TextResult(fmt.Sprintf("Created file %s (%d bytes)", filePath, len(content))), nil
}

// replaceInFile performs the string replacement with uniqueness validation.
func (t *EditTool) replaceInFile(filePath, oldStr, newStr string) (*tools.ToolResult, error) {
	// Read the original file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to read file %s: %s", filePath, err)), nil
	}

	content := string(data)

	// Count occurrences of old_str
	count := strings.Count(content, oldStr)

	if count == 0 {
		return tools.ErrorResult(fmt.Sprintf(`"old_str" not found in file %s`, filePath)), nil
	}

	if count > 1 {
		return tools.ErrorResult(fmt.Sprintf(`"old_str" matches %d locations in file %s (must match exactly 1)`, count, filePath)), nil
	}

	// Preserve original file permissions
	info, err := os.Stat(filePath)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to stat file %s: %s", filePath, err)), nil
	}
	perm := info.Mode().Perm()

	// Perform the replacement (count == 1, so exactly one replacement)
	newContent := strings.Replace(content, oldStr, newStr, 1)

	// Write back with original permissions
	if err := os.WriteFile(filePath, []byte(newContent), perm); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to write file %s: %s", filePath, err)), nil
	}

	// Build a snippet showing context around the edit
	snippet := buildSnippet(newContent, newStr)

	return tools.TextResult(fmt.Sprintf("Edited %s\n\n%s", filePath, snippet)), nil
}

// buildSnippet extracts a few lines around the replacement to show context.
func buildSnippet(content, replacement string) string {
	lines := strings.Split(content, "\n")
	replLines := strings.Split(replacement, "\n")

	// Find the line containing the start of the replacement
	idx := strings.Index(content, replacement)
	if idx < 0 {
		// Replacement might be empty (deletion)
		return "(content deleted)"
	}

	// Count newlines before the replacement to find the line number
	lineNum := strings.Count(content[:idx], "\n")

	// Show a window of lines around the edit
	const contextLines = 3
	start := lineNum - contextLines
	if start < 0 {
		start = 0
	}
	end := lineNum + len(replLines) + contextLines
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&b, "%4d | %s\n", i+1, lines[i])
	}
	return b.String()
}

// CheckPermissions determines whether the tool should be allowed, denied, or require user prompt.
func (t *EditTool) CheckPermissions(ctx context.Context, inp json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
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
	return permissions.CheckPermission(t.Name(), t.IsReadOnly(), permCtx), nil
}
