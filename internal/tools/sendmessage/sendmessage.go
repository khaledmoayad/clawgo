// Package sendmessage implements the SendMessage tool for delivering
// follow-up messages to running worker agents in the swarm system.
package sendmessage

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
	To      string `json:"to"`
	Message string `json:"message"`
}

// SendMessageTool sends a follow-up message to a running worker agent.
type SendMessageTool struct {
	Manager *swarm.Manager
}

// New creates a new SendMessageTool wired to the swarm manager.
func New(manager *swarm.Manager) *SendMessageTool {
	return &SendMessageTool{Manager: manager}
}

func (t *SendMessageTool) Name() string                { return "SendMessage" }
func (t *SendMessageTool) Description() string          { return toolDescription }
func (t *SendMessageTool) IsReadOnly() bool             { return false }
func (t *SendMessageTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because sending messages modifies shared state.
func (t *SendMessageTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *SendMessageTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("SendMessage", false, permCtx), nil
}

func (t *SendMessageTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.To) == "" {
		return tools.ErrorResult("required field \"to\" is missing or empty"), nil
	}
	if strings.TrimSpace(in.Message) == "" {
		return tools.ErrorResult("required field \"message\" is missing or empty"), nil
	}

	if t.Manager == nil {
		return tools.ErrorResult("swarm manager not available"), nil
	}

	if err := t.Manager.SendMessage(in.To, in.Message); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	return tools.TextResult(fmt.Sprintf("Message delivered to worker %q.", in.To)), nil
}
