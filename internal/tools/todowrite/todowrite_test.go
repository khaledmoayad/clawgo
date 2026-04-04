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
			{"id": "1", "content": "Write tests", "status": "pending", "priority": "high"},
			{"id": "2", "content": "Implement feature", "status": "in_progress"}
		]
	}`)

	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "2 added")
	assert.Contains(t, result.Content[0].Text, "0 updated")
	assert.Contains(t, result.Content[0].Text, "2 total")

	// Verify file was created
	todosPath := filepath.Join(tmpDir, ".claude", "todos.json")
	data, err := os.ReadFile(todosPath)
	require.NoError(t, err)

	var todos []TodoItem
	err = json.Unmarshal(data, &todos)
	require.NoError(t, err)
	assert.Len(t, todos, 2)
}

func TestTodoWriteTool_Call_UpdateTodos(t *testing.T) {
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
			{"id": "1", "content": "Write tests", "status": "pending"},
			{"id": "2", "content": "Implement feature", "status": "pending"}
		]
	}`)
	result, err := tool.Call(ctx, input1, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	// Update one todo
	input2 := json.RawMessage(`{
		"todos": [
			{"id": "1", "content": "Write tests", "status": "done"},
			{"id": "3", "content": "Deploy", "status": "pending"}
		]
	}`)
	result, err = tool.Call(ctx, input2, toolCtx)
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "1 added")
	assert.Contains(t, result.Content[0].Text, "1 updated")
	assert.Contains(t, result.Content[0].Text, "3 total")

	// Verify file content
	todosPath := filepath.Join(tmpDir, ".claude", "todos.json")
	data, err := os.ReadFile(todosPath)
	require.NoError(t, err)

	var todos []TodoItem
	err = json.Unmarshal(data, &todos)
	require.NoError(t, err)
	assert.Len(t, todos, 3)
}

func TestTodoWriteTool_Call_EmptyTodos(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp", ProjectRoot: "/tmp"}

	input := json.RawMessage(`{"todos": []}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "todos")
}

func TestTodoWriteTool_Call_InvalidStatus(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp", ProjectRoot: "/tmp"}

	input := json.RawMessage(`{
		"todos": [{"id": "1", "content": "Test", "status": "invalid"}]
	}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "invalid status")
}

func TestTodoWriteTool_Call_MissingID(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp", ProjectRoot: "/tmp"}

	input := json.RawMessage(`{
		"todos": [{"id": "", "content": "Test", "status": "pending"}]
	}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "id is required")
}

func TestTodoWriteTool_Call_MissingContent(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp", ProjectRoot: "/tmp"}

	input := json.RawMessage(`{
		"todos": [{"id": "1", "content": "", "status": "pending"}]
	}`)
	result, err := tool.Call(ctx, input, toolCtx)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "content is required")
}
