package todowrite

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

func TestTodoWriteTool_Name(t *testing.T) {
	tool := New()
	assert.Equal(t, "TodoWrite", tool.Name())
}

func TestTodoWriteTool_IsReadOnly(t *testing.T) {
	tool := New()
	assert.False(t, tool.IsReadOnly())
}

func TestTodoWriteTool_IsConcurrencySafe(t *testing.T) {
	tool := New()
	assert.False(t, tool.IsConcurrencySafe(nil))
}

func TestTodoWriteTool_Call_CreateTodos(t *testing.T) {
	tmpDir := t.TempDir()
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  tmpDir,
		ProjectRoot: tmpDir,
	}

	input := json.RawMessage(`{
		"todos": [
			{"content": "Write tests", "status": "pending", "activeForm": "Writing tests"},
			{"content": "Implement feature", "status": "in_progress", "activeForm": "Implementing feature"}
		]
	}`)

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "modified successfully")

	// Verify file was created
	todosPath := filepath.Join(tmpDir, ".claude", "todos.json")
	data, err := os.ReadFile(todosPath)
	require.NoError(t, err)

	var todos []TodoItem
	err = json.Unmarshal(data, &todos)
	require.NoError(t, err)
	assert.Len(t, todos, 2)
	assert.Equal(t, "Write tests", todos[0].Content)
	assert.Equal(t, "Writing tests", todos[0].ActiveForm)
}

func TestTodoWriteTool_Call_ReplacesFullList(t *testing.T) {
	tmpDir := t.TempDir()
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  tmpDir,
		ProjectRoot: tmpDir,
	}

	// Create initial todos
	input1 := json.RawMessage(`{
		"todos": [
			{"content": "Write tests", "status": "pending", "activeForm": "Writing tests"},
			{"content": "Implement feature", "status": "pending", "activeForm": "Implementing feature"}
		]
	}`)
	result, err := tool.Call(ctx, input1, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Replace the full list (not merge)
	input2 := json.RawMessage(`{
		"todos": [
			{"content": "Deploy", "status": "pending", "activeForm": "Deploying"}
		]
	}`)
	result, err = tool.Call(ctx, input2, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Verify file content -- should only have 1 item (full replacement)
	todosPath := filepath.Join(tmpDir, ".claude", "todos.json")
	data, err := os.ReadFile(todosPath)
	require.NoError(t, err)

	var todos []TodoItem
	err = json.Unmarshal(data, &todos)
	require.NoError(t, err)
	assert.Len(t, todos, 1)
	assert.Equal(t, "Deploy", todos[0].Content)
}

func TestTodoWriteTool_Call_AllCompletedClearsList(t *testing.T) {
	tmpDir := t.TempDir()
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{
		WorkingDir:  tmpDir,
		ProjectRoot: tmpDir,
	}

	// Mark all as completed
	input := json.RawMessage(`{
		"todos": [
			{"content": "Write tests", "status": "completed", "activeForm": "Writing tests"},
			{"content": "Deploy", "status": "completed", "activeForm": "Deploying"}
		]
	}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Verify file -- should be empty list
	todosPath := filepath.Join(tmpDir, ".claude", "todos.json")
	data, err := os.ReadFile(todosPath)
	require.NoError(t, err)

	var todos []TodoItem
	err = json.Unmarshal(data, &todos)
	require.NoError(t, err)
	assert.Len(t, todos, 0)
}

func TestTodoWriteTool_Call_EmptyTodosAllowed(t *testing.T) {
	tmpDir := t.TempDir()
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: tmpDir, ProjectRoot: tmpDir}

	// Empty array is a valid "clear all" operation
	input := json.RawMessage(`{"todos": []}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	// Empty array writes an empty file -- this is valid
	assert.False(t, result.IsError)
}

func TestTodoWriteTool_Call_InvalidStatus(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp", ProjectRoot: "/tmp"}

	input := json.RawMessage(`{
		"todos": [{"content": "Test", "status": "done", "activeForm": "Testing"}]
	}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "invalid status")
}

func TestTodoWriteTool_Call_MissingContent(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp", ProjectRoot: "/tmp"}

	input := json.RawMessage(`{
		"todos": [{"content": "", "status": "pending", "activeForm": "Testing"}]
	}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "content is required")
}

func TestTodoWriteTool_Call_MissingActiveForm(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp", ProjectRoot: "/tmp"}

	input := json.RawMessage(`{
		"todos": [{"content": "Test", "status": "pending", "activeForm": ""}]
	}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "activeForm is required")
}

func TestTodoWriteTool_Call_StatusCompleted(t *testing.T) {
	// Verify "completed" is accepted (not "done")
	tmpDir := t.TempDir()
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: tmpDir, ProjectRoot: tmpDir}

	input := json.RawMessage(`{
		"todos": [
			{"content": "Test", "status": "pending", "activeForm": "Testing"},
			{"content": "Done task", "status": "completed", "activeForm": "Done testing"}
		]
	}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
}
