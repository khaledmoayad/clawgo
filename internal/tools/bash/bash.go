// Package bash implements the BashTool for executing shell commands.
// It captures stdout, stderr, and exit codes, and enforces timeouts.
package bash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/classify"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

const defaultTimeout = 120 * time.Second // 2 minutes
const maxTimeout = 600 * time.Second     // 10 minutes

// BashTool executes shell commands via bash -c.
type BashTool struct{}

// New creates a new BashTool.
func New() *BashTool { return &BashTool{} }

func (t *BashTool) Name() string                { return "Bash" }
func (t *BashTool) Description() string          { return ToolDescription() }
func (t *BashTool) IsReadOnly() bool             { return false }
func (t *BashTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe uses the AST-based bash classifier to determine
// if the command is safe to run concurrently. Read-only and safe commands
// can execute in parallel; all others are serialized.
func (t *BashTool) IsConcurrencySafe(input json.RawMessage) bool {
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return false
	}
	result, _ := classify.ClassifyBashCommand(in.Command)
	return result == classify.ClassifyReadOnly || result == classify.ClassifySafe
}

// CheckPermissions uses the AST-based bash classifier to determine
// the permission level. Read-only/safe commands are treated as read-only
// for permission checks; denied commands are blocked; everything else
// requires the standard write-tool permission check.
func (t *BashTool) CheckPermissions(_ context.Context, input json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return permissions.Ask, nil
	}
	result, _ := classify.ClassifyBashCommand(in.Command)
	switch result {
	case classify.ClassifyReadOnly, classify.ClassifySafe:
		return permissions.CheckPermissionWithRules("Bash", true, permCtx, permCtx.ToolRules), nil
	case classify.ClassifyDeny:
		return permissions.Deny, nil
	default:
		return permissions.CheckPermissionWithRules("Bash", false, permCtx, permCtx.ToolRules), nil
	}
}

func (t *BashTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	data, err := tools.ParseRawInput(inp)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	command, err := tools.RequireString(data, "command")
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(command) == "" {
		return tools.ErrorResult("required field \"command\" is missing or empty"), nil
	}

	// Parse run_in_background with semantic coercion (handles string "true"/"false")
	runInBackground, err := tools.OptionalSemanticBool(data, "run_in_background", false)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("invalid \"run_in_background\" parameter: %s", err.Error())), nil
	}

	// Parse dangerouslyDisableSandbox (schema-only for now, actual sandbox bypass in Phase 11)
	_, err = tools.OptionalSemanticBool(data, "dangerouslyDisableSandbox", false)
	if err != nil {
		return tools.ErrorResult(fmt.Sprintf("invalid \"dangerouslyDisableSandbox\" parameter: %s", err.Error())), nil
	}

	// Parse description (metadata only, not used for execution)
	_ = tools.OptionalString(data, "description", "")

	// Parse timeout
	timeoutMs := tools.OptionalInt(data, "timeout", 0)

	// Handle background execution
	if runInBackground {
		return t.callBackground(ctx, command, toolCtx)
	}

	// Determine timeout
	timeout := defaultTimeout
	if timeoutMs > 0 {
		timeout = time.Duration(timeoutMs) * time.Millisecond
		if timeout > maxTimeout {
			timeout = maxTimeout
		}
	}

	// Create command with timeout context
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use the working directory from tool context
	workDir := toolCtx.WorkingDir
	if workDir == "" {
		workDir = "."
	}

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", command)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	// Build output: combine stdout and stderr
	var output strings.Builder
	if stdout.Len() > 0 {
		output.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString("STDERR:\n")
		output.WriteString(stderr.String())
	}

	if err != nil {
		// Check if timeout
		if cmdCtx.Err() == context.DeadlineExceeded {
			return tools.ErrorResult(fmt.Sprintf("Command timed out after %s", timeout)), nil
		}
		// Get exit code
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		result := output.String()
		if result == "" {
			result = err.Error()
		}
		return &tools.ToolResult{
			Content: []tools.ContentBlock{{Type: "text", Text: fmt.Sprintf("Exit code: %d\n%s", exitCode, result)}},
			IsError: true,
		}, nil
	}

	resultText := output.String()
	if resultText == "" {
		resultText = "(no output)"
	}
	return tools.TextResult(resultText), nil
}

// callBackground starts a command in a background goroutine and returns
// immediately with a task ID. The output can be retrieved later via TaskOutput.
func (t *BashTool) callBackground(ctx context.Context, command string, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	if toolCtx.TaskStore == nil {
		return tools.ErrorResult("background execution not available: task store not configured"), nil
	}

	task := toolCtx.TaskStore.Create(command, "local_bash")
	_ = toolCtx.TaskStore.Update(task.ID, "running", "")

	workDir := toolCtx.WorkingDir
	if workDir == "" {
		workDir = "."
	}

	go func() {
		cmd := exec.CommandContext(ctx, "bash", "-c", command)
		cmd.Dir = workDir

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		var output strings.Builder
		if stdout.Len() > 0 {
			output.WriteString(stdout.String())
		}
		if stderr.Len() > 0 {
			if output.Len() > 0 {
				output.WriteString("\n")
			}
			output.WriteString("STDERR:\n")
			output.WriteString(stderr.String())
		}

		if err != nil {
			exitCode := -1
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			result := output.String()
			if result == "" {
				result = err.Error()
			}
			_ = toolCtx.TaskStore.Update(task.ID, "failed", fmt.Sprintf("Exit code: %d\n%s", exitCode, result))
		} else {
			result := output.String()
			if result == "" {
				result = "(no output)"
			}
			_ = toolCtx.TaskStore.Update(task.ID, "completed", result)
		}
	}()

	return tools.TextResult(fmt.Sprintf("Command started in background. Task ID: %s\nUse TaskOutput to check results.", task.ID)), nil
}
