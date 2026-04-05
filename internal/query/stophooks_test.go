package query

import (
	"context"
	"testing"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// mockHookRunner implements HookRunner for testing.
type mockHookRunner struct {
	stopEvents        []HookEvent
	stopFailureEvents []HookEvent
	stopFailureCalled chan struct{}
}

func newMockHookRunner(stopEvents []HookEvent) *mockHookRunner {
	return &mockHookRunner{
		stopEvents:        stopEvents,
		stopFailureCalled: make(chan struct{}, 1),
	}
}

func (m *mockHookRunner) ExecuteStopHooks(_ context.Context, _ []api.Message) <-chan HookEvent {
	ch := make(chan HookEvent, len(m.stopEvents))
	for _, e := range m.stopEvents {
		ch <- e
	}
	close(ch)
	return ch
}

func (m *mockHookRunner) ExecuteStopFailureHooks(_ context.Context, _ api.Message) <-chan HookEvent {
	ch := make(chan HookEvent, len(m.stopFailureEvents))
	for _, e := range m.stopFailureEvents {
		ch <- e
	}
	close(ch)
	// Signal that stop failure hooks were called
	select {
	case m.stopFailureCalled <- struct{}{}:
	default:
	}
	return ch
}

func TestStopHookNoHooks(t *testing.T) {
	// No hooks returns empty result
	runner := newMockHookRunner(nil)
	result, err := HandleStopHooks(context.Background(), nil, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HookCount != 0 {
		t.Errorf("expected 0 hooks, got %d", result.HookCount)
	}
	if result.PreventContinuation {
		t.Error("expected PreventContinuation=false")
	}
	if len(result.BlockingErrors) != 0 {
		t.Errorf("expected 0 blocking errors, got %d", len(result.BlockingErrors))
	}
}

func TestStopHookNilRunner(t *testing.T) {
	// nil runner returns empty result (no hooks configured)
	result, err := HandleStopHooks(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HookCount != 0 {
		t.Errorf("expected 0 hooks, got %d", result.HookCount)
	}
	if result.PreventContinuation {
		t.Error("expected PreventContinuation=false")
	}
}

func TestStopHookBlockingError(t *testing.T) {
	runner := newMockHookRunner([]HookEvent{
		{Type: HookEventProgress, Command: "lint"},
		{Type: HookEventBlockingError, Error: "lint check failed: missing semicolons"},
	})

	result, err := HandleStopHooks(context.Background(), nil, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.BlockingErrors) != 1 {
		t.Fatalf("expected 1 blocking error, got %d", len(result.BlockingErrors))
	}
	if result.BlockingErrors[0] != "lint check failed: missing semicolons" {
		t.Errorf("unexpected blocking error: %s", result.BlockingErrors[0])
	}
	if result.HookCount != 1 {
		t.Errorf("expected 1 hook, got %d", result.HookCount)
	}
}

func TestStopHookPreventContinuation(t *testing.T) {
	runner := newMockHookRunner([]HookEvent{
		{Type: HookEventProgress, Command: "check-budget"},
		{Type: HookEventPreventContinuation, StopReason: "budget exceeded"},
	})

	result, err := HandleStopHooks(context.Background(), nil, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.PreventContinuation {
		t.Error("expected PreventContinuation=true")
	}
	if result.StopReason != "budget exceeded" {
		t.Errorf("expected stop reason 'budget exceeded', got %q", result.StopReason)
	}
}

func TestStopHookPreventContinuationDefaultReason(t *testing.T) {
	runner := newMockHookRunner([]HookEvent{
		{Type: HookEventPreventContinuation},
	})

	result, err := HandleStopHooks(context.Background(), nil, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.PreventContinuation {
		t.Error("expected PreventContinuation=true")
	}
	if result.StopReason != "Stop hook prevented continuation" {
		t.Errorf("expected default stop reason, got %q", result.StopReason)
	}
}

func TestStopHookContextCancellation(t *testing.T) {
	// Create a slow hook runner that blocks on a channel
	slowRunner := &slowHookRunner{
		delay: 500 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately after starting
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	result, err := HandleStopHooks(ctx, nil, slowRunner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.PreventContinuation {
		t.Error("expected PreventContinuation=true on context cancellation")
	}
}

func TestStopHookNonBlockingError(t *testing.T) {
	runner := newMockHookRunner([]HookEvent{
		{Type: HookEventProgress, Command: "optional-check"},
		{Type: HookEventNonBlockingError, Error: "warning: could not reach server"},
	})

	result, err := HandleStopHooks(context.Background(), nil, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.HookErrors) != 1 {
		t.Fatalf("expected 1 hook error, got %d", len(result.HookErrors))
	}
	if result.HookErrors[0] != "warning: could not reach server" {
		t.Errorf("unexpected hook error: %s", result.HookErrors[0])
	}
	// Non-blocking errors do not prevent continuation
	if result.PreventContinuation {
		t.Error("expected PreventContinuation=false for non-blocking error")
	}
}

func TestStopHookSuccessWithDuration(t *testing.T) {
	runner := newMockHookRunner([]HookEvent{
		{Type: HookEventProgress, Command: "typecheck"},
		{Type: HookEventSuccess, Command: "typecheck", DurationMs: 1234},
	})

	result, err := HandleStopHooks(context.Background(), nil, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HookCount != 1 {
		t.Fatalf("expected 1 hook, got %d", result.HookCount)
	}
	if len(result.HookInfos) != 1 {
		t.Fatalf("expected 1 hook info, got %d", len(result.HookInfos))
	}
	if result.HookInfos[0].DurationMs != 1234 {
		t.Errorf("expected duration 1234, got %d", result.HookInfos[0].DurationMs)
	}
}

func TestStopHookMultipleEvents(t *testing.T) {
	runner := newMockHookRunner([]HookEvent{
		{Type: HookEventProgress, Command: "lint"},
		{Type: HookEventProgress, Command: "typecheck"},
		{Type: HookEventNonBlockingError, Error: "lint warning"},
		{Type: HookEventBlockingError, Error: "typecheck failed"},
	})

	result, err := HandleStopHooks(context.Background(), nil, runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HookCount != 2 {
		t.Errorf("expected 2 hooks, got %d", result.HookCount)
	}
	if len(result.BlockingErrors) != 1 {
		t.Errorf("expected 1 blocking error, got %d", len(result.BlockingErrors))
	}
	if len(result.HookErrors) != 1 {
		t.Errorf("expected 1 hook error, got %d", len(result.HookErrors))
	}
}

func TestStopFailureHooksFireAndForget(t *testing.T) {
	runner := newMockHookRunner(nil)
	runner.stopFailureEvents = []HookEvent{
		{Type: HookEventSuccess, Command: "notify-failure"},
	}

	msg := api.AssistantMessage("some error response")
	ExecuteStopFailureHooks(context.Background(), msg, runner)

	// Wait for the goroutine to execute
	select {
	case <-runner.stopFailureCalled:
		// Success -- hook was called
	case <-time.After(2 * time.Second):
		t.Fatal("stop failure hook was not called within timeout")
	}
}

func TestStopFailureHooksNilRunner(t *testing.T) {
	// Should not panic with nil runner
	msg := api.AssistantMessage("error")
	ExecuteStopFailureHooks(context.Background(), msg, nil)
	// No assertion needed -- just verify no panic
}

// slowHookRunner delays before sending events, used to test cancellation.
type slowHookRunner struct {
	delay time.Duration
}

func (s *slowHookRunner) ExecuteStopHooks(ctx context.Context, _ []api.Message) <-chan HookEvent {
	ch := make(chan HookEvent)
	go func() {
		defer close(ch)
		select {
		case <-time.After(s.delay):
			ch <- HookEvent{Type: HookEventProgress, Command: "slow-hook"}
		case <-ctx.Done():
			return
		}
	}()
	return ch
}

func (s *slowHookRunner) ExecuteStopFailureHooks(_ context.Context, _ api.Message) <-chan HookEvent {
	ch := make(chan HookEvent)
	close(ch)
	return ch
}
