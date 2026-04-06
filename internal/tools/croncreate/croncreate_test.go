package croncreate

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/khaledmoayad/clawgo/internal/daemon"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCronCreateTool_Name(t *testing.T) {
	tool := New()
	assert.Equal(t, "CronCreate", tool.Name())
}

func TestCronCreateTool_IsReadOnly(t *testing.T) {
	tool := New()
	assert.False(t, tool.IsReadOnly())
}

func TestCronCreateTool_Call_Success(t *testing.T) {
	dir := t.TempDir()
	daemon.ClearSessionTasks()
	defer daemon.ClearSessionTasks()

	tool := New()
	inp := `{"cron": "*/5 * * * *", "prompt": "check status"}`
	tuc := &tools.ToolUseContext{ProjectRoot: dir}

	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Parse output
	var out output
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &out))
	assert.Len(t, out.ID, 8)
	assert.Equal(t, "Every 5 minutes", out.HumanSchedule)
	assert.True(t, out.Recurring) // default
	assert.False(t, out.Durable) // default

	// Verify session task was created (non-durable)
	sessionTasks := daemon.GetSessionTasks()
	require.Len(t, sessionTasks, 1)
	assert.Equal(t, out.ID, sessionTasks[0].ID)
	assert.Equal(t, "check status", sessionTasks[0].Prompt)
}

func TestCronCreateTool_Call_Durable(t *testing.T) {
	dir := t.TempDir()
	tool := New()
	durable := true
	recurring := false

	inp, err := json.Marshal(map[string]any{
		"cron":      "0 9 * * 1",
		"prompt":    "weekly review",
		"recurring": recurring,
		"durable":   durable,
	})
	require.NoError(t, err)

	tuc := &tools.ToolUseContext{ProjectRoot: dir}
	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out output
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &out))
	assert.False(t, out.Recurring)
	assert.True(t, out.Durable)

	// Verify persisted to file
	tasks, err := daemon.ReadCronTasks(dir)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, out.ID, tasks[0].ID)
	assert.Equal(t, "weekly review", tasks[0].Prompt)
}

func TestCronCreateTool_Call_MissingCron(t *testing.T) {
	tool := New()
	inp := `{"prompt": "no cron"}`
	tuc := &tools.ToolUseContext{ProjectRoot: t.TempDir()}

	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestCronCreateTool_Call_MissingPrompt(t *testing.T) {
	tool := New()
	inp := `{"cron": "* * * * *"}`
	tuc := &tools.ToolUseContext{ProjectRoot: t.TempDir()}

	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestCronCreateTool_Call_InvalidCron(t *testing.T) {
	tool := New()
	inp := `{"cron": "not a cron", "prompt": "test"}`
	tuc := &tools.ToolUseContext{ProjectRoot: t.TempDir()}

	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "invalid cron expression")
}

func TestCronCreateTool_Call_MaxJobsLimit(t *testing.T) {
	dir := t.TempDir()
	daemon.ClearSessionTasks()
	defer daemon.ClearSessionTasks()

	// Fill up to MaxCronJobs with file-backed tasks
	tasks := make([]daemon.CronTask, daemon.MaxCronJobs)
	for i := range tasks {
		tasks[i] = daemon.CronTask{
			ID:        "fill" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + "zz",
			Cron:      "* * * * *",
			Prompt:    "filler",
			CreatedAt: daemon.NowMs(),
		}
	}
	require.NoError(t, daemon.WriteCronTasks(tasks, dir))

	tool := New()
	inp := `{"cron": "* * * * *", "prompt": "one too many"}`
	tuc := &tools.ToolUseContext{ProjectRoot: dir}

	result, err := tool.Call(context.Background(), json.RawMessage(inp), tuc)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "maximum")
}
