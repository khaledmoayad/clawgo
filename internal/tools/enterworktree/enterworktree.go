// Package enterworktree implements the EnterWorktree tool for creating git worktrees.
package enterworktree

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

type input struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// EnterWorktreeTool creates a new git worktree and switches the working directory.
type EnterWorktreeTool struct{}

// New creates a new EnterWorktreeTool.
func New() *EnterWorktreeTool { return &EnterWorktreeTool{} }

func (t *EnterWorktreeTool) Name() string                { return "EnterWorktree" }
func (t *EnterWorktreeTool) Description() string          { return toolDescription }
func (t *EnterWorktreeTool) IsReadOnly() bool             { return false }
func (t *EnterWorktreeTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because creating worktrees modifies git state.
func (t *EnterWorktreeTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *EnterWorktreeTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("EnterWorktree", false, permCtx), nil
}

func (t *EnterWorktreeTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Branch) == "" {
		return tools.ErrorResult("required field \"branch\" is missing or empty"), nil
	}

	// Determine worktree path
	worktreePath := in.Path
	if worktreePath == "" {
		// Default: sibling directory named after the branch
		projectRoot := toolCtx.ProjectRoot
		if projectRoot == "" {
			projectRoot = toolCtx.WorkingDir
		}
		parent := filepath.Dir(projectRoot)
		worktreePath = filepath.Join(parent, in.Branch)
	}

	// Create the worktree via git CLI
	workDir := toolCtx.WorkingDir
	if workDir == "" {
		workDir = "."
	}

	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "worktree", "add", worktreePath, in.Branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("Failed to create worktree: %s\n%s", err.Error(), strings.TrimSpace(string(output)))), nil
	}

	// Return result with context modifier to switch working directory
	return &tools.ToolResult{
		Content: []tools.ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Created worktree at %s (branch: %s). Working directory switched.", worktreePath, in.Branch),
		}},
		ContextModifier: func(ctx *tools.ToolUseContext) {
			ctx.WorkingDir = worktreePath
		},
	}, nil
}
