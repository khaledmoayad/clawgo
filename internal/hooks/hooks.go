package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"
)

// DefaultTimeout is the default hook execution timeout in seconds.
const DefaultTimeout = 30

// globalDisabled prevents all hooks from running when true.
// Set via SetGlobalDisabled, respects the disableAllHooks setting.
var globalDisabled bool

// managedHooksOnly restricts execution to managed hooks only when true.
// Set via SetManagedHooksOnly, respects the allowManagedHooksOnly setting.
var managedHooksOnly bool

// SetGlobalDisabled enables or disables all hook execution globally.
func SetGlobalDisabled(disabled bool) { globalDisabled = disabled }

// SetManagedHooksOnly restricts execution to managed hooks only.
func SetManagedHooksOnly(only bool) { managedHooksOnly = only }

// IsGlobalDisabled returns the current global disabled state.
func IsGlobalDisabled() bool { return globalDisabled }

// IsManagedHooksOnly returns the current managed hooks only state.
func IsManagedHooksOnly() bool { return managedHooksOnly }

// onceTracker records which once-hooks have already been executed.
// Key format: "{event}:{matcher}:{command}"
var onceTracker sync.Map

// ResetOnceTracker clears the once-hook tracker. Useful for testing.
func ResetOnceTracker() {
	onceTracker.Range(func(key, _ any) bool {
		onceTracker.Delete(key)
		return true
	})
}

// onceKey builds the dedup key for a once-hook.
func onceKey(event HookEvent, matcher string, cmd HookCommand) string {
	return fmt.Sprintf("%s:%s:%s", event, matcher, cmd.Command)
}

// RunHooks dispatches hooks for the given event and input.
//
// Steps:
//  1. Check global disabled state.
//  2. Look up config[event] for matchers.
//  3. Filter matchers by input.ToolName.
//  4. For each matching HookCommand of type "command": execute via
//     exec.CommandContext with configurable timeout. Environment variables
//     matching Claude Code are set (HOOK_EVENT, TOOL_NAME, TOOL_INPUT,
//     SESSION_ID, CWD, TRANSCRIPT_PATH, AGENT_ID, HOOK_ID, ARGUMENTS,
//     HOOK_STATUS_MESSAGE).
//  5. For non-"command" types, return an error result.
//  6. Return slice of HookResult.
func RunHooks(ctx context.Context, event HookEvent, input *HookInput, config HooksConfig) ([]HookResult, error) {
	if globalDisabled {
		return nil, nil
	}

	matchers, ok := config[event]
	if !ok || len(matchers) == 0 {
		return nil, nil
	}

	toolName := ""
	if input != nil {
		toolName = input.ToolName
	}

	commands := FilterMatchers(matchers, toolName)
	if len(commands) == 0 {
		return nil, nil
	}

	// Serialize input for ARGUMENTS env var
	var argsJSON []byte
	if input != nil {
		var err error
		argsJSON, err = json.Marshal(input)
		if err != nil {
			argsJSON = []byte("{}")
		}
	} else {
		argsJSON = []byte("{}")
	}

	var results []HookResult
	var asyncWg sync.WaitGroup

	for _, hk := range commands {
		// Handle Once flag: skip if already executed
		if hk.Once {
			key := onceKey(event, "", hk)
			if _, loaded := onceTracker.LoadOrStore(key, true); loaded {
				continue
			}
		}

		// Only "command" type is implemented
		if hk.Type != CommandType {
			results = append(results, HookResult{
				ExitCode: 1,
				Stderr:   fmt.Sprintf("Hook type %q not yet implemented", hk.Type),
			})
			continue
		}

		if hk.Async {
			asyncWg.Add(1)
			go func(hook HookCommand) {
				defer asyncWg.Done()
				_ = executeCommandHook(ctx, event, hook, argsJSON, input)
			}(hk)
			continue
		}

		result := executeCommandHook(ctx, event, hk, argsJSON, input)
		results = append(results, result)
	}

	// Do not wait for async hooks -- they run fire-and-forget
	return results, nil
}

// executeCommandHook runs a single "command" hook and returns the result.
// Sets all 10 environment variables matching Claude Code behavior.
func executeCommandHook(ctx context.Context, event HookEvent, hk HookCommand, argsJSON []byte, input *HookInput) HookResult {
	timeout := hk.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	hookCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	shell := hk.Shell
	if shell == "" {
		shell = "bash"
	}

	cmd := exec.CommandContext(hookCtx, shell, "-c", hk.Command)

	// Extract fields from input
	sessionID := ""
	toolName := ""
	toolInput := ""
	cwd := ""
	transcriptPath := ""
	agentID := ""
	hookID := ""
	if input != nil {
		sessionID = input.SessionID
		toolName = input.ToolName
		cwd = input.ProjectRoot
		transcriptPath = input.TranscriptPath
		agentID = input.AgentID
		hookID = input.HookID
		if len(input.ToolInput) > 0 {
			toolInput = string(input.ToolInput)
		}
	}

	// Build environment: inherit parent process env + hook-specific vars.
	// Matches the TS subprocessEnv() + hook env pattern.
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HOOK_EVENT=%s", event),
		fmt.Sprintf("TOOL_NAME=%s", toolName),
		fmt.Sprintf("TOOL_INPUT=%s", toolInput),
		fmt.Sprintf("SESSION_ID=%s", sessionID),
		fmt.Sprintf("CWD=%s", cwd),
		fmt.Sprintf("TRANSCRIPT_PATH=%s", transcriptPath),
		fmt.Sprintf("AGENT_ID=%s", agentID),
		fmt.Sprintf("HOOK_ID=%s", hookID),
		fmt.Sprintf("ARGUMENTS=%s", string(argsJSON)),
		fmt.Sprintf("HOOK_STATUS_MESSAGE=%s", hk.StatusMessage),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Context timeout or other exec failure
			exitCode = 1
		}
	}

	result := HookResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}

	// Attempt to parse hook stdout as JSON output (hookJSONOutputSchema).
	// If parsing fails, leave Output nil -- plain text stdout is valid.
	if stdoutStr := stdout.String(); stdoutStr != "" {
		var output HookJSONOutput
		if json.Unmarshal([]byte(stdoutStr), &output) == nil {
			// Only set Output if it parsed as meaningful JSON (not just "null" or empty)
			if output.Decision != "" || output.Async || output.Continue != nil ||
				output.SuppressOutput || output.StopReason != "" ||
				output.SystemMessage != "" || output.Reason != "" ||
				len(output.HookSpecificOutput) > 0 {
				result.Output = &output
			}
		}
	}

	return result
}

// RunPreToolHook executes PreToolUse hooks and returns whether the tool
// should be blocked (any hook returned non-zero exit code or JSON "block" decision).
func RunPreToolHook(
	ctx context.Context,
	toolName string,
	toolInput json.RawMessage,
	sessionID, projectRoot string,
	config HooksConfig,
) (blocked bool, err error) {
	input := &HookInput{
		ToolName:    toolName,
		ToolInput:   toolInput,
		SessionID:   sessionID,
		ProjectRoot: projectRoot,
	}

	results, err := RunHooks(ctx, PreToolUse, input, config)
	if err != nil {
		return false, err
	}

	for _, r := range results {
		if r.ExitCode != 0 {
			return true, nil
		}
		// Also check JSON output for block decision
		if r.Output != nil && r.Output.Decision == "block" {
			return true, nil
		}
	}

	return false, nil
}

// RunPostToolHook executes PostToolUse hooks. It is fire-and-forget:
// errors are logged but do not propagate.
func RunPostToolHook(
	ctx context.Context,
	toolName string,
	toolInput json.RawMessage,
	sessionID, projectRoot string,
	config HooksConfig,
) {
	input := &HookInput{
		ToolName:    toolName,
		ToolInput:   toolInput,
		SessionID:   sessionID,
		ProjectRoot: projectRoot,
	}

	_, err := RunHooks(ctx, PostToolUse, input, config)
	if err != nil {
		log.Printf("hooks: PostToolUse error (non-fatal): %v", err)
	}
}
