package renderers

import (
	"fmt"
	"strings"
)

// RenderAssistantText renders assistant text messages with the "Claude" role
// label and markdown-formatted content.
func RenderAssistantText(msg DisplayMessage, width int) string {
	var sb strings.Builder
	sb.WriteString(assistantStyle.Render("Claude"))
	sb.WriteString("\n")
	if msg.Content != "" {
		// Use padding for consistent indentation; content is expected to be
		// pre-rendered markdown by the caller or rendered inline.
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}

// RenderAssistantThinking renders thinking blocks with the characteristic
// "Thinking..." header in purple/italic. Matches Claude Code's
// AssistantThinkingMessage.tsx which shows "\u2234 Thinking" with dim italic.
func RenderAssistantThinking(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	if msg.IsCollapsed {
		// Collapsed thinking: just the label, matching TS "∴ Thinking" + CtrlOToExpand
		sb.WriteString(thinkingStyle.Render("\u2234 Thinking"))
		return sb.String()
	}
	// Expanded thinking: header + indented thinking text
	sb.WriteString(thinkingStyle.Render("\u2234 Thinking\u2026"))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(thinkingStyle.Render(msg.Content)))
	}
	return sb.String()
}

// RenderAssistantRedactedThinking renders redacted thinking blocks as
// "[Redacted thinking]" in dim text, matching Claude Code's behavior for
// redacted_thinking block types.
func RenderAssistantRedactedThinking(_ DisplayMessage, _ int) string {
	return dimStyle.Render("[Redacted thinking]")
}

// RenderAssistantToolUse renders tool use messages with the tool name in
// blue/bold and the JSON input formatted below. Matches Claude Code's
// AssistantToolUseMessage.tsx.
func RenderAssistantToolUse(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	name := msg.ToolName
	if name == "" {
		name = "unknown tool"
	}
	sb.WriteString(toolStyle.Render(fmt.Sprintf("\u25b6 %s", name)))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(dimStyle.Render(msg.Content)))
	}
	return sb.String()
}
