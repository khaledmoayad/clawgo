// Package sleep implements the SleepTool for pausing execution.
// It supports context cancellation so that sleep can be interrupted
// by user abort or timeout, matching the TypeScript Sleep tool behavior.
package sleep

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

const (
	minSeconds = 1
	maxSeconds = 300
)

type input struct {
	Seconds float64 `json:"seconds"`
}

// SleepTool pauses execution for a specified duration.
type SleepTool struct{}

// New creates a new SleepTool.
func New() *SleepTool { return &SleepTool{} }

func (t *SleepTool) Name() string                { return "Sleep" }
func (t *SleepTool) Description() string          { return toolDescription }
func (t *SleepTool) IsReadOnly() bool             { return true }
func (t *SleepTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true -- sleeping doesn't modify state.
func (t *SleepTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// CheckPermissions returns Allow -- sleeping is always permitted.
func (t *SleepTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

func (t *SleepTool) Call(ctx context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	seconds := int(in.Seconds)
	if seconds < minSeconds {
		return tools.ErrorResult(fmt.Sprintf("seconds must be at least %d", minSeconds)), nil
	}
	if seconds > maxSeconds {
		return tools.ErrorResult(fmt.Sprintf("seconds must be at most %d", maxSeconds)), nil
	}

	// Sleep with context cancellation support
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()

	select {
	case <-timer.C:
		return tools.TextResult(fmt.Sprintf("Slept for %d seconds.", seconds)), nil
	case <-ctx.Done():
		return tools.TextResult(fmt.Sprintf("Sleep interrupted after partial wait (requested %d seconds).", seconds)), nil
	}
}
