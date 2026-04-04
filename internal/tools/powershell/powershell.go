// Package powershell implements the PowerShell tool for executing PowerShell commands.
// Uses "powershell" on Windows and "pwsh" on Linux/macOS.
package powershell

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

const defaultTimeout = 120 * time.Second // 2 minutes
const maxTimeout = 600 * time.Second     // 10 minutes

type input struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"` // milliseconds, 0 = default
}

// PowerShellTool executes PowerShell commands.
type PowerShellTool struct{}

// New creates a new PowerShellTool.
func New() *PowerShellTool { return &PowerShellTool{} }

func (t *PowerShellTool) Name() string                { return "PowerShell" }
func (t *PowerShellTool) Description() string          { return toolDescription }
func (t *PowerShellTool) IsReadOnly() bool             { return false }
func (t *PowerShellTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns false because PowerShell commands can have side effects.
func (t *PowerShellTool) IsConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *PowerShellTool) CheckPermissions(_ context.Context, _ json.RawMessage, permCtx *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.CheckPermission("PowerShell", false, permCtx), nil
}

func (t *PowerShellTool) Call(ctx context.Context, inp json.RawMessage, toolCtx *tools.ToolUseContext) (*tools.ToolResult, error) {
	var in input
	if err := tools.ValidateInput(inp, &in); err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(in.Command) == "" {
		return tools.ErrorResult("required field \"command\" is missing or empty"), nil
	}

	// Determine timeout
	timeout := defaultTimeout
	if in.Timeout > 0 {
		timeout = time.Duration(in.Timeout) * time.Millisecond
		if timeout > maxTimeout {
			timeout = maxTimeout
		}
	}

	// Determine PowerShell executable based on platform
	psExe := "pwsh" // Linux/macOS use PowerShell Core
	if runtime.GOOS == "windows" {
		psExe = "powershell"
	}

	// Create command with timeout context
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	workDir := toolCtx.WorkingDir
	if workDir == "" {
		workDir = "."
	}

	cmd := exec.CommandContext(cmdCtx, psExe, "-Command", in.Command)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

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
