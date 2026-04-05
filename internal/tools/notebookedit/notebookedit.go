// Package notebookedit implements the NotebookEditTool for manipulating
// Jupyter notebook (.ipynb) files. It supports replace, insert, and delete
// operations using cell_id-based addressing matching Claude Code's interface.
package notebookedit

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
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
	ID             string          `json:"id,omitempty"`
	Source         json.RawMessage `json:"source"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	Outputs        json.RawMessage `json:"outputs,omitempty"`
	ExecutionCount *int            `json:"execution_count,omitempty"`
}

type input struct {
	NotebookPath string `json:"notebook_path"`
	CellID       string `json:"cell_id,omitempty"`
	NewSource    string `json:"new_source"`
	CellType     string `json:"cell_type,omitempty"`
	EditMode     string `json:"edit_mode,omitempty"` // replace, insert, delete
}

func (in *input) Validate() error {
	if strings.TrimSpace(in.NotebookPath) == "" {
		return fmt.Errorf("required field \"notebook_path\" is missing or empty")
	}
	// Default edit_mode to replace
	if in.EditMode == "" {
		in.EditMode = "replace"
	}
	validModes := map[string]bool{
		"replace": true, "insert": true, "delete": true,
	}
	if !validModes[in.EditMode] {
		return fmt.Errorf("invalid edit_mode %q: must be one of replace, insert, delete", in.EditMode)
	}
	// cell_id is required for replace and delete
	if in.CellID == "" && in.EditMode != "insert" {
		return fmt.Errorf("cell_id is required for %s mode", in.EditMode)
	}
	// cell_type is required for insert
	if in.EditMode == "insert" && in.CellType == "" {
		in.CellType = "code" // default to code for insert
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

// IsConcurrencySafe returns false -- notebook editing modifies files on disk.
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
	nbPath := in.NotebookPath
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

	// Execute based on edit_mode
	switch in.EditMode {
	case "replace":
		if err := replaceCell(&nb, &in); err != nil {
			return tools.ErrorResult(err.Error()), nil
		}
	case "insert":
		if err := insertCell(&nb, &in); err != nil {
			return tools.ErrorResult(err.Error()), nil
		}
	case "delete":
		if err := deleteCell(&nb, &in); err != nil {
			return tools.ErrorResult(err.Error()), nil
		}
	}

	// Write modified notebook back with 1-space indentation (matches TS IPYNB_INDENT = 1)
	output, err := json.MarshalIndent(nb, "", " ")
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to serialize notebook: %s", err)), nil
	}

	if err := os.WriteFile(nbPath, append(output, '\n'), 0644); err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to write notebook: %s", err)), nil
	}

	return tools.TextResult(fmt.Sprintf("Successfully executed %s on %s (%d cells)", in.EditMode, in.NotebookPath, len(nb.Cells))), nil
}

// findCellByID scans cells for a matching ID field.
// Returns the index or -1 if not found.
// If cellID is empty, returns -1.
// As a fallback, if cellID is purely numeric, it is treated as a 0-based index.
func findCellByID(cells []Cell, cellID string) int {
	if cellID == "" {
		return -1
	}
	// First try exact ID match
	for i, c := range cells {
		if c.ID == cellID {
			return i
		}
	}
	// Fallback: if cellID is purely numeric, use as 0-based index
	if idx, err := strconv.Atoi(cellID); err == nil {
		if idx >= 0 && idx < len(cells) {
			return idx
		}
	}
	return -1
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

// generateCellID generates a random cell ID for new cells (matches TS behavior).
func generateCellID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 13)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func newCell(cellType, source string, nb *Notebook) Cell {
	cell := Cell{
		CellType: cellType,
		Source:   sourceToLines(source),
		Metadata: json.RawMessage(`{}`),
	}
	// Assign cell ID for notebooks that support it (nbformat >= 4.5)
	if nb.NBFormat > 4 || (nb.NBFormat == 4 && nb.NBFormatMinor >= 5) {
		cell.ID = generateCellID()
	}
	if cellType == "code" {
		cell.Outputs = json.RawMessage(`[]`)
		cell.ExecutionCount = nil
	}
	return cell
}

func replaceCell(nb *Notebook, in *input) error {
	idx := findCellByID(nb.Cells, in.CellID)
	if idx == -1 {
		return fmt.Errorf("cell with ID %q not found in notebook", in.CellID)
	}
	nb.Cells[idx].Source = sourceToLines(in.NewSource)
	// Reset execution count and outputs for code cells (matches TS)
	if nb.Cells[idx].CellType == "code" {
		nb.Cells[idx].ExecutionCount = nil
		nb.Cells[idx].Outputs = json.RawMessage(`[]`)
	}
	// Update cell type if specified and different
	if in.CellType != "" && in.CellType != nb.Cells[idx].CellType {
		nb.Cells[idx].CellType = in.CellType
	}
	return nil
}

func insertCell(nb *Notebook, in *input) error {
	cell := newCell(in.CellType, in.NewSource, nb)
	if in.CellID == "" {
		// Insert at beginning (index 0) when no cell_id specified
		nb.Cells = append([]Cell{cell}, nb.Cells...)
		return nil
	}
	idx := findCellByID(nb.Cells, in.CellID)
	if idx == -1 {
		return fmt.Errorf("cell with ID %q not found in notebook", in.CellID)
	}
	// Insert AFTER the found cell
	insertIdx := idx + 1
	nb.Cells = append(nb.Cells, Cell{}) // grow slice
	copy(nb.Cells[insertIdx+1:], nb.Cells[insertIdx:])
	nb.Cells[insertIdx] = cell
	return nil
}

func deleteCell(nb *Notebook, in *input) error {
	idx := findCellByID(nb.Cells, in.CellID)
	if idx == -1 {
		return fmt.Errorf("cell with ID %q not found in notebook", in.CellID)
	}
	nb.Cells = append(nb.Cells[:idx], nb.Cells[idx+1:]...)
	return nil
}
