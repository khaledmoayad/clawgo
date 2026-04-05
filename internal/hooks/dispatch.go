package hooks

import (
	"context"
	"encoding/json"
)

// DispatchContext carries shared context for all hook dispatches.
// Create one at session start and pass it to every Dispatch* call.
type DispatchContext struct {
	SessionID      string
	ProjectRoot    string
	TranscriptPath string
	AgentID        string
	AgentType      string
	PermissionMode string
	Config         HooksConfig
}

// baseInput creates a HookInput with shared fields from DispatchContext.
func (dc *DispatchContext) baseInput(event HookEvent) *HookInput {
	return &HookInput{
		SessionID:      dc.SessionID,
		ProjectRoot:    dc.ProjectRoot,
		TranscriptPath: dc.TranscriptPath,
		AgentID:        dc.AgentID,
		AgentType:      dc.AgentType,
		PermissionMode: dc.PermissionMode,
		HookID:         GenerateHookID(),
		HookEventName:  string(event),
	}
}

// DispatchPreToolUse fires PreToolUse hooks before a tool executes.
// Returns (blocked bool, modifiedInput json.RawMessage, err error).
// If a hook returns JSON with decision="block", blocked is true.
// If a hook returns JSON with hookSpecificOutput containing updatedInput,
// the modified input is returned for the caller to use.
func DispatchPreToolUse(ctx context.Context, dc *DispatchContext, toolName string, toolInput json.RawMessage, toolUseID string) (bool, json.RawMessage, error) {
	input := dc.baseInput(PreToolUse)
	input.ToolName = toolName
	input.ToolInput = toolInput
	input.ToolUseID = toolUseID

	results, err := RunHooks(ctx, PreToolUse, input, dc.Config)
	if err != nil {
		return false, nil, err
	}

	var modifiedInput json.RawMessage
	for _, r := range results {
		// Non-zero exit code means block
		if r.ExitCode != 0 {
			return true, nil, nil
		}
		if r.Output != nil {
			// JSON "block" decision
			if r.Output.Decision == "block" {
				return true, nil, nil
			}
			// Check hookSpecificOutput for updatedInput
			if len(r.Output.HookSpecificOutput) > 0 {
				var specific struct {
					UpdatedInput json.RawMessage `json:"updatedInput,omitempty"`
				}
				if json.Unmarshal(r.Output.HookSpecificOutput, &specific) == nil && len(specific.UpdatedInput) > 0 {
					modifiedInput = specific.UpdatedInput
				}
			}
		}
	}

	return false, modifiedInput, nil
}

// DispatchPostToolUse fires PostToolUse hooks after successful tool execution.
// Errors are non-fatal (logged but not propagated).
func DispatchPostToolUse(ctx context.Context, dc *DispatchContext, toolName string, toolInput json.RawMessage, toolUseID string) error {
	input := dc.baseInput(PostToolUse)
	input.ToolName = toolName
	input.ToolInput = toolInput
	input.ToolUseID = toolUseID

	_, err := RunHooks(ctx, PostToolUse, input, dc.Config)
	return err
}

// DispatchPostToolUseFailure fires PostToolUseFailure hooks after failed tool execution.
func DispatchPostToolUseFailure(ctx context.Context, dc *DispatchContext, toolName string, toolInput json.RawMessage, toolUseID string, errMsg string) error {
	input := dc.baseInput(PostToolUseFailure)
	input.ToolName = toolName
	input.ToolInput = toolInput
	input.ToolUseID = toolUseID
	input.Error = errMsg

	_, err := RunHooks(ctx, PostToolUseFailure, input, dc.Config)
	return err
}

// DispatchNotification fires Notification hooks.
func DispatchNotification(ctx context.Context, dc *DispatchContext, title, message, notificationType string) error {
	input := dc.baseInput(Notification)
	input.Title = title
	input.Message = message
	input.NotificationType = notificationType

	_, err := RunHooks(ctx, Notification, input, dc.Config)
	return err
}

// DispatchUserPromptSubmit fires UserPromptSubmit hooks when user sends a prompt.
func DispatchUserPromptSubmit(ctx context.Context, dc *DispatchContext, prompt string) error {
	input := dc.baseInput(UserPromptSubmit)
	input.Prompt = prompt

	_, err := RunHooks(ctx, UserPromptSubmit, input, dc.Config)
	return err
}

// DispatchSessionStart fires SessionStart hooks at session start.
func DispatchSessionStart(ctx context.Context, dc *DispatchContext, source, model string) error {
	input := dc.baseInput(SessionStart)
	input.Source = source
	input.Model = model

	_, err := RunHooks(ctx, SessionStart, input, dc.Config)
	return err
}

// DispatchSessionEnd fires SessionEnd hooks at session end.
func DispatchSessionEnd(ctx context.Context, dc *DispatchContext, reason string) error {
	input := dc.baseInput(SessionEnd)
	input.ExitReason = reason

	_, err := RunHooks(ctx, SessionEnd, input, dc.Config)
	return err
}

// DispatchStop fires Stop hooks when the model stops generating.
func DispatchStop(ctx context.Context, dc *DispatchContext, stopHookActive bool, lastMessage string) error {
	input := dc.baseInput(Stop)
	input.StopHookActive = stopHookActive
	input.LastAssistantMessage = lastMessage

	_, err := RunHooks(ctx, Stop, input, dc.Config)
	return err
}

// DispatchStopFailure fires StopFailure hooks on API errors.
func DispatchStopFailure(ctx context.Context, dc *DispatchContext, stopError json.RawMessage, errorDetails, lastMessage string) error {
	input := dc.baseInput(StopFailure)
	input.StopError = stopError
	input.ErrorDetails = errorDetails
	input.LastAssistantMessage = lastMessage

	_, err := RunHooks(ctx, StopFailure, input, dc.Config)
	return err
}

// DispatchSubagentStart fires SubagentStart hooks when a subagent is spawned.
func DispatchSubagentStart(ctx context.Context, dc *DispatchContext, agentID, agentType string) error {
	input := dc.baseInput(SubagentStart)
	input.SubagentID = agentID
	input.SubagentAgentType = agentType

	_, err := RunHooks(ctx, SubagentStart, input, dc.Config)
	return err
}

// DispatchSubagentStop fires SubagentStop hooks when a subagent completes.
func DispatchSubagentStop(ctx context.Context, dc *DispatchContext, agentID, agentTranscriptPath, agentType string, stopHookActive bool, lastMessage string) error {
	input := dc.baseInput(SubagentStop)
	input.SubagentID = agentID
	input.AgentTranscriptPath = agentTranscriptPath
	input.SubagentAgentType = agentType
	input.StopHookActive = stopHookActive
	input.LastAssistantMessage = lastMessage

	_, err := RunHooks(ctx, SubagentStop, input, dc.Config)
	return err
}

// DispatchPreCompact fires PreCompact hooks before context compaction.
func DispatchPreCompact(ctx context.Context, dc *DispatchContext, trigger, customInstructions string) error {
	input := dc.baseInput(PreCompact)
	input.CompactTrigger = trigger
	input.CustomInstructions = customInstructions

	_, err := RunHooks(ctx, PreCompact, input, dc.Config)
	return err
}

// DispatchPostCompact fires PostCompact hooks after context compaction.
func DispatchPostCompact(ctx context.Context, dc *DispatchContext, trigger, compactSummary string) error {
	input := dc.baseInput(PostCompact)
	input.CompactTrigger = trigger
	input.CompactSummary = compactSummary

	_, err := RunHooks(ctx, PostCompact, input, dc.Config)
	return err
}

// DispatchPermissionRequest fires PermissionRequest hooks.
func DispatchPermissionRequest(ctx context.Context, dc *DispatchContext, toolName string, toolInput json.RawMessage, suggestions json.RawMessage) error {
	input := dc.baseInput(PermissionRequest)
	input.ToolName = toolName
	input.ToolInput = toolInput
	input.PermissionSuggestions = suggestions

	_, err := RunHooks(ctx, PermissionRequest, input, dc.Config)
	return err
}

// DispatchPermissionDenied fires PermissionDenied hooks.
func DispatchPermissionDenied(ctx context.Context, dc *DispatchContext, toolName string, toolInput json.RawMessage, toolUseID, reason string) error {
	input := dc.baseInput(PermissionDenied)
	input.ToolName = toolName
	input.ToolInput = toolInput
	input.ToolUseID = toolUseID
	input.Reason = reason

	_, err := RunHooks(ctx, PermissionDenied, input, dc.Config)
	return err
}

// DispatchSetup fires Setup hooks during initialization.
func DispatchSetup(ctx context.Context, dc *DispatchContext, trigger string) error {
	input := dc.baseInput(Setup)
	input.SetupTrigger = trigger

	_, err := RunHooks(ctx, Setup, input, dc.Config)
	return err
}

// DispatchTeammateIdle fires TeammateIdle hooks.
func DispatchTeammateIdle(ctx context.Context, dc *DispatchContext, teammateName, teamName string) error {
	input := dc.baseInput(TeammateIdle)
	input.TeammateName = teammateName
	input.TeamName = teamName

	_, err := RunHooks(ctx, TeammateIdle, input, dc.Config)
	return err
}

// DispatchTaskCreated fires TaskCreated hooks.
func DispatchTaskCreated(ctx context.Context, dc *DispatchContext, taskID, subject, description, teammateName, teamName string) error {
	input := dc.baseInput(TaskCreated)
	input.TaskID = taskID
	input.TaskSubject = subject
	input.TaskDescription = description
	input.TeammateName = teammateName
	input.TeamName = teamName

	_, err := RunHooks(ctx, TaskCreated, input, dc.Config)
	return err
}

// DispatchTaskCompleted fires TaskCompleted hooks.
func DispatchTaskCompleted(ctx context.Context, dc *DispatchContext, taskID, subject, description, teammateName, teamName string) error {
	input := dc.baseInput(TaskCompleted)
	input.TaskID = taskID
	input.TaskSubject = subject
	input.TaskDescription = description
	input.TeammateName = teammateName
	input.TeamName = teamName

	_, err := RunHooks(ctx, TaskCompleted, input, dc.Config)
	return err
}

// DispatchElicitation fires Elicitation hooks.
func DispatchElicitation(ctx context.Context, dc *DispatchContext, mcpServerName, message, mode, url, elicitationID string, requestedSchema json.RawMessage) error {
	input := dc.baseInput(Elicitation)
	input.MCPServerName = mcpServerName
	input.Message = message
	input.ElicitationMode = mode
	input.ElicitationURL = url
	input.ElicitationID = elicitationID
	input.RequestedSchema = requestedSchema

	_, err := RunHooks(ctx, Elicitation, input, dc.Config)
	return err
}

// DispatchElicitationResult fires ElicitationResult hooks.
func DispatchElicitationResult(ctx context.Context, dc *DispatchContext, mcpServerName, elicitationID, mode, action string, content json.RawMessage) error {
	input := dc.baseInput(ElicitationResult)
	input.MCPServerName = mcpServerName
	input.ElicitationID = elicitationID
	input.ElicitationMode = mode
	input.Action = action
	input.Content = content

	_, err := RunHooks(ctx, ElicitationResult, input, dc.Config)
	return err
}

// DispatchConfigChange fires ConfigChange hooks.
func DispatchConfigChange(ctx context.Context, dc *DispatchContext, source, filePath string) error {
	input := dc.baseInput(ConfigChange)
	input.ConfigSource = source
	input.FilePath = filePath

	_, err := RunHooks(ctx, ConfigChange, input, dc.Config)
	return err
}

// DispatchWorktreeCreate fires WorktreeCreate hooks.
func DispatchWorktreeCreate(ctx context.Context, dc *DispatchContext, name string) error {
	input := dc.baseInput(WorktreeCreate)
	input.WorktreeName = name

	_, err := RunHooks(ctx, WorktreeCreate, input, dc.Config)
	return err
}

// DispatchWorktreeRemove fires WorktreeRemove hooks.
func DispatchWorktreeRemove(ctx context.Context, dc *DispatchContext, worktreePath string) error {
	input := dc.baseInput(WorktreeRemove)
	input.WorktreePath = worktreePath

	_, err := RunHooks(ctx, WorktreeRemove, input, dc.Config)
	return err
}

// DispatchInstructionsLoaded fires InstructionsLoaded hooks.
func DispatchInstructionsLoaded(ctx context.Context, dc *DispatchContext, filePath, memoryType, loadReason string, globs []string, triggerFilePath, parentFilePath string) error {
	input := dc.baseInput(InstructionsLoaded)
	input.FilePath = filePath
	input.MemoryType = memoryType
	input.LoadReason = loadReason
	input.Globs = globs
	input.TriggerFilePath = triggerFilePath
	input.ParentFilePath = parentFilePath

	_, err := RunHooks(ctx, InstructionsLoaded, input, dc.Config)
	return err
}

// DispatchCwdChanged fires CwdChanged hooks.
func DispatchCwdChanged(ctx context.Context, dc *DispatchContext, oldCwd, newCwd string) error {
	input := dc.baseInput(CwdChanged)
	input.OldCwd = oldCwd
	input.NewCwd = newCwd

	_, err := RunHooks(ctx, CwdChanged, input, dc.Config)
	return err
}

// DispatchFileChanged fires FileChanged hooks.
func DispatchFileChanged(ctx context.Context, dc *DispatchContext, filePath, fileEvent string) error {
	input := dc.baseInput(FileChanged)
	input.FilePath = filePath
	input.FileEvent = fileEvent

	_, err := RunHooks(ctx, FileChanged, input, dc.Config)
	return err
}
