package renderers

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// agentColors is the rotating palette for sub-agent distinction, matching
// Claude Code's theme.AgentColor() behavior.
var agentColors = []color.Color{
	lipgloss.Color("#61AFEF"), // blue
	lipgloss.Color("#E5C07B"), // yellow
	lipgloss.Color("#C678DD"), // magenta
	lipgloss.Color("#56B6C2"), // cyan
	lipgloss.Color("#98C379"), // green
	lipgloss.Color("#E06C75"), // red
	lipgloss.Color("#D19A66"), // orange
	lipgloss.Color("#BE5046"), // dark red
}

// AgentColor returns a color from the rotating agent palette based on index.
// Different agents get different colors for visual distinction in the TUI.
func AgentColor(index int) color.Color {
	if index < 0 {
		index = 0
	}
	return agentColors[index%len(agentColors)]
}

// agentStyle returns a lipgloss style for the given agent index.
func agentStyle(index int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(AgentColor(index)).Bold(true)
}

// parseAgentIndex extracts the agent index from metadata, defaulting to 0.
func parseAgentIndex(meta map[string]string) int {
	idxStr := meta["agent_index"]
	if idxStr == "" {
		return 0
	}
	idx := 0
	for _, c := range idxStr {
		if c >= '0' && c <= '9' {
			idx = idx*10 + int(c-'0')
		}
	}
	return idx
}

// RenderAgentOutput renders agent/sub-agent output messages with a colored
// agent name badge. Matches Claude Code's agent message rendering where each
// agent gets a distinct color from the rotating palette.
func RenderAgentOutput(msg DisplayMessage, width int) string {
	var sb strings.Builder

	agentName := msg.Metadata["agent_name"]
	if agentName == "" {
		agentName = "Agent"
	}
	index := parseAgentIndex(msg.Metadata)
	style := agentStyle(index)

	// Agent name badge with optional worker ID
	workerID := msg.Metadata["worker_id"]
	if workerID != "" {
		sb.WriteString(style.Render(fmt.Sprintf("[%s (worker %s)]", agentName, workerID)))
	} else {
		sb.WriteString(style.Render(fmt.Sprintf("[%s]", agentName)))
	}
	sb.WriteString("\n")

	// Content
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}

	return sb.String()
}

// RenderSwarmWorker renders a swarm worker status line with colored badge
// and status indicator (running spinner dots, completed check, failed x).
// Used in the status bar area for multi-agent coordination displays.
func RenderSwarmWorker(msg DisplayMessage, _ int) string {
	var sb strings.Builder

	workerName := msg.Metadata["worker_name"]
	if workerName == "" {
		workerName = "Worker"
	}
	index := parseAgentIndex(msg.Metadata)
	style := agentStyle(index)

	// Status icon
	status := msg.Metadata["status"]
	var statusIcon string
	switch status {
	case "running":
		statusIcon = "\u23F3" // hourglass
	case "completed":
		statusIcon = "\u2714" // checkmark
	case "failed":
		statusIcon = "\u2718" // x mark
	case "canceled":
		statusIcon = "\u2014" // em dash
	default:
		statusIcon = "\u25CB" // circle
	}

	// Worker badge + status
	sb.WriteString(style.Render(fmt.Sprintf("%s %s", statusIcon, workerName)))

	// Current task description
	taskDesc := msg.Metadata["task_description"]
	if taskDesc != "" {
		sb.WriteString(dimStyle.Render(": " + taskDesc))
	}

	return sb.String()
}

// RenderAgentNotification renders agent notification messages with a dim
// border and colored status indicator. Matches Claude Code's
// UserAgentNotificationMessage.tsx which shows a colored circle + summary.
func RenderAgentNotification(msg DisplayMessage, _ int) string {
	var sb strings.Builder

	// Extract status and summary from message content/metadata
	status := msg.Metadata["status"]
	summary := msg.Metadata["summary"]
	if summary == "" {
		summary = msg.Content
	}
	if summary == "" {
		return ""
	}

	// Status color matches TS: completed=green, failed=red, killed=warning
	var statusColor color.Color
	switch status {
	case "completed":
		statusColor = lipgloss.Color("#98C379") // success/green
	case "failed":
		statusColor = lipgloss.Color("#E06C75") // error/red
	case "killed":
		statusColor = lipgloss.Color("#E5C07B") // warning/yellow
	default:
		statusColor = lipgloss.Color("#ABB2BF") // text/default
	}
	circleStyle := lipgloss.NewStyle().Foreground(statusColor)

	// BLACK_CIRCLE + summary matching TS exactly
	sb.WriteString(circleStyle.Render("\u25CF"))
	sb.WriteString(" ")
	sb.WriteString(summary)

	return sb.String()
}

// RenderUserAgentNotification is the registry-compatible wrapper.
func RenderUserAgentNotification(msg DisplayMessage, width int) string {
	return RenderAgentNotification(msg, width)
}

// RenderTeammate renders teammate messages with a colored name badge and
// message content. In collapsed mode, shows only the first line with an
// expand hint. Matches Claude Code's UserTeammateMessage.tsx.
func RenderTeammate(msg DisplayMessage, width int) string {
	var sb strings.Builder

	teammateName := msg.Metadata["teammate_id"]
	if teammateName == "" {
		teammateName = "Teammate"
	}
	index := parseAgentIndex(msg.Metadata)
	style := agentStyle(index)

	// Teammate name badge
	sb.WriteString(style.Render(fmt.Sprintf("\u25B6 %s", teammateName)))
	sb.WriteString("\n")

	if msg.IsCollapsed {
		// Collapsed mode: first line + ellipsis
		firstLine := msg.Content
		if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
			firstLine = firstLine[:idx]
		}
		maxLen := width - 10
		if maxLen > 0 && len(firstLine) > maxLen {
			firstLine = firstLine[:maxLen]
		}
		sb.WriteString(paddingStyle.Render(dimStyle.Render(firstLine + "...")))
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render("[Ctrl+O to expand]")))
	} else if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}

	return sb.String()
}

// RenderUserTeammate is the registry-compatible wrapper.
func RenderUserTeammate(msg DisplayMessage, width int) string {
	return RenderTeammate(msg, width)
}
