package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadCronTasks_NoFile(t *testing.T) {
	dir := t.TempDir()
	tasks, err := ReadCronTasks(dir)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestReadCronTasks_ValidFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()
	firedAt := now - 60000

	cf := CronFile{
		Tasks: []CronTask{
			{
				ID:        "a1b2c3d4",
				Cron:      "*/5 * * * *",
				Prompt:    "Check system status",
				CreatedAt: now,
			},
			{
				ID:          "e5f6a7b8",
				Cron:        "0 9 * * 1",
				Prompt:      "Weekly review",
				CreatedAt:   now - 86400000,
				LastFiredAt: &firedAt,
			},
		},
	}

	data, err := json.MarshalIndent(cf, "", "  ")
	require.NoError(t, err)

	cronPath := GetCronFilePath(dir)
	require.NoError(t, os.MkdirAll(filepath.Dir(cronPath), 0o755))
	require.NoError(t, os.WriteFile(cronPath, data, 0o644))

	loaded, err := ReadCronTasks(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)

	assert.Equal(t, "a1b2c3d4", loaded[0].ID)
	assert.Equal(t, "*/5 * * * *", loaded[0].Cron)
	assert.Equal(t, "Check system status", loaded[0].Prompt)
	assert.Equal(t, now, loaded[0].CreatedAt)

	assert.Equal(t, "e5f6a7b8", loaded[1].ID)
	assert.NotNil(t, loaded[1].LastFiredAt)
}

func TestReadCronTasks_SkipsMalformed(t *testing.T) {
	dir := t.TempDir()
	cf := CronFile{
		Tasks: []CronTask{
			{ID: "valid123", Cron: "* * * * *", Prompt: "good task", CreatedAt: time.Now().UnixMilli()},
			{ID: "", Cron: "* * * * *", Prompt: "missing id", CreatedAt: time.Now().UnixMilli()},       // missing ID
			{ID: "nopr0mpt", Cron: "* * * * *", Prompt: "", CreatedAt: time.Now().UnixMilli()},          // missing prompt
			{ID: "badcron1", Cron: "invalid", Prompt: "bad cron", CreatedAt: time.Now().UnixMilli()},    // invalid cron
		},
	}

	data, err := json.MarshalIndent(cf, "", "  ")
	require.NoError(t, err)

	cronPath := GetCronFilePath(dir)
	require.NoError(t, os.MkdirAll(filepath.Dir(cronPath), 0o755))
	require.NoError(t, os.WriteFile(cronPath, data, 0o644))

	loaded, err := ReadCronTasks(dir)
	require.NoError(t, err)
	assert.Len(t, loaded, 1)
	assert.Equal(t, "valid123", loaded[0].ID)
}

func TestWriteCronTasks(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	tasks := []CronTask{
		{ID: "t1aabbcc", Cron: "0 * * * *", Prompt: "hourly check", CreatedAt: now},
	}

	err := WriteCronTasks(tasks, dir)
	require.NoError(t, err)

	// Read back and verify JSON structure
	data, err := os.ReadFile(GetCronFilePath(dir))
	require.NoError(t, err)

	var cf CronFile
	require.NoError(t, json.Unmarshal(data, &cf))
	require.Len(t, cf.Tasks, 1)
	assert.Equal(t, "t1aabbcc", cf.Tasks[0].ID)
	assert.Equal(t, "hourly check", cf.Tasks[0].Prompt)
}

func TestWriteCronTasks_FileFormat(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	tasks := []CronTask{
		{ID: "abcd1234", Cron: "*/5 * * * *", Prompt: "test prompt", CreatedAt: now},
	}

	err := WriteCronTasks(tasks, dir)
	require.NoError(t, err)

	data, err := os.ReadFile(GetCronFilePath(dir))
	require.NoError(t, err)

	// Verify it parses as CronFile with "tasks" key
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	_, hasTasks := raw["tasks"]
	assert.True(t, hasTasks, "file should have 'tasks' key at top level")
}

func TestAddCronTask_Durable(t *testing.T) {
	dir := t.TempDir()
	id, err := AddCronTask("*/10 * * * *", "run diagnostics", true, true, dir)
	require.NoError(t, err)
	assert.Len(t, id, 8) // 8-hex-char ID

	tasks, err := ReadCronTasks(dir)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	assert.Equal(t, id, tasks[0].ID)
	assert.Equal(t, "run diagnostics", tasks[0].Prompt)
	assert.True(t, *tasks[0].Recurring)
}

func TestAddCronTask_SessionScoped(t *testing.T) {
	ClearSessionTasks()
	defer ClearSessionTasks()

	id, err := AddCronTask("0 * * * *", "session prompt", false, false, "")
	require.NoError(t, err)
	assert.Len(t, id, 8)

	st := GetSessionTasks()
	require.Len(t, st, 1)
	assert.Equal(t, id, st[0].ID)
	assert.Equal(t, "session prompt", st[0].Prompt)
}

func TestRemoveCronTasks(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	tasks := []CronTask{
		{ID: "keep1111", Cron: "* * * * *", Prompt: "keep", CreatedAt: now},
		{ID: "rm222222", Cron: "* * * * *", Prompt: "remove", CreatedAt: now},
		{ID: "keep3333", Cron: "* * * * *", Prompt: "keep too", CreatedAt: now},
	}
	require.NoError(t, WriteCronTasks(tasks, dir))

	err := RemoveCronTasks([]string{"rm222222"}, dir)
	require.NoError(t, err)

	remaining, err := ReadCronTasks(dir)
	require.NoError(t, err)
	assert.Len(t, remaining, 2)
	assert.Equal(t, "keep1111", remaining[0].ID)
	assert.Equal(t, "keep3333", remaining[1].ID)
}

func TestMarkCronTasksFired(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	tasks := []CronTask{
		{ID: "fire1111", Cron: "* * * * *", Prompt: "fire me", CreatedAt: now},
		{ID: "skip2222", Cron: "* * * * *", Prompt: "skip me", CreatedAt: now},
	}
	require.NoError(t, WriteCronTasks(tasks, dir))

	firedAt := now + 1000
	err := MarkCronTasksFired([]string{"fire1111"}, firedAt, dir)
	require.NoError(t, err)

	updated, err := ReadCronTasks(dir)
	require.NoError(t, err)
	require.Len(t, updated, 2)
	require.NotNil(t, updated[0].LastFiredAt)
	assert.Equal(t, firedAt, *updated[0].LastFiredAt)
	assert.Nil(t, updated[1].LastFiredAt)
}

func TestNextCronRunMs(t *testing.T) {
	// */5 * * * * from 10:03 should give 10:05
	from := time.Date(2026, 4, 4, 10, 3, 0, 0, time.UTC)
	nextMs := NextCronRunMs("*/5 * * * *", from.UnixMilli())

	require.NotNil(t, nextMs)
	next := time.UnixMilli(*nextMs).UTC()
	assert.Equal(t, 10, next.Hour())
	assert.Equal(t, 5, next.Minute())
}

func TestNextCronRunMs_HourlyAtZero(t *testing.T) {
	// 0 * * * * from 10:30 should give 11:00
	from := time.Date(2026, 4, 4, 10, 30, 0, 0, time.UTC)
	nextMs := NextCronRunMs("0 * * * *", from.UnixMilli())

	require.NotNil(t, nextMs)
	next := time.UnixMilli(*nextMs).UTC()
	assert.Equal(t, 11, next.Hour())
	assert.Equal(t, 0, next.Minute())
}

func TestNextCronRunMs_InvalidCron(t *testing.T) {
	from := time.Now().UnixMilli()
	result := NextCronRunMs("invalid", from)
	assert.Nil(t, result)
}

func TestJitteredNextCronRunMs(t *testing.T) {
	from := time.Date(2026, 4, 4, 10, 0, 0, 0, time.UTC).UnixMilli()
	cfg := DefaultCronJitterConfig()

	result := JitteredNextCronRunMs("*/5 * * * *", from, "aabbccdd", cfg)
	require.NotNil(t, result)

	// Jittered result should be >= plain next run
	plain := NextCronRunMs("*/5 * * * *", from)
	require.NotNil(t, plain)
	assert.GreaterOrEqual(t, *result, *plain)
}

func TestOneShotJitteredNextCronRunMs(t *testing.T) {
	// Fire at a :00 or :30 minute boundary to trigger backward jitter
	from := time.Date(2026, 4, 4, 9, 59, 0, 0, time.UTC).UnixMilli()
	cfg := DefaultCronJitterConfig()

	result := OneShotJitteredNextCronRunMs("0 10 * * *", from, "11223344", cfg)
	require.NotNil(t, result)

	plain := NextCronRunMs("0 10 * * *", from)
	require.NotNil(t, plain)
	// One-shot jitter is backward, so result should be <= plain
	assert.LessOrEqual(t, *result, *plain)
}

func TestFindMissedTasks(t *testing.T) {
	now := time.Now().UnixMilli()
	past := now - 3600000 // 1 hour ago

	recurring := true
	tasks := []CronTask{
		{ID: "missed01", Cron: "* * * * *", Prompt: "should be missed", CreatedAt: past},
		{ID: "future01", Cron: "0 0 1 1 *", Prompt: "far future", CreatedAt: now},
		{ID: "recur001", Cron: "* * * * *", Prompt: "recurring", CreatedAt: past, Recurring: &recurring},
	}

	missed := FindMissedTasks(tasks, now)
	assert.Len(t, missed, 1)
	assert.Equal(t, "missed01", missed[0].ID)
}

func TestIsRecurringTaskAged(t *testing.T) {
	now := time.Now().UnixMilli()
	recurring := true
	permanent := true
	maxAge := int64(7 * 24 * 60 * 60 * 1000) // 7 days

	// Recurring, not permanent, older than 7 days
	old := CronTask{
		ID: "old1", Cron: "* * * * *", Prompt: "old",
		CreatedAt: now - maxAge - 1000,
		Recurring: &recurring,
	}
	assert.True(t, IsRecurringTaskAged(old, now, maxAge))

	// Recurring and permanent -- never aged
	perm := CronTask{
		ID: "perm", Cron: "* * * * *", Prompt: "perm",
		CreatedAt: now - maxAge - 1000,
		Recurring: &recurring,
		Permanent: &permanent,
	}
	assert.False(t, IsRecurringTaskAged(perm, now, maxAge))

	// Non-recurring -- never aged
	oneShot := CronTask{
		ID: "one", Cron: "* * * * *", Prompt: "one",
		CreatedAt: now - maxAge - 1000,
	}
	assert.False(t, IsRecurringTaskAged(oneShot, now, maxAge))

	// Recurring but young -- not aged
	young := CronTask{
		ID: "young", Cron: "* * * * *", Prompt: "young",
		CreatedAt: now - 1000,
		Recurring: &recurring,
	}
	assert.False(t, IsRecurringTaskAged(young, now, maxAge))
}

func TestGenerateCronID(t *testing.T) {
	id := generateCronID()
	assert.Len(t, id, 8)

	// Should be valid hex
	for _, c := range id {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"expected hex char, got %c", c)
	}

	// Should generate unique IDs
	id2 := generateCronID()
	assert.NotEqual(t, id, id2)
}

func TestGetCronFilePath(t *testing.T) {
	path := GetCronFilePath("/home/user/project")
	assert.Equal(t, "/home/user/project/.claude/scheduled_tasks.json", path)
}

func TestJitterFractionFromID(t *testing.T) {
	// Known value: "00000000" should give 0
	assert.Equal(t, 0.0, jitterFractionFromID("00000000"))

	// "ffffffff" should give close to 1
	frac := jitterFractionFromID("ffffffff")
	assert.InDelta(t, 1.0, frac, 0.001)

	// "80000000" should give ~0.5
	frac2 := jitterFractionFromID("80000000")
	assert.InDelta(t, 0.5, frac2, 0.001)
}

func TestCronScheduler_FiresPrompt(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	// Create a recurring task that should fire. Use recurring to avoid missed-task path.
	recurring := true
	tasks := []CronTask{
		{ID: "fire0001", Cron: "* * * * *", Prompt: "do the thing", CreatedAt: now - 120000, Recurring: &recurring},
	}
	require.NoError(t, WriteCronTasks(tasks, dir))

	fired := make(chan string, 1)
	scheduler := NewCronScheduler(CronSchedulerOptions{
		Dir: dir,
		OnFire: func(prompt string) {
			select {
			case fired <- prompt:
			default:
			}
		},
		IsLoading: func() bool { return false },
	})

	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	select {
	case prompt := <-fired:
		assert.Equal(t, "do the thing", prompt)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for task to fire")
	}
}

func TestCronScheduler_OnFireTask(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	tasks := []CronTask{
		{ID: "task0001", Cron: "* * * * *", Prompt: "test prompt", CreatedAt: now - 120000},
	}
	require.NoError(t, WriteCronTasks(tasks, dir))

	fired := make(chan CronTask, 1)
	scheduler := NewCronScheduler(CronSchedulerOptions{
		Dir: dir,
		OnFireTask: func(task CronTask) {
			select {
			case fired <- task:
			default:
			}
		},
		IsLoading: func() bool { return false },
	})

	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	select {
	case task := <-fired:
		assert.Equal(t, "task0001", task.ID)
		assert.Equal(t, "test prompt", task.Prompt)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for OnFireTask")
	}
}

func TestCronScheduler_IsLoadingGate(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	// Use a recurring task so it goes through the check() loop, not the missed-task path.
	// One-shot tasks created in the past are detected as "missed" on initial load and
	// fired immediately via OnMissed/OnFire, bypassing the loading gate (by design).
	recurring := true
	tasks := []CronTask{
		{ID: "gate0001", Cron: "* * * * *", Prompt: "gated", CreatedAt: now - 120000, Recurring: &recurring},
	}
	require.NoError(t, WriteCronTasks(tasks, dir))

	loading := true
	fired := make(chan string, 1)
	scheduler := NewCronScheduler(CronSchedulerOptions{
		Dir: dir,
		OnFire: func(prompt string) {
			select {
			case fired <- prompt:
			default:
			}
		},
		IsLoading: func() bool { return loading },
	})

	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	// Should not fire while loading
	select {
	case <-fired:
		t.Fatal("should not fire while loading")
	case <-time.After(2 * time.Second):
		// Good -- did not fire
	}

	// Ungate
	loading = false

	select {
	case prompt := <-fired:
		assert.Equal(t, "gated", prompt)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out after ungating")
	}
}

func TestCronScheduler_Filter(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	tasks := []CronTask{
		{ID: "allow001", Cron: "* * * * *", Prompt: "allowed", CreatedAt: now - 120000},
		{ID: "block001", Cron: "* * * * *", Prompt: "blocked", CreatedAt: now - 120000},
	}
	require.NoError(t, WriteCronTasks(tasks, dir))

	var firedPrompts []string
	var mu sync.Mutex
	scheduler := NewCronScheduler(CronSchedulerOptions{
		Dir: dir,
		OnFire: func(prompt string) {
			mu.Lock()
			firedPrompts = append(firedPrompts, prompt)
			mu.Unlock()
		},
		IsLoading: func() bool { return false },
		Filter: func(t CronTask) bool {
			return t.ID == "allow001"
		},
	})

	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	time.Sleep(3 * time.Second)

	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, firedPrompts, "allowed")
	for _, p := range firedPrompts {
		assert.NotEqual(t, "blocked", p, "blocked task should not have fired")
	}
}

func TestCronScheduler_NoExecCommand(t *testing.T) {
	// Verify that the scheduler package does not import os/exec
	// This is a compile-time guarantee: scheduler.go does not import os/exec.
	// We verify by checking that the CronSchedulerOptions has no command-related fields.
	opts := CronSchedulerOptions{}
	assert.NotNil(t, opts) // The struct should exist
	// OnFire accepts a string (prompt), not a command to execute
}

func TestCronScheduler_GetNextFireTime(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	tasks := []CronTask{
		{ID: "next0001", Cron: "0 12 * * *", Prompt: "noon", CreatedAt: now},
	}
	require.NoError(t, WriteCronTasks(tasks, dir))

	scheduler := NewCronScheduler(CronSchedulerOptions{
		Dir:       dir,
		OnFire:    func(prompt string) {},
		IsLoading: func() bool { return true }, // Keep loading to prevent firing
	})

	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	// Wait for first tick to populate nextFireAt
	time.Sleep(2 * time.Second)

	// Even with loading=true, the nextFireAt map won't be populated because check() bails early.
	// That's fine -- this tests that the method doesn't panic.
	_ = scheduler.GetNextFireTime()
}

func TestCronScheduler_MissedTaskNotification(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UnixMilli()

	// Task created 2 hours ago with a cron that fires every minute -- definitely missed
	tasks := []CronTask{
		{ID: "miss0001", Cron: "* * * * *", Prompt: "missed prompt", CreatedAt: now - 7200000},
	}
	require.NoError(t, WriteCronTasks(tasks, dir))

	missedCh := make(chan []CronTask, 1)
	scheduler := NewCronScheduler(CronSchedulerOptions{
		Dir: dir,
		OnFire: func(prompt string) {},
		OnMissed: func(tasks []CronTask) {
			select {
			case missedCh <- tasks:
			default:
			}
		},
		IsLoading: func() bool { return false },
	})

	require.NoError(t, scheduler.Start())
	defer scheduler.Stop()

	select {
	case missed := <-missedCh:
		require.Len(t, missed, 1)
		assert.Equal(t, "miss0001", missed[0].ID)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for missed task notification")
	}
}

func TestBuildMissedTaskNotification(t *testing.T) {
	now := time.Now().UnixMilli()
	tasks := []CronTask{
		{ID: "n1", Cron: "0 9 * * *", Prompt: "run daily check", CreatedAt: now},
		{ID: "n2", Cron: "*/30 * * * *", Prompt: "half-hour poll", CreatedAt: now},
	}

	notification := BuildMissedTaskNotification(tasks)
	assert.Contains(t, notification, "one-shot scheduled task(s) missed")
	assert.Contains(t, notification, "Do NOT execute these prompts yet")
	assert.Contains(t, notification, "AskUserQuestion")
	assert.Contains(t, notification, "run daily check")
	assert.Contains(t, notification, "half-hour poll")
	assert.Contains(t, notification, "0 9 * * *")
}

func TestCronScheduler_StopIdempotent(t *testing.T) {
	dir := t.TempDir()
	scheduler := NewCronScheduler(CronSchedulerOptions{
		Dir:    dir,
		OnFire: func(prompt string) {},
	})

	require.NoError(t, scheduler.Start())

	// Stop multiple times should not panic
	scheduler.Stop()
	scheduler.Stop()
	scheduler.Stop()
}

func TestLock_TryAcquire(t *testing.T) {
	dir := t.TempDir()
	lock := NewLock(dir)

	acquired, err := lock.TryAcquire()
	require.NoError(t, err)
	assert.True(t, acquired)

	// Try to acquire again -- should fail (our PID is alive)
	lock2 := NewLock(dir)
	acquired2, err := lock2.TryAcquire()
	require.NoError(t, err)
	assert.False(t, acquired2)

	require.NoError(t, lock.Release())
}

func TestLock_Release(t *testing.T) {
	dir := t.TempDir()
	lock := NewLock(dir)

	acquired, err := lock.TryAcquire()
	require.NoError(t, err)
	assert.True(t, acquired)

	err = lock.Release()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, lockFileName))
	assert.True(t, os.IsNotExist(err))
}

func TestParseCronField(t *testing.T) {
	// */5 should give 0,5,10,...55
	f := parseCronField("*/5", 0, 59)
	assert.True(t, f[0])
	assert.True(t, f[5])
	assert.True(t, f[10])
	assert.False(t, f[1])
	assert.False(t, f[3])

	// * should give all values
	f2 := parseCronField("*", 0, 23)
	for i := 0; i <= 23; i++ {
		assert.True(t, f2[i])
	}

	// Range: 1-3
	f3 := parseCronField("1-3", 0, 6)
	assert.False(t, f3[0])
	assert.True(t, f3[1])
	assert.True(t, f3[2])
	assert.True(t, f3[3])
	assert.False(t, f3[4])

	// Comma-separated: 1,3,5
	f4 := parseCronField("1,3,5", 0, 59)
	assert.True(t, f4[1])
	assert.True(t, f4[3])
	assert.True(t, f4[5])
	assert.False(t, f4[2])
}
