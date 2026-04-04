// Package notebookedit implements the NotebookEditTool for manipulating
// Jupyter notebook (.ipynb) files. It supports adding, editing, deleting,
// and inserting cells while preserving notebook metadata and format.
package notebookedit

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

// Notebook represents the top-level structure of a .ipynb file.
type Notebook struct {
	Cells         []Cell          `json:"cells"`
	Metadata      json.RawMessage `json:"metadata"`
	NBFormat      int             `json:"nbformat"`
	NBFormatMinor int             `json:"nbformat_minor"`
}

// Cell represents a single cell in a Jupyter notebook.
type Cell struct {
	CellType       string          `json:"cell_type"`
	Source         json.RawMessage `json:"source"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	Outputs        json.RawMessage `json:"outputs,omitempty"`
	ExecutionCount *int            `json:"execution_count,omitempty"`
}

type input struct {
	Path     string `json:"path"`
	Command  string `json:"command"`
	CellType string `json:"cell_type"`
	Index    *int   `json:"index"`
	Source   string `json:"source"`
}

func (in *input) Validate() error {
	if strings.TrimSpace(in.Path) == "" {
		return fmt.Errorf("required field \"path\" is missing or empty")
	}
	if strings.TrimSpace(in.Command) == "" {
		return fmt.Errorf("required field \"command\" is missing or empty")
	}
	validCommands := map[string]bool{
		"add_cell": true, "edit_cell": true, "delete_cell": true, "insert_cell": true,
	}
	if !validCommands[in.Command] {
		return fmt.Errorf("invalid command %q: must be one of add_cell, edit_cell, delete_cell, insert_cell", in.Command)
	}
	return nil
}

// NotebookEditTool edits Jupyter notebook (.ipynb) files.
type NotebookEditTool struct{}

// New creates a new NotebookEditTool.
func New() *NotebookEditTool { return &NotebookEditTool{} }

func (t *NotebookEditTool) Name() string                { return "NotebookEdit" }
func (t *NotebookEditTool) Description() string          { return toolDescription }
func (t *NotebookEditTool) IsReadOnly() bool             { return false }
func (t *NotebookEditTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false — notebook editing modifies files on disk.
func (t *NotebookEditTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

// CheckPermissions returns Ask for write operations (requires user confirmation in default mode).
func (t *NotebookEditTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	if permCtx == nil {
		return permissions.Ask, nil
	}
	return permissions.CheckPermission("NotebookEdit", false, permCtx), nil
}

func (t *NotebookEditTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	// Resolve path relative to working directory
	nbPath := in.Path
	if !filepath.IsAbs(nbPath) {
		nbPath = filepath.Join(toolCtx.WorkingDir, nbPath)
	}

	// Read the notebook file
	data, err := os.ReadFile(nbPath)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to read notebook: %s", err)), nil
	}

	// Parse notebook JSON
	var nb Notebook
	if err := json.Unmarshal(data, &nb); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to parse notebook JSON: %s", err)), nil
	}

	// Execute command
	switch in.Command {
	case "add_cell":
		if err := addCell(&nb, &in); err != nil {
			return tools.ErrorResult(err.Error()), nil
		}
	case "edit_cell":
		if err := editCell(&nb, &in); err != nil {
			return tools.ErrorResult(err.Error()), nil
		}
	case "delete_cell":
		if err := deleteCell(&nb, &in); err != nil {
			return tools.ErrorResult(err.Error()), nil
		}
	case "insert_cell":
		if err := insertCell(&nb, &in); err != nil {
			return tools.ErrorResult(err.Error()), nil
		}
	}

	// Write modified notebook back with 2-space indentation
	output, err := json.MarshalIndent(nb, "", "  ")
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to serialize notebook: %s", err)), nil
	}

	if err := os.WriteFile(nbPath, append(output, '\n'), 0644); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to write notebook: %s", err)), nil
	}

	return tools.TextResult(fmt.Sprintf("Successfully executed %s on %s (%d cells)", in.Command, in.Path, len(nb.Cells))), nil
}

// sourceToLines converts a source string to a JSON array of line strings,
// which is the standard .ipynb format for cell source.
func sourceToLines(source string) json.RawMessage {
	lines := strings.SplitAfter(source, "\n")
	// Remove trailing empty string from split if source doesn't end with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	// Ensure the last line ends with newline
	if len(lines) > 0 && !strings.HasSuffix(lines[len(lines)-1], "\n") {
		lines[len(lines)-1] = lines[len(lines)-1] + "\n"
	}
	b, _ := json.Marshal(lines)
	return b
}

func newCell(cellType, source string) Cell {
	cell := Cell{
		CellType: cellType,
		Source:   sourceToLines(source),
		Metadata: json.RawMessage(`{}`),
	}
	if cellType == "code" {
		cell.Outputs = json.RawMessage(`[]`)
		cell.ExecutionCount = nil
	}
	return cell
}

func addCell(nb *Notebook, in *input) error {
	cellType := in.CellType
	if cellType == "" {
		cellType = "code"
	}
	nb.Cells = append(nb.Cells, newCell(cellType, in.Source))
	return nil
}

func editCell(nb *Notebook, in *input) error {
	if in.Index == nil {
		return fmt.Errorf("\"index\" is required for edit_cell command")
	}
	idx := *in.Index
	if idx < 0 || idx >= len(nb.Cells) {
		return fmt.Errorf("cell index %d out of bounds (notebook has %d cells)", idx, len(nb.Cells))
	}
	nb.Cells[idx].Source = sourceToLines(in.Source)
	return nil
}

func deleteCell(nb *Notebook, in *input) error {
	if in.Index == nil {
		return fmt.Errorf("\"index\" is required for delete_cell command")
	}
	idx := *in.Index
	if idx < 0 || idx >= len(nb.Cells) {
		return fmt.Errorf("cell index %d out of bounds (notebook has %d cells)", idx, len(nb.Cells))
	}
	nb.Cells = append(nb.Cells[:idx], nb.Cells[idx+1:]...)
	return nil
}

func insertCell(nb *Notebook, in *input) error {
	if in.Index == nil {
		return fmt.Errorf("\"index\" is required for insert_cell command")
	}
	idx := *in.Index
	if idx < 0 || idx > len(nb.Cells) {
		return fmt.Errorf("cell index %d out of bounds (notebook has %d cells)", idx, len(nb.Cells))
	}
	cellType := in.CellType
	if cellType == "" {
		cellType = "code"
	}
	cell := newCell(cellType, in.Source)
	// Insert at position
	nb.Cells = append(nb.Cells, Cell{}) // grow slice
	copy(nb.Cells[idx+1:], nb.Cells[idx:])
	nb.Cells[idx] = cell
	return nil
}
