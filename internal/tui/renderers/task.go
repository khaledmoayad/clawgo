package renderers

import (
	"fmt"
	"strings"
)

// Task state icons matching Claude Code's TaskAssignmentMessage.tsx and
// task rendering components. Each state gets a distinct Unicode indicator.
const (
	taskIconPending   = "\U0001F551" // clock face one o'clock
	taskIconRunning   = "\u23F3"     // hourglass flowing
	taskIconCompleted = "\u2714"     // checkmark
	taskIconFailed    = "\u2718"     // cross mark
	taskIconCanceled  = "\u2014"     // em dash
)

// RenderTaskStatus renders a task with its name, type, state icon, duration,
// and an optional output preview. Matches Claude Code's task rendering which
// shows task type (bash/agent/in-process), state indicator, and elapsed time.
func RenderTaskStatus(msg DisplayMessage, width int) string {
	var sb strings.Builder

	taskName := msg.Metadata["task_name"]
	if taskName == "" {
		taskName = "Task"
	}
	taskType := msg.Metadata["task_type"]
	status := msg.Metadata["status"]
	duration := msg.Metadata["duration"]

	// State icon
	var icon string
	var stateStyle func(...string) string
	switch status {
	case "pending":
		icon = taskIconPending
		stateStyle = dimStyle.Render
	case "running":
		icon = taskIconRunning
		stateStyle = toolStyle.Render
	case "completed":
		icon = taskIconCompleted
		stateStyle = successStyle.Render
	case "failed":
		icon = taskIconFailed
		stateStyle = errorStyle.Render
	case "canceled":
		icon = taskIconCanceled
		stateStyle = dimStyle.Render
	default:
		icon = "\u25CB" // open circle
		stateStyle = dimStyle.Render
	}

	// Header: icon + name + type
	header := fmt.Sprintf("%s %s", icon, taskName)
	if taskType != "" {
		header += dimStyle.Render(fmt.Sprintf(" (%s)", taskType))
	}
	sb.WriteString(stateStyle(header))

	// Duration if completed or failed
	if duration != "" && (status == "completed" || status == "failed") {
		sb.WriteString(dimStyle.Render(fmt.Sprintf(" [%s]", duration)))
	}

	// Output preview: first 3 lines
	if msg.Content != "" {
		sb.WriteString("\n")
		lines := strings.Split(msg.Content, "\n")
		maxPreview := 3
		if len(lines) > maxPreview {
			preview := strings.Join(lines[:maxPreview], "\n")
			remaining := len(lines) - maxPreview
			sb.WriteString(paddingStyle.Render(dimStyle.Render(preview)))
			sb.WriteString("\n")
			sb.WriteString(paddingStyle.Render(dimStyle.Render(
				fmt.Sprintf("... %d more lines", remaining),
			)))
		} else {
			sb.WriteString(paddingStyle.Render(dimStyle.Render(msg.Content)))
		}
	}

	return sb.String()
}

// RenderBackgroundTask renders a compact single-line background task indicator
// suitable for the status bar area. Format: "[bg] task_name: status"
func RenderBackgroundTask(msg DisplayMessage, _ int) string {
	taskName := msg.Metadata["task_name"]
	if taskName == "" {
		taskName = "task"
	}
	status := msg.Metadata["status"]
	if status == "" {
		status = "running"
	}

	// Compact single-line format matching Claude Code's background task display
	return dimStyle.Render(fmt.Sprintf("[bg] %s: %s", taskName, status))
}

// RenderTaskAssignmentDetail renders a detailed task assignment message with
// agent name (in color), task description, and expected behavior. This is the
// expanded version used in the message stream, complementing the basic
// RenderTaskAssignment in system.go.
func RenderTaskAssignmentDetail(msg DisplayMessage, _ int) string {
	var sb strings.Builder

	agentName := msg.Metadata["agent_name"]
	if agentName == "" {
		agentName = "Agent"
	}
	taskID := msg.Metadata["task_id"]
	index := parseAgentIndex(msg.Metadata)
	style := agentStyle(index)

	// Header with agent color
	header := fmt.Sprintf("\u279C Task assigned to %s", agentName)
	if taskID != "" {
		header += fmt.Sprintf(" (#%s)", taskID)
	}
	sb.WriteString(style.Render(header))

	// Task subject
	subject := msg.Metadata["subject"]
	if subject != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(subject))
	}

	// Task description
	if msg.Content != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(msg.Content)))
	}

	return sb.String()
}
