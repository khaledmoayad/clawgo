package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTasks_NoFile(t *testing.T) {
	dir := t.TempDir()
	tasks, err := LoadTasks(dir)
	require.NoError(t, err)
	assert.Empty(t, tasks)
}

func TestLoadTasks_ValidFile(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Truncate(time.Second)

	tasks := []Task{
		{
			ID:         "task-1",
			CronExpr:   "*/5 * * * *",
			Command:    "echo hello",
			ProjectDir: "/tmp",
			NextRun:    now,
			Enabled:    true,
		},
		{
			ID:         "task-2",
			CronExpr:   "0 9 * * 1",
			Command:    "ls -la",
			ProjectDir: "/home",
			NextRun:    now.Add(time.Hour),
			LastRun:    &now,
			Enabled:    false,
		},
	}

	data, err := json.MarshalIndent(tasks, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, tasksFile), data, 0o644))

	loaded, err := LoadTasks(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)

	assert.Equal(t, "task-1", loaded[0].ID)
	assert.Equal(t, "*/5 * * * *", loaded[0].CronExpr)
	assert.Equal(t, "echo hello", loaded[0].Command)
	assert.True(t, loaded[0].Enabled)

	assert.Equal(t, "task-2", loaded[1].ID)
	assert.False(t, loaded[1].Enabled)
	assert.NotNil(t, loaded[1].LastRun)
}

func TestSaveTasks(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Truncate(time.Second)

	tasks := []Task{
		{
			ID:       "t1",
			CronExpr: "0 * * * *",
			Command:  "date",
			NextRun:  now,
			Enabled:  true,
		},
	}

	err := SaveTasks(dir, tasks)
	require.NoError(t, err)

	// Read back and verify JSON structure
	data, err := os.ReadFile(filepath.Join(dir, tasksFile))
	require.NoError(t, err)

	var loaded []Task
	require.NoError(t, json.Unmarshal(data, &loaded))
	require.Len(t, loaded, 1)
	assert.Equal(t, "t1", loaded[0].ID)
	assert.Equal(t, "date", loaded[0].Command)
}

func TestTask_IsDue(t *testing.T) {
	now := time.Now()

	// NextRun in the past -- should be due
	task := Task{NextRun: now.Add(-time.Minute)}
	assert.True(t, task.IsDue(now))

	// NextRun equal to now -- should be due
	task2 := Task{NextRun: now}
	assert.True(t, task2.IsDue(now))

	// NextRun in the future -- should not be due
	task3 := Task{NextRun: now.Add(time.Minute)}
	assert.False(t, task3.IsDue(now))
}

func TestTask_CalculateNextRun(t *testing.T) {
	// */5 * * * * from 10:03 should give 10:05
	task := Task{CronExpr: "*/5 * * * *"}
	from := time.Date(2026, 4, 4, 10, 3, 0, 0, time.UTC)
	next := task.CalculateNextRun(from)

	assert.Equal(t, 2026, next.Year())
	assert.Equal(t, time.Month(4), next.Month())
	assert.Equal(t, 4, next.Day())
	assert.Equal(t, 10, next.Hour())
	assert.Equal(t, 5, next.Minute())
}

func TestTask_CalculateNextRun_HourlyAtZero(t *testing.T) {
	// 0 * * * * from 10:30 should give 11:00
	task := Task{CronExpr: "0 * * * *"}
	from := time.Date(2026, 4, 4, 10, 30, 0, 0, time.UTC)
	next := task.CalculateNextRun(from)

	assert.Equal(t, 11, next.Hour())
	assert.Equal(t, 0, next.Minute())
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

	// Cleanup
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

	// Lock file should be removed
	_, err = os.Stat(filepath.Join(dir, lockFileName))
	assert.True(t, os.IsNotExist(err))
}

func TestLock_StaleLock(t *testing.T) {
	dir := t.TempDir()

	// Write a lock file with a PID that does not exist
	// Use a very high PID that is unlikely to be in use
	stalePID := 99999999
	err := os.WriteFile(filepath.Join(dir, lockFileName), []byte(fmt.Sprintf("%d", stalePID)), 0o644)
	require.NoError(t, err)

	lock := NewLock(dir)
	acquired, err := lock.TryAcquire()
	require.NoError(t, err)
	assert.True(t, acquired, "should acquire lock when existing PID is stale")

	require.NoError(t, lock.Release())
}

func TestScheduler_CheckLoop(t *testing.T) {
	dir := t.TempDir()

	// Create a task that is already due
	marker := filepath.Join(dir, "executed.txt")
	tasks := []Task{
		{
			ID:         "check-task",
			CronExpr:   "* * * * *",
			Command:    fmt.Sprintf("touch %s", marker),
			ProjectDir: dir,
			NextRun:    time.Now().Add(-time.Minute), // Already due
			Enabled:    true,
		},
	}

	require.NoError(t, SaveTasks(dir, tasks))

	scheduler := NewScheduler(dir)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Start scheduler in background
	done := make(chan error, 1)
	go func() {
		done <- scheduler.Start(ctx)
	}()

	// Wait for the marker file to appear (task executed)
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for task execution")
		default:
			if _, err := os.Stat(marker); err == nil {
				// Task was executed -- success
				cancel()
				<-done
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}
