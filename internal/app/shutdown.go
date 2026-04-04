package app

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	cleanupMu    sync.Mutex
	cleanupFuncs []func()
)

// RegisterCleanup adds a cleanup function to run on shutdown.
// Cleanup functions are executed in reverse order (LIFO).
func RegisterCleanup(fn func()) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	cleanupFuncs = append(cleanupFuncs, fn)
}

// RunCleanups executes all registered cleanup functions in reverse order.
// It is safe to call multiple times; subsequent calls are no-ops.
func RunCleanups() {
	cleanupMu.Lock()
	fns := make([]func(), len(cleanupFuncs))
	copy(fns, cleanupFuncs)
	cleanupFuncs = nil
	cleanupMu.Unlock()

	// Execute in reverse order (LIFO)
	for i := len(fns) - 1; i >= 0; i-- {
		fns[i]()
	}
}

// SetupGracefulShutdown listens for SIGINT and SIGTERM and runs cleanups.
// The cancel function is called first to signal context cancellation,
// then all registered cleanup functions are executed.
func SetupGracefulShutdown(cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		signal.Stop(sigCh)
		cancel()
		RunCleanups()
	}()
}

// resetCleanups is used by tests to reset the global cleanup state.
func resetCleanups() {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	cleanupFuncs = nil
}
