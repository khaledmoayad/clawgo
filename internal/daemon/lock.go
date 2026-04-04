package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const lockFileName = "scheduler.lock"

// Lock provides a file-based mutex for ensuring only one scheduler instance
// runs at a time. The lock file contains the PID of the owner.
type Lock struct {
	path string
	file *os.File
}

// NewLock creates a lock targeting the given config directory.
func NewLock(configDir string) *Lock {
	return &Lock{
		path: filepath.Join(configDir, lockFileName),
	}
}

// TryAcquire attempts to acquire the scheduler lock.
// Returns true if the lock was successfully acquired.
// If the lock file exists with a stale PID, it is removed and retried.
func (l *Lock) TryAcquire() (bool, error) {
	// Try to create the lock file exclusively
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		// Successfully created -- write our PID
		pid := os.Getpid()
		if _, writeErr := fmt.Fprintf(f, "%d", pid); writeErr != nil {
			f.Close()
			os.Remove(l.path)
			return false, fmt.Errorf("write pid to lock: %w", writeErr)
		}
		l.file = f
		return true, nil
	}

	if !os.IsExist(err) {
		return false, fmt.Errorf("open lock file: %w", err)
	}

	// Lock file exists -- check if the PID is still alive
	data, readErr := os.ReadFile(l.path)
	if readErr != nil {
		return false, fmt.Errorf("read lock file: %w", readErr)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, parseErr := strconv.Atoi(pidStr)
	if parseErr != nil {
		// Invalid PID in lock file -- treat as stale
		if removeErr := os.Remove(l.path); removeErr != nil {
			return false, fmt.Errorf("remove invalid lock: %w", removeErr)
		}
		return l.TryAcquire()
	}

	// Check if the process is still running
	proc, findErr := os.FindProcess(pid)
	if findErr != nil {
		// Process not found -- stale lock
		os.Remove(l.path)
		return l.TryAcquire()
	}

	// Send signal 0 to check if process is alive
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		// Process is not running -- stale lock
		os.Remove(l.path)
		return l.TryAcquire()
	}

	// Process is alive -- lock is held by another instance
	return false, nil
}

// Release releases the scheduler lock by closing and removing the lock file.
func (l *Lock) Release() error {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
	return os.Remove(l.path)
}
