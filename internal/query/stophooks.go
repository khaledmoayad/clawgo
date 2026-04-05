// Package query implements stop hook execution for the query engine.
// Stop hooks fire after each assistant turn completes, and stop failure
// hooks fire when an API error occurs. This matches Claude Code's
// query/stopHooks.ts behavior.
package query

import (
	"context"
	"time"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// StopHookResult captures the outcome of running stop hooks after a turn.
type StopHookResult struct {
	// BlockingErrors collects error messages from hooks that returned
	// non-zero exit codes with blocking semantics.
	BlockingErrors []string

	// PreventContinuation is true when a hook explicitly requested that
	// the query loop should not continue (or the context was cancelled).
	PreventContinuation bool

	// StopReason describes why continuation was prevented.
	StopReason string

	// HookCount is the number of hooks that executed.
	HookCount int

	// HookErrors collects non-blocking error messages from hooks.
	HookErrors []string

	// HookInfos records metadata about each executed hook.
	HookInfos []StopHookInfo
}

// StopHookInfo records metadata about a single executed stop hook.
type StopHookInfo struct {
	Command    string
	PromptText string
	DurationMs int64
}

// HookEventType identifies the kind of event emitted by a hook runner.
type HookEventType string

const (
	// HookEventProgress indicates a hook started or is running.
	HookEventProgress HookEventType = "progress"

	// HookEventBlockingError indicates a hook returned a blocking error.
	HookEventBlockingError HookEventType = "blocking_error"

	// HookEventNonBlockingError indicates a hook failed but should not block.
	HookEventNonBlockingError HookEventType = "non_blocking_error"

	// HookEventPreventContinuation indicates a hook wants to stop the loop.
	HookEventPreventContinuation HookEventType = "prevent_continuation"

	// HookEventSuccess indicates a hook completed successfully.
	HookEventSuccess HookEventType = "success"
)

// HookEvent is emitted by a HookRunner during hook execution.
type HookEvent struct {
	Type       HookEventType
	Command    string
	PromptText string
	Error      string
	StopReason string
	Stdout     string
	Stderr     string
	DurationMs int64
}

// HookRunner abstracts hook execution so the query loop can inject a
// real or mock hook executor. This matches the TS pattern where
// executeStopHooks and executeStopFailureHooks are injected functions.
type HookRunner interface {
	// ExecuteStopHooks runs all registered Stop hooks for the given messages
	// and returns a channel of HookEvents. The channel is closed when all
	// hooks have completed.
	ExecuteStopHooks(ctx context.Context, messages []api.Message) <-chan HookEvent

	// ExecuteStopFailureHooks runs all registered StopFailure hooks for the
	// given last message. The channel is closed when all hooks have completed.
	ExecuteStopFailureHooks(ctx context.Context, lastMessage api.Message) <-chan HookEvent
}

// HandleStopHooks executes stop hooks after each assistant turn.
// It consumes all HookEvents from the runner and builds a StopHookResult.
// If the context is cancelled during execution, PreventContinuation is
// set to true, matching the TypeScript abort handling.
func HandleStopHooks(ctx context.Context, messages []api.Message, hookRunner HookRunner) (*StopHookResult, error) {
	result := &StopHookResult{}

	if hookRunner == nil {
		return result, nil
	}

	eventCh := hookRunner.ExecuteStopHooks(ctx, messages)

	for {
		select {
		case <-ctx.Done():
			// Context cancelled during hook execution -- prevent continuation
			result.PreventContinuation = true
			result.StopReason = "context cancelled during stop hook execution"
			// Drain remaining events to avoid goroutine leak
			for range eventCh {
			}
			return result, nil

		case event, ok := <-eventCh:
			if !ok {
				// Channel closed -- all hooks completed
				return result, nil
			}

			switch event.Type {
			case HookEventProgress:
				result.HookCount++
				if event.Command != "" {
					result.HookInfos = append(result.HookInfos, StopHookInfo{
						Command:    event.Command,
						PromptText: event.PromptText,
						DurationMs: event.DurationMs,
					})
				}

			case HookEventBlockingError:
				result.BlockingErrors = append(result.BlockingErrors, event.Error)

			case HookEventNonBlockingError:
				result.HookErrors = append(result.HookErrors, event.Error)

			case HookEventPreventContinuation:
				result.PreventContinuation = true
				if event.StopReason != "" {
					result.StopReason = event.StopReason
				} else {
					result.StopReason = "Stop hook prevented continuation"
				}

			case HookEventSuccess:
				// Track duration if available
				if event.DurationMs > 0 && event.Command != "" {
					for i := range result.HookInfos {
						if result.HookInfos[i].Command == event.Command && result.HookInfos[i].DurationMs == 0 {
							result.HookInfos[i].DurationMs = event.DurationMs
							break
						}
					}
				}
			}
		}
	}
}

// stopFailureTimeout is the maximum time we wait for stop failure hooks.
const stopFailureTimeout = 30 * time.Second

// ExecuteStopFailureHooks fires stop failure hooks when an API error occurs.
// It is fire-and-forget: launched in a goroutine with a 30-second timeout,
// matching Claude Code's executeStopFailureHooks() call pattern.
// The function does not block the caller.
func ExecuteStopFailureHooks(ctx context.Context, lastMessage api.Message, hookRunner HookRunner) {
	if hookRunner == nil {
		return
	}

	go func() {
		failCtx, cancel := context.WithTimeout(ctx, stopFailureTimeout)
		defer cancel()

		eventCh := hookRunner.ExecuteStopFailureHooks(failCtx, lastMessage)
		// Drain all events -- we don't use them, but we must drain to
		// avoid leaking the goroutine in the hook runner.
		for range eventCh {
		}
	}()
}
