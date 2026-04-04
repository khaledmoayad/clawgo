package websearch

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSearchToolMetadata(t *testing.T) {
	tool := New()

	t.Run("Name returns WebSearch", func(t *testing.T) {
		assert.Equal(t, "WebSearch", tool.Name())
	})

	t.Run("IsReadOnly returns true", func(t *testing.T) {
		assert.True(t, tool.IsReadOnly())
	})

	t.Run("IsConcurrencySafe returns true", func(t *testing.T) {
		assert.True(t, tool.IsConcurrencySafe(nil))
	})

	t.Run("Description is non-empty", func(t *testing.T) {
		assert.NotEmpty(t, tool.Description())
	})

	t.Run("InputSchema is valid JSON with query property", func(t *testing.T) {
		var schema map[string]any
		err := json.Unmarshal(tool.InputSchema(), &schema)
		assert.NoError(t, err)
		props, ok := schema["properties"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, props, "query")
	})

	t.Run("IsServerSide returns true", func(t *testing.T) {
		assert.True(t, tool.IsServerSide)
	})
}

func TestWebSearchToolCall(t *testing.T) {
	tool := New()
	ctx := context.Background()
	toolCtx := &tools.ToolUseContext{WorkingDir: "/tmp"}

	t.Run("returns server-side indicator", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{"query": "test search"})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.False(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "server-side")
		// Metadata should indicate server-side tool
		assert.Equal(t, true, result.Metadata["server_tool"])
	})

	t.Run("requires query field", func(t *testing.T) {
		input, _ := json.Marshal(map[string]string{})
		result, err := tool.Call(ctx, input, toolCtx)
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestWebSearchToolCheckPermissions(t *testing.T) {
	tool := New()
	result, err := tool.CheckPermissions(context.Background(), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, tools.PermissionAllow, result)
}
