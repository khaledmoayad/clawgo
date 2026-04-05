// Package read implements the ReadTool for reading files from the filesystem.
// It returns file contents in cat -n format with line numbers, supports
// offset/limit for partial reads, and detects binary files.
package read

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/filestate"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

const defaultLimit = 2000
const binaryCheckSize = 8192

type input struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"` // 0-indexed line offset
	Limit    int    `json:"limit"`  // number of lines to read, 0 = default
}

// ReadTool reads files from the filesystem.
type ReadTool struct{}

// New creates a new ReadTool.
func New() *ReadTool { return &ReadTool{} }

func (t *ReadTool) Name() string                { return "Read" }
func (t *ReadTool) Description() string          { return toolDescription }
func (t *ReadTool) IsReadOnly() bool             { return true }
func (t *ReadTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true because reading files is always safe for concurrent execution.
func (t *ReadTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

func (t *ReadTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("Read", true, permCtx), nil
}

func (t *ReadTool) Call(_ context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.FilePath) == "" {
		return tools.ErrorResult("required field \"file_path\" is missing or empty"), nil
	}

	// Read the file
	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to read file: %s", err.Error())), nil
	}

	// Check for binary content (null bytes in first 8192 bytes)
	checkLen := len(data)
	if checkLen > binaryCheckSize {
		checkLen = binaryCheckSize
	}
	if bytes.ContainsRune(data[:checkLen], 0) {
		return tools.ErrorResult("File appears to be binary and cannot be displayed as text"), nil
	}

	// Handle empty file
	content := string(data)
	if content == "" {
		return tools.TextResult("(empty file)"), nil
	}

	// Split into lines
	lines := strings.Split(content, "\n")
	// Remove trailing empty line from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Apply offset
	offset := in.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		return tools.TextResult("(offset beyond end of file)"), nil
	}

	// Apply limit
	limit := in.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}

	// Format in cat -n style: right-aligned 6-char line number + tab + content
	// Line numbers start at 1 + offset
	var output strings.Builder
	for i := offset; i < end; i++ {
		lineNum := i + 1 // 1-indexed line numbers
		fmt.Fprintf(&output, "%6d\t%s\n", lineNum, lines[i])
	}

	// Record the read in the file state cache for read-before-edit enforcement
	if toolCtx != nil && toolCtx.FileStateCache != nil {
		toolCtx.FileStateCache.Set(in.FilePath, filestate.FileState{
			Content:   content,
			Timestamp: time.Now().UnixMilli(),
			Offset:    offset,
			Limit:     limit,
		})
	}

	return tools.TextResult(output.String()), nil
}
