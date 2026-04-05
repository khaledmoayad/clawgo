package renderers

import (
	"fmt"
	"strings"
)

// RenderToolResult renders a generic tool result with content.
// Handles truncation for large results.
func RenderToolResult(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	name := msg.ToolName
	if name == "" {
		name = "Tool"
	}
	sb.WriteString(dimStyle.Render(fmt.Sprintf("%s result:", name)))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}

// RenderToolSuccess renders a success indicator with a green checkmark
// and result summary.
func RenderToolSuccess(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	name := msg.ToolName
	if name == "" {
		name = "Tool"
	}
	sb.WriteString(successStyle.Render(fmt.Sprintf("\u2713 %s", name)))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}

// RenderToolError renders an error indicator with a red X and error message.
func RenderToolError(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	name := msg.ToolName
	if name == "" {
		name = "Tool"
	}
	sb.WriteString(errorStyle.Render(fmt.Sprintf("\u2717 %s", name)))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(errorStyle.Render(msg.Content)))
	}
	return sb.String()
}

// RenderToolRejected renders a "Rejected" message in warning color with reason.
func RenderToolRejected(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	name := msg.ToolName
	if name == "" {
		name = "Tool"
	}
	sb.WriteString(warningStyle.Render(fmt.Sprintf("\u26A0 %s rejected", name)))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(warningStyle.Render(msg.Content)))
	}
	return sb.String()
}

// RenderToolCanceled renders a "Canceled" message in dim text.
func RenderToolCanceled(msg DisplayMessage, _ int) string {
	name := msg.ToolName
	if name == "" {
		name = "Tool"
	}
	return dimStyle.Render(fmt.Sprintf("\u2014 %s canceled", name))
}

// RenderPlanApproval renders plan approval with accept/reject display.
func RenderPlanApproval(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	status := msg.Metadata["status"]
	switch status {
	case "approved":
		sb.WriteString(successStyle.Render("\u2713 Plan approved"))
	case "rejected":
		sb.WriteString(warningStyle.Render("\u2717 Plan rejected"))
	default:
		sb.WriteString(warningStyle.Render("\u2753 Plan approval pending"))
	}
	if msg.Content != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}

// RenderRejectedPlan renders a rejected plan entry.
func RenderRejectedPlan(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(warningStyle.Render("\u2717 Plan rejected"))
	if msg.Content != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(msg.Content)))
	}
	return sb.String()
}

// RenderGroupedToolUse renders multiple tool uses collapsed into a summary
// line, matching Claude Code's GroupedToolUseContent.tsx.
func RenderGroupedToolUse(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	count := msg.Metadata["count"]
	if count == "" {
		count = "multiple"
	}
	sb.WriteString(toolStyle.Render(fmt.Sprintf("\u25B6 %s tool uses", count)))
	if msg.Content != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(msg.Content)))
	}
	return sb.String()
}

// RenderCollapsedReadSearch renders collapsed read/search results with an
// expand hint, matching Claude Code's CollapsedReadSearchContent.tsx.
func RenderCollapsedReadSearch(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	name := msg.ToolName
	if name == "" {
		name = "Read/Search"
	}
	sb.WriteString(dimStyle.Render(fmt.Sprintf("\u25B7 %s (collapsed)", name)))
	if msg.Content != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(msg.Content)))
	}
	return sb.String()
}
