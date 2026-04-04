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

// emptyNotebook returns a minimal valid .ipynb notebook JSON.
func emptyNotebook() string {
	return `{
  "cells": [],
  "metadata": {
    "kernelspec": {
      "display_name": "Python 3",
      "language": "python",
      "name": "python3"
    },
    "language_info": {
      "name": "python",
      "version": "3.10.0"
    }
  },
  "nbformat": 4,
  "nbformat_minor": 5
}`
}

// notebookWithCells returns a notebook JSON with some pre-existing cells.
func notebookWithCells() string {
	return `{
  "cells": [
    {
      "cell_type": "code",
      "source": ["print('hello')\n"],
      "metadata": {},
      "outputs": [],
      "execution_count": 1
    },
    {
      "cell_type": "markdown",
      "source": ["# Title\n"],
      "metadata": {}
    }
  ],
  "metadata": {
    "kernelspec": {
      "display_name": "Python 3",
      "language": "python",
      "name": "python3"
    }
  },
  "nbformat": 4,
  "nbformat_minor": 5
}`
}

// writeNotebook writes notebook content to a temp file and returns the path.
func writeNotebook(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "test.ipynb")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
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

	t.Run("InputSchema is valid JSON", func(t *testing.T) {
		var schema map[string]any
		err := json.Unmarshal(tool.InputSchema(), &schema)
		assert.NoError(t, err)
		props, ok := schema["properties"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, props, "path")
		assert.Contains(t, props, "command")
	})
}

func TestNotebookEditAddCell(t *testing.T) {
	dir := t.TempDir()
	nbPath := writeNotebook(t, dir, emptyNotebook())

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	t.Run("add code cell to empty notebook", func(t *testing.T) {
		input := makeInput(t, map[string]any{
			"path":      "test.ipynb",
			"command":   "add_cell",
			"cell_type": "code",
			"source":    "print('hello')",
		})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.False(t, result.IsError, "unexpected error: %v", result.Content)

		nb := readNotebookFile(t, nbPath)
		require.Len(t, nb.Cells, 1)
		assert.Equal(t, "code", nb.Cells[0].CellType)
	})

	t.Run("add markdown cell", func(t *testing.T) {
		input := makeInput(t, map[string]any{
			"path":      "test.ipynb",
			"command":   "add_cell",
			"cell_type": "markdown",
			"source":    "# Heading",
		})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.False(t, result.IsError)

		nb := readNotebookFile(t, nbPath)
		require.Len(t, nb.Cells, 2)
		assert.Equal(t, "markdown", nb.Cells[1].CellType)
	})
}

func TestNotebookEditEditCell(t *testing.T) {
	dir := t.TempDir()
	writeNotebook(t, dir, notebookWithCells())

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	t.Run("edit existing cell source", func(t *testing.T) {
		input := makeInput(t, map[string]any{
			"path":    "test.ipynb",
			"command": "edit_cell",
			"index":   0,
			"source":  "print('modified')",
		})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.False(t, result.IsError)

		nb := readNotebookFile(t, filepath.Join(dir, "test.ipynb"))
		// Source should be the new content as a string array
		var source []string
		require.NoError(t, json.Unmarshal(nb.Cells[0].Source, &source))
		assert.Contains(t, source[0], "modified")
	})

	t.Run("edit cell out of bounds returns error", func(t *testing.T) {
		input := makeInput(t, map[string]any{
			"path":    "test.ipynb",
			"command": "edit_cell",
			"index":   99,
			"source":  "oops",
		})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "out of bounds")
	})
}

func TestNotebookEditDeleteCell(t *testing.T) {
	dir := t.TempDir()
	writeNotebook(t, dir, notebookWithCells())

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	t.Run("delete cell by index", func(t *testing.T) {
		input := makeInput(t, map[string]any{
			"path":    "test.ipynb",
			"command": "delete_cell",
			"index":   0,
		})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.False(t, result.IsError)

		nb := readNotebookFile(t, filepath.Join(dir, "test.ipynb"))
		require.Len(t, nb.Cells, 1)
		// Remaining cell should be the markdown one
		assert.Equal(t, "markdown", nb.Cells[0].CellType)
	})

	t.Run("delete cell out of bounds returns error", func(t *testing.T) {
		input := makeInput(t, map[string]any{
			"path":    "test.ipynb",
			"command": "delete_cell",
			"index":   99,
		})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestNotebookEditInsertCell(t *testing.T) {
	dir := t.TempDir()
	writeNotebook(t, dir, notebookWithCells())

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	input := makeInput(t, map[string]any{
		"path":      "test.ipynb",
		"command":   "insert_cell",
		"cell_type": "code",
		"index":     1,
		"source":    "x = 42",
	})
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	nb := readNotebookFile(t, filepath.Join(dir, "test.ipynb"))
	require.Len(t, nb.Cells, 3)
	// The inserted cell should be at index 1
	assert.Equal(t, "code", nb.Cells[1].CellType)
	var source []string
	require.NoError(t, json.Unmarshal(nb.Cells[1].Source, &source))
	assert.Contains(t, source[0], "x = 42")
}

func TestNotebookEditInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.ipynb")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	input := makeInput(t, map[string]any{
		"path":      "bad.ipynb",
		"command":   "add_cell",
		"cell_type": "code",
		"source":    "test",
	})
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestNotebookEditPreservesMetadata(t *testing.T) {
	dir := t.TempDir()
	writeNotebook(t, dir, notebookWithCells())

	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: dir}

	input := makeInput(t, map[string]any{
		"path":      "test.ipynb",
		"command":   "add_cell",
		"cell_type": "code",
		"source":    "new_cell",
	})
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	nb := readNotebookFile(t, filepath.Join(dir, "test.ipynb"))
	// Metadata should be preserved
	assert.Equal(t, 4, nb.NBFormat)
	assert.Equal(t, 5, nb.NBFormatMinor)
	// Kernelspec should be preserved in metadata
	var meta map[string]any
	require.NoError(t, json.Unmarshal(nb.Metadata, &meta))
	assert.Contains(t, meta, "kernelspec")
}

func TestNotebookEditCheckPermissions(t *testing.T) {
	tool := New()
	// With nil permission context, should return Ask (write tool)
	result, err := tool.CheckPermissions(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, tools.PermissionAsk, result)
}
