package renderers

import (
	"fmt"
	"strings"
)

// RenderSystemText renders system messages in dim text.
func RenderSystemText(msg DisplayMessage, _ int) string {
	return dimStyle.Render(msg.Content)
}

// RenderSystemAPIError renders an API error with status code, retry info,
// and styled box. Matches Claude Code's SystemAPIErrorMessage.tsx.
func RenderSystemAPIError(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	statusCode := msg.Metadata["status_code"]
	retryIn := msg.Metadata["retry_in"]
	attempt := msg.Metadata["retry_attempt"]
	maxRetries := msg.Metadata["max_retries"]

	sb.WriteString(errorStyle.Render(msg.Content))

	if retryIn != "" {
		sb.WriteString("\n")
		retryMsg := fmt.Sprintf("Retrying in %s", retryIn)
		if attempt != "" && maxRetries != "" {
			retryMsg += fmt.Sprintf(" (attempt %s/%s)", attempt, maxRetries)
		}
		if statusCode != "" {
			retryMsg += fmt.Sprintf(" [%s]", statusCode)
		}
		sb.WriteString(dimStyle.Render(retryMsg))
	}
	return sb.String()
}

// RenderRateLimit renders a rate limit message with a progress bar showing
// fill/empty colors and a retry timer. Matches Claude Code's
// RateLimitMessage.tsx.
func RenderRateLimit(msg DisplayMessage, width int) string {
	var sb strings.Builder
	retryIn := msg.Metadata["retry_in"]
	if retryIn == "" {
		retryIn = "a few seconds"
	}

	sb.WriteString(warningStyle.Render("\u26A0 Rate limited"))
	sb.WriteString("\n")

	// Progress bar
	barWidth := width - 6
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 60 {
		barWidth = 60
	}
	filled := barWidth / 3 // Show ~33% filled as a visual placeholder
	empty := barWidth - filled
	bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", empty)
	sb.WriteString(paddingStyle.Render(warningStyle.Render(bar)))
	sb.WriteString("\n")

	sb.WriteString(paddingStyle.Render(
		dimStyle.Render(fmt.Sprintf("Retrying in %s...", retryIn)),
	))
	return sb.String()
}

// RenderCompactBoundary renders the "--- Context compacted ---" separator
// line. Matches Claude Code's CompactBoundaryMessage.tsx which shows
// "Conversation compacted (ctrl+o for history)".
func RenderCompactBoundary(_ DisplayMessage, width int) string {
	separator := "\u273B Conversation compacted (ctrl+o for history)"
	return dimStyle.Render(separator)
}

// RenderShutdown renders the shutdown/exit message.
func RenderShutdown(msg DisplayMessage, _ int) string {
	content := msg.Content
	if content == "" {
		content = "Session ended"
	}
	return dimStyle.Render(content)
}

// RenderHookProgress renders a hook execution progress indicator.
// Matches Claude Code's HookProgressMessage.tsx.
func RenderHookProgress(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	hookName := msg.Metadata["hook_name"]
	if hookName == "" {
		hookName = "hook"
	}
	status := msg.Metadata["status"]

	switch status {
	case "running":
		sb.WriteString(dimStyle.Render(fmt.Sprintf("\u23F3 Running %s...", hookName)))
	case "completed":
		sb.WriteString(successStyle.Render(fmt.Sprintf("\u2713 %s completed", hookName)))
	case "failed":
		sb.WriteString(errorStyle.Render(fmt.Sprintf("\u2717 %s failed", hookName)))
	default:
		sb.WriteString(dimStyle.Render(fmt.Sprintf("\u2022 %s", hookName)))
	}
	if msg.Content != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(msg.Content)))
	}
	return sb.String()
}

// RenderTaskAssignment renders a task assigned to an agent with agent
// name/color. Matches Claude Code's TaskAssignmentMessage.tsx.
func RenderTaskAssignment(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	agentName := msg.Metadata["agent_name"]
	if agentName == "" {
		agentName = "Agent"
	}
	taskID := msg.Metadata["task_id"]

	header := fmt.Sprintf("\u279C Task assigned to %s", agentName)
	if taskID != "" {
		header += fmt.Sprintf(" (%s)", taskID)
	}
	sb.WriteString(toolStyle.Render(header))
	if msg.Content != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}

// RenderAdvisor renders an advisor suggestion message.
func RenderAdvisor(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(systemStyle.Render("\u2139 Advisor"))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}
