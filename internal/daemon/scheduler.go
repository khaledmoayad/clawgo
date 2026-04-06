// Package daemon implements the background daemon worker system for ClawGo.
// The daemon runs a cron scheduler that checks a task file every second
// and fires due tasks with lock-based single-owner execution.
//
// The scheduler fires prompts into the query loop via OnFire(prompt),
// never executing shell commands directly. This matches the TS cronScheduler.ts.
package daemon

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const checkInterval = 1 * time.Second

// CronSchedulerOptions configures the cron scheduler behavior.
// Matches TS CronSchedulerOptions.
type CronSchedulerOptions struct {
	// OnFire is called when a task fires -- injects prompt into query loop.
	OnFire func(prompt string)

	// IsLoading returns true while the system is loading; firing is deferred.
	IsLoading func() bool

	// AssistantMode bypasses the IsLoading gate.
	AssistantMode bool

	// OnFireTask receives the full task on fire. When set, OnFire is not called.
	OnFireTask func(task CronTask)

	// OnMissed receives missed one-shot tasks on initial load.
	OnMissed func(tasks []CronTask)

	// Dir is the project directory.
	Dir string

	// LockIdentity is the owner key for the lock file.
	LockIdentity string

	// GetJitterConfig returns the current jitter config per tick.
	// If nil, DefaultCronJitterConfig() is used.
	GetJitterConfig func() CronJitterConfig

	// IsKilled is a killswitch polled per tick; returns true to stop.
	IsKilled func() bool

	// Filter is a per-task gate. If non-nil and returns false, the task is skipped.
	Filter func(CronTask) bool
}

// CronScheduler manages cron task scheduling with file watching and jitter.
type CronScheduler struct {
	options CronSchedulerOptions

	// tasks from the file (file-backed)
	tasks []CronTask

	// nextFireAt stores computed next-fire timestamps per task ID (epoch ms)
	nextFireAt map[string]int64

	// missedAsked tracks task IDs for which we've already surfaced missed notifications
	missedAsked map[string]bool

	// inFlight tracks task IDs currently being processed (async remove)
	inFlight map[string]bool

	stopped bool
	isOwner bool // whether we hold the scheduler lock

	lock    *Lock
	watcher *fsnotify.Watcher
	ticker  *time.Ticker
	done    chan struct{}
	mu      sync.Mutex
}

// NewCronScheduler creates a new scheduler with the given options.
func NewCronScheduler(opts CronSchedulerOptions) *CronScheduler {
	return &CronScheduler{
		options:     opts,
		nextFireAt:  make(map[string]int64),
		missedAsked: make(map[string]bool),
		inFlight:    make(map[string]bool),
		done:        make(chan struct{}),
	}
}

// Start initializes and runs the scheduler.
// It acquires the lock, loads tasks, detects missed tasks, starts file watching,
// and begins the 1-second check loop. Blocks until Stop is called.
func (s *CronScheduler) Start() error {
	dir := s.options.Dir
	if dir == "" {
		return fmt.Errorf("scheduler: Dir is required")
	}

	// Acquire scheduler lock
	lockDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return fmt.Errorf("create lock dir: %w", err)
	}
	s.lock = NewLock(lockDir)

	acquired, err := s.lock.TryAcquire()
	if err != nil {
		return fmt.Errorf("acquire scheduler lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("another scheduler instance is running")
	}
	s.isOwner = true

	// Load initial tasks
	if err := s.load(true); err != nil {
		s.lock.Release()
		return fmt.Errorf("initial load: %w", err)
	}

	// Start file watcher
	if err := s.startWatcher(); err != nil {
		log.Printf("daemon: failed to start file watcher: %v (continuing without)", err)
	}

	// Start check loop
	s.ticker = time.NewTicker(checkInterval)
	go s.loop()

	return nil
}

// Stop halts the scheduler, stops timers, closes the watcher, and releases the lock.
func (s *CronScheduler) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	close(s.done)

	if s.ticker != nil {
		s.ticker.Stop()
	}

	if s.watcher != nil {
		s.watcher.Close()
	}

	if s.lock != nil && s.isOwner {
		if err := s.lock.Release(); err != nil {
			log.Printf("daemon: failed to release lock: %v", err)
		}
	}
}

// loop runs the 1-second check loop until stopped.
func (s *CronScheduler) loop() {
	for {
		select {
		case <-s.done:
			return
		case <-s.ticker.C:
			s.check()
		}
	}
}

// load reads tasks from the file and optionally surfaces missed tasks.
func (s *CronScheduler) load(initial bool) error {
	tasks, err := ReadCronTasks(s.options.Dir)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.tasks = tasks
	s.mu.Unlock()

	if initial {
		s.surfaceMissedTasks(tasks)
	}

	log.Printf("daemon: loaded %d tasks", len(tasks))
	return nil
}

// surfaceMissedTasks detects and reports missed one-shot tasks on initial load.
func (s *CronScheduler) surfaceMissedTasks(tasks []CronTask) {
	now := nowMs()
	missed := FindMissedTasks(tasks, now)
	if len(missed) == 0 {
		return
	}

	// Filter out already-asked tasks
	var newMissed []CronTask
	for _, t := range missed {
		if !s.missedAsked[t.ID] {
			s.missedAsked[t.ID] = true
			newMissed = append(newMissed, t)
		}
	}

	if len(newMissed) == 0 {
		return
	}

	if s.options.OnMissed != nil {
		s.options.OnMissed(newMissed)
	} else if s.options.OnFire != nil {
		// Build notification and fire it as a prompt
		notification := BuildMissedTaskNotification(newMissed)
		s.options.OnFire(notification)
	}
}

// check is the core scheduler tick. Called every second.
func (s *CronScheduler) check() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return
	}

	// Check killswitch
	if s.options.IsKilled != nil && s.options.IsKilled() {
		return
	}

	// Check loading gate
	if s.options.IsLoading != nil && s.options.IsLoading() && !s.options.AssistantMode {
		return
	}

	cfg := DefaultCronJitterConfig()
	if s.options.GetJitterConfig != nil {
		cfg = s.options.GetJitterConfig()
	}

	now := nowMs()

	// Collect all tasks to check: file-backed (if owner) + session-scoped
	var allTasks []CronTask
	if s.isOwner {
		allTasks = append(allTasks, s.tasks...)
	}
	allTasks = append(allTasks, GetSessionTasks()...)

	// Track which task IDs are still present (for stale eviction)
	presentIDs := make(map[string]bool, len(allTasks))

	for _, task := range allTasks {
		presentIDs[task.ID] = true

		// Apply filter
		if s.options.Filter != nil && !s.options.Filter(task) {
			continue
		}

		// Skip in-flight tasks
		if s.inFlight[task.ID] {
			continue
		}

		// Compute nextFireAt on first sight
		if _, seen := s.nextFireAt[task.ID]; !seen {
			isRecurring := task.Recurring != nil && *task.Recurring
			var fireMs *int64
			if isRecurring {
				from := task.CreatedAt
				if task.LastFiredAt != nil {
					from = *task.LastFiredAt
				}
				fireMs = JitteredNextCronRunMs(task.Cron, from, task.ID, cfg)
			} else {
				from := task.CreatedAt
				if task.LastFiredAt != nil {
					from = *task.LastFiredAt
				}
				fireMs = OneShotJitteredNextCronRunMs(task.Cron, from, task.ID, cfg)
			}
			if fireMs == nil {
				continue
			}
			s.nextFireAt[task.ID] = *fireMs
		}

		// Not yet due
		if now < s.nextFireAt[task.ID] {
			continue
		}

		// Fire the task
		s.fireTask(task)

		isRecurring := task.Recurring != nil && *task.Recurring

		if isRecurring && !IsRecurringTaskAged(task, now, cfg.RecurringMaxAgeMs) {
			// Reschedule from now with jitter
			nextMs := JitteredNextCronRunMs(task.Cron, now, task.ID, cfg)
			if nextMs != nil {
				s.nextFireAt[task.ID] = *nextMs
			} else {
				delete(s.nextFireAt, task.ID)
			}
			// Persist lastFiredAt
			if s.isOwner {
				go func(id string, firedAt int64, dir string) {
					if err := MarkCronTasksFired([]string{id}, firedAt, dir); err != nil {
						log.Printf("daemon: failed to mark task %s fired: %v", id, err)
					}
				}(task.ID, now, s.options.Dir)
			}
		} else {
			// One-shot or aged recurring: remove from file
			s.inFlight[task.ID] = true
			delete(s.nextFireAt, task.ID)
			if s.isOwner {
				go func(id string, dir string) {
					if err := RemoveCronTasks([]string{id}, dir); err != nil {
						log.Printf("daemon: failed to remove task %s: %v", id, err)
					}
					s.mu.Lock()
					delete(s.inFlight, id)
					s.mu.Unlock()
				}(task.ID, s.options.Dir)
			} else {
				delete(s.inFlight, task.ID)
			}
		}
	}

	// Evict stale nextFireAt entries for removed tasks
	for id := range s.nextFireAt {
		if !presentIDs[id] {
			delete(s.nextFireAt, id)
		}
	}
}

// fireTask dispatches a task's prompt via the configured callbacks.
func (s *CronScheduler) fireTask(task CronTask) {
	log.Printf("daemon: firing task %s, prompt: %s", task.ID, task.Prompt)

	if s.options.OnFireTask != nil {
		s.options.OnFireTask(task)
		return
	}

	if s.options.OnFire != nil {
		s.options.OnFire(task.Prompt)
	}
}

// startWatcher sets up fsnotify watching on the cron tasks file.
func (s *CronScheduler) startWatcher() error {
	cronFilePath := GetCronFilePath(s.options.Dir)
	watchDir := filepath.Dir(cronFilePath)

	// Ensure the watch directory exists
	if err := os.MkdirAll(watchDir, 0o755); err != nil {
		return fmt.Errorf("create watch dir: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	if err := watcher.Add(watchDir); err != nil {
		watcher.Close()
		return fmt.Errorf("watch dir: %w", err)
	}

	s.watcher = watcher

	go func() {
		for {
			select {
			case <-s.done:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Name == cronFilePath &&
					(event.Op&fsnotify.Write != 0 || event.Op&fsnotify.Create != 0) {
					// Reload tasks on file change
					if err := s.load(false); err != nil {
						log.Printf("daemon: failed to reload tasks on file change: %v", err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("daemon: watcher error: %v", err)
			}
		}
	}()

	return nil
}

// GetNextFireTime returns the minimum nextFireAt across all tasks (nil if none).
func (s *CronScheduler) GetNextFireTime() *int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.nextFireAt) == 0 {
		return nil
	}

	var minTime int64
	first := true
	for _, t := range s.nextFireAt {
		if first || t < minTime {
			minTime = t
			first = false
		}
	}
	return &minTime
}

// Tasks returns a copy of the current file-backed task list.
func (s *CronScheduler) Tasks() []CronTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]CronTask, len(s.tasks))
	copy(result, s.tasks)
	return result
}

// BuildMissedTaskNotification formats a missed task notification matching the TS format.
func BuildMissedTaskNotification(missed []CronTask) string {
	var b strings.Builder
	b.WriteString("The following one-shot scheduled task(s) missed while Claude was not running:\n\n")
	b.WriteString("Do NOT execute these prompts yet. First use AskUserQuestion to confirm whether the user wants to proceed.\n\n")

	for _, task := range missed {
		createdTime := time.UnixMilli(task.CreatedAt).UTC().Format(time.RFC3339)
		b.WriteString(fmt.Sprintf("- Schedule: %s\n", task.Cron))
		b.WriteString(fmt.Sprintf("  Created: %s\n", createdTime))
		b.WriteString("  Prompt:\n```\n")
		b.WriteString(task.Prompt)
		b.WriteString("\n```\n\n")
	}

	return b.String()
}
