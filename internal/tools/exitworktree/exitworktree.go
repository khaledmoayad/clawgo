// Package exitworktree implements the ExitWorktree tool for leaving git worktrees.
package exitworktree

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Path   string `json:"path"`
	Remove bool   `json:"remove"`
}

// ExitWorktreeTool exits a git worktree and resets the working directory.
type ExitWorktreeTool struct{}

// New creates a new ExitWorktreeTool.
func New() *ExitWorktreeTool { return &ExitWorktreeTool{} }

func (t *ExitWorktreeTool) Name() string                { return "ExitWorktree" }
func (t *ExitWorktreeTool) Description() string          { return toolDescription }
func (t *ExitWorktreeTool) IsReadOnly() bool             { return false }
func (t *ExitWorktreeTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because removing worktrees modifies git state.
func (t *ExitWorktreeTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *ExitWorktreeTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("ExitWorktree", false, permCtx), nil
}

func (t *ExitWorktreeTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Path) == "" {
		return tools.ErrorResult("required field \"path\" is missing or empty"), nil
	}

	msg := fmt.Sprintf("Exited worktree at %s.", in.Path)

	// Optionally remove the worktree
	if in.Remove {
		projectRoot := toolCtx.ProjectRoot
		if projectRoot == "" {
			projectRoot = toolCtx.WorkingDir
		}
		cmd := exec.CommandContext(ctx, "git", "-C", projectRoot, "worktree", "remove", in.Path)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return tools.ErrorResult(fmt.Sprintf("Failed to remove worktree: %s\n%s", err.Error(), strings.TrimSpace(string(output)))), nil
		}
		msg = fmt.Sprintf("Removed worktree at %s.", in.Path)
	}

	// Reset working directory to project root via context modifier
	return &tools.ToolResult{
		Content: []tools.ContentBlock{{
			Type: "text",
			Text: msg + " Working directory reset to project root.",
		}},
		ContextModifier: func(ctx *tools.ToolUseContext) {
			ctx.WorkingDir = ctx.ProjectRoot
		},
	}, nil
}
