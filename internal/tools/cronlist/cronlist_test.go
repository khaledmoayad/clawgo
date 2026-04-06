package cronlist

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/daemon"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCronListTool_Name(t *testing.T) {
	tool := New()
	assert.Equal(t, "CronList", tool.Name())
}

func TestCronListTool_IsReadOnly(t *testing.T) {
	tool := New()
	assert.True(t, tool.IsReadOnly())
}

func TestCronListTool_IsConcurrencySafe(t *testing.T) {
	tool := New()
	assert.True(t, tool.IsConcurrencySafe(nil))
}

func TestCronListTool_Call_Empty(t *testing.T) {
	daemon.ClearSessionTasks()
	defer daemon.ClearSessionTasks()

	tool := New()
	tuc := &tools.ToolUseContext{ProjectRoot: t.TempDir()}

	result, err := tool.Call(context.Background(), json.RawMessage(`{}`), tuc)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out output
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &out))
	assert.Empty(t, out.Jobs)
}

func TestCronListTool_Call_MixedTasks(t *testing.T) {
	daemon.ClearSessionTasks()
	defer daemon.ClearSessionTasks()

	dir := t.TempDir()
	now := daemon.NowMs()

	// File-backed task
	recurring := true
	fileTasks := []daemon.CronTask{
		{ID: "file0001", Cron: "*/5 * * * *", Prompt: "file task", CreatedAt: now, Recurring: &recurring},
	}
	require.NoError(t, daemon.WriteCronTasks(fileTasks, dir))

	// Session-scoped task
	_, err := daemon.AddCronTask("0 9 * * *", "session task", false, false, "")
	require.NoError(t, err)

	tool := New()
	tuc := &tools.ToolUseContext{ProjectRoot: dir}

	result, err := tool.Call(context.Background(), json.RawMessage(`{}`), tuc)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out output
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &out))
	require.Len(t, out.Jobs, 2)

	// File-backed task
	assert.Equal(t, "file0001", out.Jobs[0].ID)
	assert.Equal(t, "*/5 * * * *", out.Jobs[0].Cron)
	assert.Equal(t, "Every 5 minutes", out.Jobs[0].HumanSchedule)
	assert.Equal(t, "file task", out.Jobs[0].Prompt)
	assert.True(t, out.Jobs[0].Recurring)
	assert.True(t, out.Jobs[0].Durable)

	// Session-scoped task
	assert.Equal(t, "0 9 * * *", out.Jobs[1].Cron)
	assert.Equal(t, "Daily at 09:00", out.Jobs[1].HumanSchedule)
	assert.Equal(t, "session task", out.Jobs[1].Prompt)
	assert.False(t, out.Jobs[1].Recurring)
	assert.False(t, out.Jobs[1].Durable)
}
