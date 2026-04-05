package notebookedit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestNotebook writes a notebook with known cell IDs to a temp file.
func writeTestNotebook(t *testing.T, dir string) string {
	t.Helper()
	nb := Notebook{
		Cells: []Cell{
			{CellType: "code", ID: "cell-abc", Source: sourceToLines("print('hello')"), Metadata: json.RawMessage(`{}`), Outputs: json.RawMessage(`[]`)},
			{CellType: "markdown", ID: "cell-def", Source: sourceToLines("# Title"), Metadata: json.RawMessage(`{"collapsed": true}`)},
			{CellType: "code", ID: "cell-ghi", Source: sourceToLines("x = 42"), Metadata: json.RawMessage(`{}`), Outputs: json.RawMessage(`[]`)},
		},
		NBFormat:      4,
		NBFormatMinor: 5,
		Metadata:      json.RawMessage(`{"kernelspec": {"display_name": "Python 3", "language": "python", "name": "python3"}, "language_info": {"name": "python", "version": "3.10.0"}}`),
	}
	data, err := json.MarshalIndent(nb, "", " ")
	require.NoError(t, err)
	path := filepath.Join(dir, "test.ipynb")
	require.NoError(t, os.WriteFile(path, data, 0644))
	return path
}

// readNotebookFile reads and parses a notebook file from disk.
func readNotebookFile(t *testing.T, path string) Notebook {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var nb Notebook
	require.NoError(t, json.Unmarshal(data, &nb))
	return nb
}

func makeInput(t *testing.T, fields map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(fields)
	require.NoError(t, err)
	return b
}

func TestNotebookEditToolMetadata(t *testing.T) {
	tool := New()

	t.Run("Name returns NotebookEdit", func(t *testing.T) {
		assert.Equal(t, "NotebookEdit", tool.Name())
	})

	t.Run("IsReadOnly returns false", func(t *testing.T) {
		assert.False(t, tool.IsReadOnly())
	})

	t.Run("IsConcurrencySafe returns false", func(t *testing.T) {
		assert.False(t, tool.IsConcurrencySafe(nil))
	})

	t.Run("Description is non-empty", func(t *testing.T) {
		assert.NotEmpty(t, tool.Description())
	})

	t.Run("InputSchema has notebook_path and cell_id", func(t *testing.T) {
		var schema map[string]any
		err := json.Unmarshal(tool.InputSchema(), &schema)
		assert.NoError(t, err)
		props, ok := schema["properties"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, props, "notebook_path")
		assert.Contains(t, props, "cell_id")
		assert.Contains(t, props, "new_source")
		assert.Contains(t, props, "edit_mode")
	})
}

func TestNotebookEditReplaceByCellID(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeTestNotebook(t, dir)

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	inp := makeInput(t, map[string]any{
		"notebook_path": nbPath,
		"cell_id":       "cell-def",
		"new_source":    "# Updated Title",
		"edit_mode":     "replace",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError, "unexpected error: %v", result.Content)

	nb := readNotebookFile(t, nbPath)
	require.Len(t, nb.Cells, 3)

	// Verify the replaced cell
	var source []string
	require.NoError(t, json.Unmarshal(nb.Cells[1].Source, &source))
	assert.Contains(t, source[0], "Updated Title")
	assert.Equal(t, "markdown", nb.Cells[1].CellType)

	// Verify other cells untouched
	var src0 []string
	require.NoError(t, json.Unmarshal(nb.Cells[0].Source, &src0))
	assert.Contains(t, src0[0], "print('hello')")

	var src2 []string
	require.NoError(t, json.Unmarshal(nb.Cells[2].Source, &src2))
	assert.Contains(t, src2[0], "x = 42")
}

func TestNotebookEditInsertAfterCellID(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeTestNotebook(t, dir)

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	inp := makeInput(t, map[string]any{
		"notebook_path": nbPath,
		"cell_id":       "cell-abc",
		"new_source":    "y = 99",
		"cell_type":     "code",
		"edit_mode":     "insert",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError, "unexpected error: %v", result.Content)

	nb := readNotebookFile(t, nbPath)
	require.Len(t, nb.Cells, 4)

	// New cell should be at index 1 (after cell-abc at index 0)
	assert.Equal(t, "code", nb.Cells[1].CellType)
	var source []string
	require.NoError(t, json.Unmarshal(nb.Cells[1].Source, &source))
	assert.Contains(t, source[0], "y = 99")

	// New cell should have an ID assigned (nbformat 4.5)
	assert.NotEmpty(t, nb.Cells[1].ID)
}

func TestNotebookEditInsertAtBeginning(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeTestNotebook(t, dir)

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	inp := makeInput(t, map[string]any{
		"notebook_path": nbPath,
		"new_source":    "# Preamble",
		"cell_type":     "markdown",
		"edit_mode":     "insert",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError, "unexpected error: %v", result.Content)

	nb := readNotebookFile(t, nbPath)
	require.Len(t, nb.Cells, 4)

	// New cell should be at index 0
	assert.Equal(t, "markdown", nb.Cells[0].CellType)
	var source []string
	require.NoError(t, json.Unmarshal(nb.Cells[0].Source, &source))
	assert.Contains(t, source[0], "Preamble")

	// Original first cell should now be at index 1
	assert.Equal(t, "cell-abc", nb.Cells[1].ID)
}

func TestNotebookEditDeleteByCellID(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeTestNotebook(t, dir)

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	inp := makeInput(t, map[string]any{
		"notebook_path": nbPath,
		"cell_id":       "cell-ghi",
		"new_source":    "",
		"edit_mode":     "delete",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError, "unexpected error: %v", result.Content)

	nb := readNotebookFile(t, nbPath)
	require.Len(t, nb.Cells, 2)
	// Remaining cells should be cell-abc and cell-def
	assert.Equal(t, "cell-abc", nb.Cells[0].ID)
	assert.Equal(t, "cell-def", nb.Cells[1].ID)
}

func TestNotebookEditCellIDNotFound(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeTestNotebook(t, dir)

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	inp := makeInput(t, map[string]any{
		"notebook_path": nbPath,
		"cell_id":       "nonexistent-id",
		"new_source":    "oops",
		"edit_mode":     "replace",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "not found")
}

func TestNotebookEditDefaultEditMode(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeTestNotebook(t, dir)

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	// Omit edit_mode -- should default to "replace"
	inp := makeInput(t, map[string]any{
		"notebook_path": nbPath,
		"cell_id":       "cell-abc",
		"new_source":    "print('replaced')",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError, "unexpected error: %v", result.Content)

	nb := readNotebookFile(t, nbPath)
	var source []string
	require.NoError(t, json.Unmarshal(nb.Cells[0].Source, &source))
	assert.Contains(t, source[0], "replaced")
}

func TestNotebookEditPositionalFallback(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeTestNotebook(t, dir)

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	// Use "1" as cell_id -- should use it as 0-based index (cell-def at index 1)
	inp := makeInput(t, map[string]any{
		"notebook_path": nbPath,
		"cell_id":       "1",
		"new_source":    "# Positional Replace",
		"edit_mode":     "replace",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError, "unexpected error: %v", result.Content)

	nb := readNotebookFile(t, nbPath)
	var source []string
	require.NoError(t, json.Unmarshal(nb.Cells[1].Source, &source))
	assert.Contains(t, source[0], "Positional Replace")
}

func TestNotebookEditRequiresNotebookPath(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: t.TempDir()}

	// Omit notebook_path
	inp := makeInput(t, map[string]any{
		"cell_id":    "cell-abc",
		"new_source": "test",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "notebook_path")
}

func TestNotebookEditPreservesMetadata(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeTestNotebook(t, dir)

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	inp := makeInput(t, map[string]any{
		"notebook_path": nbPath,
		"cell_id":       "cell-abc",
		"new_source":    "print('new')",
		"edit_mode":     "replace",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	nb := readNotebookFile(t, nbPath)

	// Notebook-level metadata should be preserved
	assert.Equal(t, 4, nb.NBFormat)
	assert.Equal(t, 5, nb.NBFormatMinor)
	var meta map[string]any
	require.NoError(t, json.Unmarshal(nb.Metadata, &meta))
	assert.Contains(t, meta, "kernelspec")
	assert.Contains(t, meta, "language_info")

	// Cell-level metadata on other cells should be preserved
	var cellMeta map[string]any
	require.NoError(t, json.Unmarshal(nb.Cells[1].Metadata, &cellMeta))
	assert.Contains(t, cellMeta, "collapsed")
}

func TestNotebookEditCheckPermissions(t *testing.T) {
	tool := New()
	// With nil permission context, should return Ask (write tool)
	result, err := tool.CheckPermissions(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, tools.PermissionAsk, result)
}

func TestNotebookEditInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.ipynb")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	inp := makeInput(t, map[string]any{
		"notebook_path": path,
		"cell_id":       "cell-abc",
		"new_source":    "test",
		"edit_mode":     "replace",
	})
	result, err := tool.Call(ctx, inp, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestFindCellByID(t *testing.T) {
	cells := []Cell{
		{CellType: "code", ID: "cell-abc"},
		{CellType: "markdown", ID: "cell-def"},
		{CellType: "code", ID: "cell-ghi"},
	}

	t.Run("exact ID match", func(t *testing.T) {
		assert.Equal(t, 1, findCellByID(cells, "cell-def"))
	})

	t.Run("numeric fallback", func(t *testing.T) {
		assert.Equal(t, 2, findCellByID(cells, "2"))
	})

	t.Run("empty returns -1", func(t *testing.T) {
		assert.Equal(t, -1, findCellByID(cells, ""))
	})

	t.Run("not found returns -1", func(t *testing.T) {
		assert.Equal(t, -1, findCellByID(cells, "nonexistent"))
	})

	t.Run("out of range numeric returns -1", func(t *testing.T) {
		assert.Equal(t, -1, findCellByID(cells, "99"))
	})
}
