package skills

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PollInterval is the interval between filesystem polls for skill changes.
// Using polling instead of fsnotify to avoid CGO dependency and platform-specific
// inotify/kqueue bindings (matches CGO_ENABLED=0 constraint).
const PollInterval = 2 * time.Second

// Watcher polls skill directories for file changes and triggers a callback.
type Watcher struct {
	dirs       []string
	onChange   func()
	done       chan struct{}
	once       sync.Once
	lastMtimes map[string]time.Time
}

// NewWatcher creates a new skill file watcher.
func NewWatcher(dirs []string, onChange func()) *Watcher {
	return &Watcher{
		dirs:       dirs,
		onChange:   onChange,
		done:       make(chan struct{}),
		lastMtimes: make(map[string]time.Time),
	}
}

// Start begins polling for file changes. It returns immediately and runs
// the polling loop in a goroutine. Call Stop to terminate.
func (w *Watcher) Start() error {
	// Build initial mtime snapshot
	w.scanMtimes()

	go w.pollLoop()
	return nil
}

// Stop terminates the polling loop.
func (w *Watcher) Stop() {
	w.once.Do(func() {
		close(w.done)
	})
}

// pollLoop checks for file changes at regular intervals.
func (w *Watcher) pollLoop() {
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			if w.hasChanges() {
				w.onChange()
			}
		}
	}
}

// scanMtimes walks all watched directories and records file modification times.
func (w *Watcher) scanMtimes() map[string]time.Time {
	mtimes := make(map[string]time.Time)
	for _, dir := range w.dirs {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}
			info, infoErr := d.Info()
			if infoErr != nil {
				return nil
			}
			mtimes[path] = info.ModTime()
			return nil
		})
	}
	return mtimes
}

// hasChanges compares current mtimes against the last snapshot.
// Returns true if any file was added, removed, or modified.
func (w *Watcher) hasChanges() bool {
	current := w.scanMtimes()

	changed := false

	// Check for new or modified files
	for path, mtime := range current {
		if lastMtime, ok := w.lastMtimes[path]; !ok || !mtime.Equal(lastMtime) {
			changed = true
			break
		}
	}

	// Check for deleted files
	if !changed {
		for path := range w.lastMtimes {
			if _, ok := current[path]; !ok {
				changed = true
				break
			}
		}
	}

	w.lastMtimes = current
	return changed
}
