// Package write implements the WriteTool for creating and overwriting files.
// It creates parent directories as needed and writes content atomically.
package write

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

type input struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

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

	// Enforce read-before-write for existing files
	if toolCtx != nil && toolCtx.FileStateCache != nil {
		if _, err := os.Stat(in.FilePath); err == nil {
			// File exists -- must have been read first
			if !toolCtx.FileStateCache.Has(in.FilePath) {
				return tools.ErrorResult(fmt.Sprintf("You must read the file before overwriting it. Use the Read tool to read %s first.", in.FilePath)), nil
			}
		}
	}

	// Create parent directories
	dir := filepath.Dir(in.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to create directory %s: %s", dir, err.Error())), nil
	}

	// Write file
	if err := os.WriteFile(in.FilePath, []byte(in.Content), 0644); err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to write file: %s", err.Error())), nil
	}

	return tools.TextResult(fmt.Sprintf("Successfully wrote %d bytes to %s", len(in.Content), in.FilePath)), nil
}
