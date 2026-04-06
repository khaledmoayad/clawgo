package crondelete

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/daemon"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCronDeleteTool_Name(t *testing.T) {
	tool := New()
	assert.Equal(t, "CronDelete", tool.Name())
}

func TestCronDeleteTool_IsReadOnly(t *testing.T) {
	tool := New()
	assert.False(t, tool.IsReadOnly())
}

func TestCronDeleteTool_Call_DeleteFileBacked(t *testing.T) {
	dir := t.TempDir()
	now := daemon.NowMs()

	tasks := []daemon.CronTask{
		{ID: "del00001", Cron: "* * * * *", Prompt: "to delete", CreatedAt: now},
		{ID: "keep0001", Cron: "* * * * *", Prompt: "to keep", CreatedAt: now},
	}
	require.NoError(t, daemon.WriteCronTasks(tasks, dir))

	tool := New()
	inp := `{"id": "del00001"}`
	tuc := &tools.ToolUseContext{ProjectRoot: dir}

	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out output
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &out))
	assert.Equal(t, "del00001", out.ID)

	// Verify task was removed from file
	remaining, err := daemon.ReadCronTasks(dir)
	require.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, "keep0001", remaining[0].ID)
}

func TestCronDeleteTool_Call_DeleteSessionScoped(t *testing.T) {
	daemon.ClearSessionTasks()
	defer daemon.ClearSessionTasks()

	dir := t.TempDir()
	// Add a session task
	id, err := daemon.AddCronTask("* * * * *", "session task", false, false, "")
	require.NoError(t, err)

	tool := New()
	inp, err := json.Marshal(map[string]string{"id": id})
	require.NoError(t, err)

	tuc := &tools.ToolUseContext{ProjectRoot: dir}
	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Verify session task was removed
	sessionTasks := daemon.GetSessionTasks()
	assert.Empty(t, sessionTasks)
}

func TestCronDeleteTool_Call_NotFound(t *testing.T) {
	daemon.ClearSessionTasks()
	defer daemon.ClearSessionTasks()

	tool := New()
	inp := `{"id": "nonexist"}`
	tuc := &tools.ToolUseContext{ProjectRoot: t.TempDir()}

	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "no cron task found")
}

func TestCronDeleteTool_Call_MissingID(t *testing.T) {
	tool := New()
	inp := `{}`
	tuc := &tools.ToolUseContext{ProjectRoot: t.TempDir()}

	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
