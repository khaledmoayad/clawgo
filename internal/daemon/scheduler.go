// Package daemon implements the background daemon worker system for ClawGo.
// The daemon runs a cron scheduler that checks a task file every second
// and fires due tasks with lock-based single-owner execution.
package daemon

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

const checkInterval = 1 * time.Second

// Scheduler runs cron tasks on a 1-second check loop.
// It uses a file-based lock to ensure only one scheduler instance runs at a time.
type Scheduler struct {
	configDir string
	lock      *Lock
	tasks     []Task
	mu        sync.Mutex
}

// NewScheduler creates a new scheduler targeting the given config directory.
func NewScheduler(configDir string) *Scheduler {
	return &Scheduler{
		configDir: configDir,
		lock:      NewLock(configDir),
	}
}

// Start acquires the scheduler lock, loads tasks, and begins the check loop.
// It blocks until the context is cancelled.
func (s *Scheduler) Start(ctx context.Context) error {
	acquired, err := s.lock.TryAcquire()
	if err != nil {
		return fmt.Errorf("acquire scheduler lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("another scheduler instance is running")
	}

	// Load initial tasks from disk
	tasks, err := LoadTasks(s.configDir)
	if err != nil {
		s.lock.Release()
		return fmt.Errorf("load tasks: %w", err)
	}
	s.mu.Lock()
	s.tasks = tasks
	s.mu.Unlock()

	log.Printf("daemon: scheduler started with %d tasks", len(tasks))

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.Stop()
			return ctx.Err()
		case <-ticker.C:
			s.checkAndExecute(ctx)
		}
	}
}

// Stop releases the scheduler lock.
func (s *Scheduler) Stop() {
	if err := s.lock.Release(); err != nil {
		log.Printf("daemon: failed to release lock: %v", err)
	}
}

// checkAndExecute iterates through tasks, executes due ones, and saves state.
func (s *Scheduler) checkAndExecute(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	changed := false

	for i := range s.tasks {
		task := &s.tasks[i]
		if !task.Enabled || !task.IsDue(now) {
			continue
		}

		s.executeTask(ctx, task)
		changed = true
	}

	if changed {
		if err := SaveTasks(s.configDir, s.tasks); err != nil {
			log.Printf("daemon: failed to save tasks: %v", err)
		}
	}
}

// executeTask runs a single task's command and updates its timing fields.
func (s *Scheduler) executeTask(ctx context.Context, task *Task) {
	log.Printf("daemon: executing task %s: %s", task.ID, task.Command)

	cmd := exec.CommandContext(ctx, "sh", "-c", task.Command)
	cmd.Dir = task.ProjectDir

	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("daemon: task %s failed: %v\noutput: %s", task.ID, err, string(output))
	}

	now := time.Now()
	task.LastRun = &now
	task.NextRun = task.CalculateNextRun(now)
}

// Tasks returns a copy of the current task list.
func (s *Scheduler) Tasks() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]Task, len(s.tasks))
	copy(result, s.tasks)
	return result
}
