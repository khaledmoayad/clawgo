// Package teamdelete implements the TeamDelete tool for removing teams
// and cancelling all their running worker agents.
package teamdelete

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/swarm"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Name string `json:"name"`
}

// TeamDeleteTool deletes an existing team and cancels all its running workers.
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

func (t *TeamDeleteTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Name) == "" {
		return tools.ErrorResult("required field \"name\" is missing or empty"), nil
	}

	if t.Manager == nil {
		return tools.ErrorResult("swarm manager not available"), nil
	}

	if err := t.Manager.DeleteTeam(in.Name); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.TextResult(fmt.Sprintf("Team %q deleted. All running workers in this team have been cancelled.", in.Name)), nil
}
