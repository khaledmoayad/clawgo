package renderers

import (
	"fmt"
	"strings"
)

// maxBashOutputLines is the maximum number of lines to show for bash output
// before truncating with a "... N more lines" message.
const maxBashOutputLines = 50

// RenderUserText renders user text messages with the "You" role label.
func RenderUserText(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(userStyle.Render("You"))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}

// RenderUserCommand renders slash command input (e.g. "/compact") with the
// "You" label and the command text in dim monospace style.
func RenderUserCommand(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(userStyle.Render("You"))
	sb.WriteString("\n")
	sb.WriteString(paddingStyle.Render(dimStyle.Render(msg.Content)))
	return sb.String()
}

// RenderUserBashInput renders bash command input with "$ command" styling,
// matching Claude Code's UserBashInputMessage.tsx.
func RenderUserBashInput(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(userStyle.Render("You"))
	sb.WriteString("\n")
	sb.WriteString(paddingStyle.Render(dimStyle.Render("$ " + msg.Content)))
	return sb.String()
}

// RenderUserBashOutput renders shell output with line limit truncation.
// Truncates at 50 lines with "... N more lines" message.
func RenderUserBashOutput(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(dimStyle.Render("Output:"))
	sb.WriteString("\n")

	lines := strings.Split(msg.Content, "\n")
	if len(lines) > maxBashOutputLines {
		truncated := strings.Join(lines[:maxBashOutputLines], "\n")
		remaining := len(lines) - maxBashOutputLines
		sb.WriteString(paddingStyle.Render(truncated))
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(
			fmt.Sprintf("... %d more lines", remaining),
		)))
	} else {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}

// Note: RenderUserImage is defined in image.go which provides full OSC 8
// hyperlink support matching Claude Code's UserImageMessage.tsx.

// RenderUserPlan renders plan mode entry/exit markers.
func RenderUserPlan(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(userStyle.Render("You"))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	} else {
		sb.WriteString(paddingStyle.Render(dimStyle.Render("[Plan mode]")))
	}
	return sb.String()
}

// RenderUserPrompt renders the initial prompt display.
func RenderUserPrompt(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(userStyle.Render("You"))
	sb.WriteString("\n")
	sb.WriteString(paddingStyle.Render(msg.Content))
	return sb.String()
}

// RenderUserMemoryInput renders memory-related input with a memory icon.
func RenderUserMemoryInput(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(userStyle.Render("You"))
	sb.WriteString("\n")
	sb.WriteString(paddingStyle.Render(
		fmt.Sprintf("\U0001F4DD %s", msg.Content),
	))
	return sb.String()
}

// Note: RenderUserAgentNotification is defined in agent.go which provides full
// colored agent badge support matching Claude Code's UserAgentNotificationMessage.tsx.

// Note: RenderUserTeammate is defined in agent.go which provides full
// colored teammate badge support matching Claude Code's UserTeammateMessage.tsx.

// RenderUserChannel renders a channel message.
func RenderUserChannel(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	channelName := msg.Metadata["channel_name"]
	if channelName == "" {
		channelName = "Channel"
	}
	sb.WriteString(dimStyle.Render(fmt.Sprintf("#%s", channelName)))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}

// RenderUserLocalCommandOutput renders local command output.
func RenderUserLocalCommandOutput(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	sb.WriteString(dimStyle.Render("Command output:"))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}

// RenderUserResourceUpdate renders a resource update notification.
func RenderUserResourceUpdate(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	resourceName := msg.Metadata["resource_name"]
	if resourceName != "" {
		sb.WriteString(dimStyle.Render(fmt.Sprintf("\u21BB Resource updated: %s", resourceName)))
	} else {
		sb.WriteString(dimStyle.Render("\u21BB Resource updated"))
	}
	if msg.Content != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(msg.Content)))
	}
	return sb.String()
}

// RenderAttachment renders a file attachment indicator.
func RenderAttachment(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	filename := msg.Content
	if filename == "" {
		filename = msg.Metadata["filename"]
	}
	if filename == "" {
		filename = "attachment"
	}
	sb.WriteString(paddingStyle.Render(
		dimStyle.Render(fmt.Sprintf("\U0001F4CE %s", filename)),
	))
	return sb.String()
}
