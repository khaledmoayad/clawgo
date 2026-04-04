// Package teamdelete implements the TeamDelete tool for removing teams
// and cancelling all their running worker agents.
package teamdelete

import (
	"context"
	"encoding/json"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/swarm"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// TeamDeleteTool deletes the current team and cancels all its running workers.
type TeamDeleteTool struct {
	Manager *swarm.Manager
}

// New creates a new TeamDeleteTool wired to the swarm manager.
func New(manager *swarm.Manager) *TeamDeleteTool {
	return &TeamDeleteTool{Manager: manager}
}

func (t *TeamDeleteTool) Name() string                { return "TeamDelete" }
func (t *TeamDeleteTool) Description() string          { return toolDescription }
func (t *TeamDeleteTool) IsReadOnly() bool             { return false }
func (t *TeamDeleteTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because team deletion modifies shared state.
func (t *TeamDeleteTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *TeamDeleteTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("TeamDelete", false, permCtx), nil
}

func (t *TeamDeleteTool) Call(_ context.Context, _ json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	if t.Manager == nil {
		return tools.ErrorResult("swarm manager not available"), nil
	}

	// Delete the current team (Claude Code's TeamDelete has no parameters --
	// it deletes the current team context)
	currentTeam := t.Manager.CurrentTeam()
	if currentTeam == "" {
		return tools.ErrorResult("no active team to delete"), nil
	}

	if err := t.Manager.DeleteTeam(currentTeam); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.TextResult("Team deleted. All running workers have been cancelled."), nil
}
