package app

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterAndRunCleanups(t *testing.T) {
	// Reset global state for test isolation
	resetCleanups()

	var order []int
	RegisterCleanup(func() { order = append(order, 1) })
	RegisterCleanup(func() { order = append(order, 2) })
	RegisterCleanup(func() { order = append(order, 3) })

	RunCleanups()

	// Should execute in reverse order (LIFO)
	require.Len(t, order, 3)
	assert.Equal(t, []int{3, 2, 1}, order)
}

func TestGracefulShutdown_Signal(t *testing.T) {
	// Reset global state for test isolation
	resetCleanups()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cleanupRan := make(chan struct{})
	RegisterCleanup(func() {
		close(cleanupRan)
	})

	SetupGracefulShutdown(cancel)

	// Give the goroutine time to set up the signal handler
	time.Sleep(50 * time.Millisecond)

	// Send SIGINT to self
	err := syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	require.NoError(t, err)

	// Wait for cleanup to run with timeout
	select {
	case <-cleanupRan:
		// Success: cleanup was called
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup did not run within 2 seconds after SIGINT")
	}

	// Context should be cancelled
	assert.Error(t, ctx.Err())
}
