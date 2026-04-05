// Package agent implements the AgentTool for spawning sub-agents.
// Sub-agents have their own conversation context with Claude and can use
// all available tools. This enables multi-step reasoning and focused work
// on specific subtasks, matching the TypeScript AgentTool behavior.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	Prompt          string   `json:"prompt"`
	Description     string   `json:"description,omitempty"`
	Model           string   `json:"model,omitempty"`
	PermittedTools  []string `json:"permitted_tools,omitempty"`
	SubagentType    string   `json:"subagent_type,omitempty"` // "worker" or "subagent"
	Name            string   `json:"name,omitempty"`
	TeamName        string   `json:"team_name,omitempty"`
	RunInBackground bool     `json:"run_in_background,omitempty"`
	Isolation       string   `json:"isolation,omitempty"` // "worktree"
	Cwd             string   `json:"cwd,omitempty"`
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
func (t *AgentTool) Description() string          { return ToolDescription() }
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

	// Validate isolation parameter
	if in.Isolation != "" && in.Isolation != "worktree" {
		return tools.ErrorResult(fmt.Sprintf("invalid isolation mode %q: only \"worktree\" is supported", in.Isolation)), nil
	}

	// Validate cwd is absolute path
	if in.Cwd != "" && !filepath.IsAbs(in.Cwd) {
		return tools.ErrorResult(fmt.Sprintf("cwd must be an absolute path, got %q", in.Cwd)), nil
	}

	// Validate mutual exclusivity of cwd and isolation="worktree"
	if in.Cwd != "" && in.Isolation == "worktree" {
		return tools.ErrorResult("cwd and isolation=\"worktree\" are mutually exclusive: worktree creates its own working directory"), nil
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

	// Determine if async execution is needed:
	// 1. Explicit run_in_background=true
	// 2. Coordinator mode with worker subagent type (legacy behavior)
	shouldRunAsync := in.RunInBackground
	if !shouldRunAsync && t.CoordinatorMode && subagentType == "worker" {
		shouldRunAsync = true
	}

	if shouldRunAsync && t.TaskStore != nil {
		return t.callAsync(ctx, &in, subClient, subRegistry, toolCtx)
	}

	// Default: blocking synchronous execution
	return t.callBlocking(ctx, &in, subClient, subRegistry, toolCtx)
}

// resolveWorkingDir determines the effective working directory for the agent.
// It handles cwd override and worktree isolation, returning the directory path
// and an optional cleanup function.
func (t *AgentTool) resolveWorkingDir(ctx context.Context, in *input, baseDir string) (workDir string, worktreeDir string, cleanup func(), err error) {
	// Explicit cwd override takes precedence
	if in.Cwd != "" {
		return in.Cwd, "", nil, nil
	}

	// Worktree isolation: create a temporary git worktree
	if in.Isolation == "worktree" {
		tmpDir, tmpErr := os.MkdirTemp("", "clawgo-worktree-*")
		if tmpErr != nil {
			return "", "", nil, fmt.Errorf("failed to create temp dir for worktree: %w", tmpErr)
		}

		// git worktree add requires the directory to not exist -- remove the empty temp dir
		// and use its path as the worktree target
		os.Remove(tmpDir)

		cmd := exec.CommandContext(ctx, "git", "worktree", "add", tmpDir, "--detach")
		cmd.Dir = baseDir
		if output, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
			return "", "", nil, fmt.Errorf("failed to create git worktree: %s: %w", strings.TrimSpace(string(output)), cmdErr)
		}

		cleanupFn := func() {
			// Check if there are changes in the worktree
			statusCmd := exec.Command("git", "-C", tmpDir, "status", "--porcelain")
			statusOutput, statusErr := statusCmd.Output()
			if statusErr != nil || len(strings.TrimSpace(string(statusOutput))) == 0 {
				// No changes or error checking -- clean up the worktree
				removeCmd := exec.Command("git", "worktree", "remove", tmpDir, "--force")
				removeCmd.Dir = baseDir
				_ = removeCmd.Run()
			}
		}

		return tmpDir, tmpDir, cleanupFn, nil
	}

	return baseDir, "", nil, nil
}

// buildSystemPrompt constructs the system prompt for the sub-agent,
// incorporating team_name if provided.
func (t *AgentTool) buildSystemPrompt(in *input) string {
	var parts []string

	if in.TeamName != "" {
		parts = append(parts, fmt.Sprintf("You are part of team '%s'.", in.TeamName))
	}

	depth := t.NestingDepth + 1
	if in.RunInBackground || (t.CoordinatorMode && in.SubagentType == "worker") {
		parts = append(parts, fmt.Sprintf("You are a worker agent (depth %d/%d). Complete the assigned task using available tools. Be focused and efficient.", depth, MaxNestingDepth))
	} else {
		parts = append(parts, fmt.Sprintf("You are a sub-agent (depth %d/%d). Complete the assigned task using available tools. Be focused and efficient.", depth, MaxNestingDepth))
	}

	return strings.Join(parts, " ")
}

// formatResultMessage builds the result text for the agent, including name
// and description if provided.
func formatResultMessage(in *input, baseMsg string) string {
	var parts []string
	if in.Name != "" {
		parts = append(parts, fmt.Sprintf("Agent '%s'", in.Name))
	} else {
		parts = append(parts, "Sub-agent")
	}
	if in.Description != "" {
		parts = append(parts, fmt.Sprintf("(%s)", in.Description))
	}
	parts = append(parts, baseMsg)
	return strings.Join(parts, " ")
}

// callBlocking runs the sub-agent synchronously (original behavior).
func (t *AgentTool) callBlocking(ctx context.Context, in *input, client *api.Client, registry *tools.Registry, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	workDir, worktreeDir, cleanup, err := t.resolveWorkingDir(ctx, in, toolCtx.WorkingDir)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("failed to set up working directory: %s", err.Error())), nil
	}
	if cleanup != nil {
		defer cleanup()
	}

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
			api.UserMessage(in.Prompt),
		},
		SystemPrompt: t.buildSystemPrompt(in),
		MaxTurns:     MaxAgentTurns,
		WorkingDir:   workDir,
		ProjectRoot:  toolCtx.ProjectRoot,
		SessionID:    toolCtx.SessionID,
		TextCallback: textCallback,
	}

	loopErr := query.RunLoop(ctx, params)
	if loopErr != nil {
		return tools.ErrorResult(fmt.Sprintf("sub-agent error: %s", loopErr.Error())), nil
	}

	result := output.String()
	if result == "" {
		result = "(sub-agent produced no output)"
	}

	// If worktree was used and has changes, include the path in the result
	if worktreeDir != "" {
		statusCmd := exec.Command("git", "-C", worktreeDir, "status", "--porcelain")
		if statusOutput, statusErr := statusCmd.Output(); statusErr == nil && len(strings.TrimSpace(string(statusOutput))) > 0 {
			result = result + fmt.Sprintf("\n\n[Worktree with changes at: %s]", worktreeDir)
		}
	}

	return tools.TextResult(result), nil
}

// callAsync spawns the sub-agent as a goroutine, tracks it in the task store,
// and returns immediately with the task ID.
func (t *AgentTool) callAsync(parentCtx context.Context, in *input, client *api.Client, registry *tools.Registry, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	// Create a child context with cancellation for the sub-agent
	agentCtx, cancel := context.WithCancel(parentCtx)

	// Register in the task store with description and name
	taskDesc := in.Prompt
	if in.Description != "" {
		taskDesc = in.Description
	}
	task := t.TaskStore.CreateWithCancel(taskDesc, "local_agent", cancel)
	if in.Name != "" {
		task.Name = in.Name
	}
	_ = t.TaskStore.Update(task.ID, "running", "")

	go func() {
		// Deferred recovery to catch panics and update the task store
		defer func() {
			if r := recover(); r != nil {
				_ = t.TaskStore.Update(task.ID, "failed", fmt.Sprintf("panic: %v", r))
			}
			cancel() // Ensure context is always cleaned up
		}()

		workDir, worktreeDir, cleanup, wdErr := t.resolveWorkingDir(agentCtx, in, toolCtx.WorkingDir)
		if wdErr != nil {
			_ = t.TaskStore.Update(task.ID, "failed", fmt.Sprintf("working directory setup failed: %s", wdErr.Error()))
			return
		}
		if cleanup != nil {
			defer cleanup()
		}

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
				api.UserMessage(in.Prompt),
			},
			SystemPrompt: t.buildSystemPrompt(in),
			MaxTurns:     MaxAgentTurns,
			WorkingDir:   workDir,
			ProjectRoot:  toolCtx.ProjectRoot,
			SessionID:    toolCtx.SessionID,
			TextCallback: textCallback,
		}

		loopErr := query.RunLoop(agentCtx, params)
		if loopErr != nil {
			_ = t.TaskStore.Update(task.ID, "failed", fmt.Sprintf("sub-agent error: %s", loopErr.Error()))
			return
		}

		result := output.String()
		if result == "" {
			result = "(sub-agent produced no output)"
		}

		// If worktree was used and has changes, include the path in the result
		if worktreeDir != "" {
			statusCmd := exec.Command("git", "-C", worktreeDir, "status", "--porcelain")
			if statusOutput, statusErr := statusCmd.Output(); statusErr == nil && len(strings.TrimSpace(string(statusOutput))) > 0 {
				result = result + fmt.Sprintf("\n\n[Worktree with changes at: %s]", worktreeDir)
			}
		}

		_ = t.TaskStore.Update(task.ID, "completed", result)
	}()

	launchMsg := formatResultMessage(in, fmt.Sprintf("launched (task_id: %s). The agent is working in the background. Results will be reported when complete.", task.ID))
	return tools.TextResult(launchMsg), nil
}
