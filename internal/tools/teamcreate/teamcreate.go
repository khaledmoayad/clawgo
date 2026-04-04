// Package teamcreate implements the TeamCreate tool for creating named
// teams of worker agents in the swarm system.
package teamcreate

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
	TeamName    string `json:"team_name"`
	Description string `json:"description,omitempty"`
}

// TeamCreateTool creates a new team of agents for collaborative work.
type TeamCreateTool struct {
	Manager *swarm.Manager
}

// New creates a new TeamCreateTool wired to the swarm manager.
func New(manager *swarm.Manager) *TeamCreateTool {
	return &TeamCreateTool{Manager: manager}
}

func (t *TeamCreateTool) Name() string                { return "TeamCreate" }
func (t *TeamCreateTool) Description() string          { return toolDescription }
func (t *TeamCreateTool) IsReadOnly() bool             { return false }
func (t *TeamCreateTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because team creation modifies shared state.
func (t *TeamCreateTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *TeamCreateTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("TeamCreate", false, permCtx), nil
}

func (t *TeamCreateTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.TeamName) == "" {
		return tools.ErrorResult("required field \"team_name\" is missing or empty"), nil
	}

	if t.Manager == nil {
		return tools.ErrorResult("swarm manager not available"), nil
	}

	team := t.Manager.CreateTeam(in.TeamName)
	return tools.TextResult(fmt.Sprintf("Team %q created at %s.", team.Name, team.CreatedAt.Format("15:04:05"))), nil
}
