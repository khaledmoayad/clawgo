// Package agent implements the AgentTool for spawning sub-agents.
// Sub-agents have their own conversation context with Claude and can use
// all available tools. This enables multi-step reasoning and focused work
// on specific subtasks, matching the TypeScript AgentTool behavior.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/query"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/khaledmoayad/clawgo/internal/tools/tasks"
)

const (
	// MaxNestingDepth is the maximum allowed sub-agent recursion depth.
	// Prevents resource exhaustion from infinite agent chains.
	MaxNestingDepth = 3

	// MaxAgentTurns limits the number of conversation turns a sub-agent can take.
	MaxAgentTurns = 20
)

type input struct {
	Prompt         string   `json:"prompt"`
	Model          string   `json:"model,omitempty"`
	PermittedTools []string `json:"permitted_tools,omitempty"`
	SubagentType   string   `json:"subagent_type,omitempty"` // "worker" or "subagent"
}

// AgentTool spawns a sub-agent with its own conversation context.
type AgentTool struct {
	Registry        *tools.Registry
	Client          *api.Client
	NestingDepth    int // Current nesting level (0 for top-level agents)
	TaskStore       *tasks.Store
	CoordinatorMode bool // When true, "worker" subagent_type spawns async goroutines
}

// New creates a new AgentTool.
func New(registry *tools.Registry, client *api.Client) *AgentTool {
	return &AgentTool{
		Registry:     registry,
		Client:       client,
		NestingDepth: 0,
	}
}

func (t *AgentTool) Name() string                { return "Agent" }
func (t *AgentTool) Description() string          { return toolDescription }
func (t *AgentTool) IsReadOnly() bool             { return false }
func (t *AgentTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because agents modify state and spawn
// their own query loops that can interact with the filesystem.
func (t *AgentTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

// CheckPermissions always returns Ask -- agents can execute arbitrary tools,
// so the user must always approve agent launches.
func (t *AgentTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Ask, nil
}

func (t *AgentTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return tools.ErrorResult("required field \"prompt\" is missing or empty"), nil
	}

	// Enforce nesting depth limit
	if t.NestingDepth >= MaxNestingDepth {
		return tools.ErrorResult("Maximum agent nesting depth reached"), nil
	}

	// Build sub-agent registry (filter tools if permitted_tools specified)
	subRegistry := t.Registry
	if len(in.PermittedTools) > 0 && t.Registry != nil {
		permitted := make(map[string]bool, len(in.PermittedTools))
		for _, name := range in.PermittedTools {
			permitted[name] = true
		}
		var filteredTools []tools.Tool
		for _, tool := range t.Registry.All() {
			if permitted[tool.Name()] {
				filteredTools = append(filteredTools, tool)
			}
		}
		subRegistry = tools.NewRegistry(filteredTools...)
	}

	// Sub-agent client: use model override if specified
	subClient := t.Client
	if subClient == nil {
		return tools.ErrorResult("sub-agent requires an API client"), nil
	}
	if in.Model != "" {
		// Create a shallow copy with the overridden model
		clientCopy := *subClient
		clientCopy.Model = in.Model
		subClient = &clientCopy
	}

	// Determine subagent type (default: "subagent" for backward compat)
	subagentType := in.SubagentType
	if subagentType == "" {
		subagentType = "subagent"
	}

	// In coordinator mode with worker type, use async goroutine execution
	if t.CoordinatorMode && subagentType == "worker" && t.TaskStore != nil {
		return t.callAsync(ctx, in.Prompt, subClient, subRegistry, toolCtx)
	}

	// Default: blocking synchronous execution
	return t.callBlocking(ctx, in.Prompt, subClient, subRegistry, toolCtx)
}

// callBlocking runs the sub-agent synchronously (original behavior).
func (t *AgentTool) callBlocking(ctx context.Context, prompt string, client *api.Client, registry *tools.Registry, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var output strings.Builder
	textCallback := func(text string) {
		output.WriteString(text)
	}

	params := &query.LoopParams{
		Client:      client,
		Registry:    registry,
		PermCtx:     toolCtx.PermCtx,
		CostTracker: cost.NewTracker(client.Model),
		Messages: []api.Message{
			api.UserMessage(prompt),
		},
		SystemPrompt: fmt.Sprintf("You are a sub-agent (depth %d/%d). Complete the assigned task using available tools. Be focused and efficient.", t.NestingDepth+1, MaxNestingDepth),
		MaxTurns:     MaxAgentTurns,
		WorkingDir:   toolCtx.WorkingDir,
		ProjectRoot:  toolCtx.ProjectRoot,
		SessionID:    toolCtx.SessionID,
		TextCallback: textCallback,
	}

	err := query.RunLoop(ctx, params)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("sub-agent error: %s", err.Error())), nil
	}

	result := output.String()
	if result == "" {
		result = "(sub-agent produced no output)"
	}

	return tools.TextResult(result), nil
}

// callAsync spawns the sub-agent as a goroutine, tracks it in the task store,
// and returns immediately with the task ID.
func (t *AgentTool) callAsync(parentCtx context.Context, prompt string, client *api.Client, registry *tools.Registry, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	// Create a child context with cancellation for the sub-agent
	agentCtx, cancel := context.WithCancel(parentCtx)

	// Register in the task store
	task := t.TaskStore.CreateWithCancel(prompt, "local_agent", cancel)
	_ = t.TaskStore.Update(task.ID, "running", "")

	go func() {
		// Deferred recovery to catch panics and update the task store
		defer func() {
			if r := recover(); r != nil {
				_ = t.TaskStore.Update(task.ID, "failed", fmt.Sprintf("panic: %v", r))
			}
			cancel() // Ensure context is always cleaned up
		}()

		var output strings.Builder
		textCallback := func(text string) {
			output.WriteString(text)
			// Send to output channel for streaming consumers (non-blocking)
			select {
			case task.OutputCh <- text:
			default:
			}
		}

		params := &query.LoopParams{
			Client:      client,
			Registry:    registry,
			PermCtx:     toolCtx.PermCtx,
			CostTracker: cost.NewTracker(client.Model),
			Messages: []api.Message{
				api.UserMessage(prompt),
			},
			SystemPrompt: fmt.Sprintf("You are a worker agent (depth %d/%d). Complete the assigned task using available tools. Be focused and efficient.", t.NestingDepth+1, MaxNestingDepth),
			MaxTurns:     MaxAgentTurns,
			WorkingDir:   toolCtx.WorkingDir,
			ProjectRoot:  toolCtx.ProjectRoot,
			SessionID:    toolCtx.SessionID,
			TextCallback: textCallback,
		}

		err := query.RunLoop(agentCtx, params)
		if err != nil {
			_ = t.TaskStore.Update(task.ID, "failed", fmt.Sprintf("sub-agent error: %s", err.Error()))
			return
		}

		result := output.String()
		if result == "" {
			result = "(sub-agent produced no output)"
		}
		_ = t.TaskStore.Update(task.ID, "completed", result)
	}()

	return tools.TextResult(fmt.Sprintf("Sub-agent launched (task_id: %s). The agent is working in the background. Results will be reported when complete.", task.ID)), nil
}
