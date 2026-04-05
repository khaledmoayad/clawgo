// Package renderers provides 40+ specialized message type renderers matching
// Claude Code's components/messages/ directory. Each message type gets a
// dedicated render function that formats content with appropriate styling.
package renderers

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// DisplayMessage represents a message to be rendered with all metadata needed
// by the specialized renderer functions.
type DisplayMessage struct {
	// Type is the specific message type key (e.g. "assistant_text", "user_bash_output").
	Type string
	// Role is the message role: "user", "assistant", "system", "tool".
	Role string
	// Content is the primary text content of the message.
	Content string
	// ToolName is set for tool_use and tool_result messages.
	ToolName string
	// Metadata holds extra fields: file path, session ID, agent name, etc.
	Metadata map[string]string
	// DiffContent is true if content is known to be a unified diff.
	DiffContent bool
	// IsError is true if this message represents an error condition.
	IsError bool
	// IsCollapsed is true if this message should be rendered in collapsed form.
	IsCollapsed bool
}

// RenderFunc is the signature for all message type renderers.
// It receives the message and the terminal width, returning formatted output.
type RenderFunc func(msg DisplayMessage, width int) string

// RendererRegistry maps message type strings to render functions.
type RendererRegistry struct {
	renderers map[string]RenderFunc
}

// NewRegistry creates a new RendererRegistry populated with all 40+ renderers.
func NewRegistry() *RendererRegistry {
	r := &RendererRegistry{renderers: make(map[string]RenderFunc)}

	// Assistant renderers (4)
	r.Register("assistant_text", RenderAssistantText)
	r.Register("assistant_thinking", RenderAssistantThinking)
	r.Register("assistant_redacted_thinking", RenderAssistantRedactedThinking)
	r.Register("assistant_tool_use", RenderAssistantToolUse)

	// User renderers (14)
	r.Register("user_text", RenderUserText)
	r.Register("user_command", RenderUserCommand)
	r.Register("user_bash_input", RenderUserBashInput)
	r.Register("user_bash_output", RenderUserBashOutput)
	r.Register("user_image", RenderUserImage)
	r.Register("user_plan", RenderUserPlan)
	r.Register("user_prompt", RenderUserPrompt)
	r.Register("user_memory_input", RenderUserMemoryInput)
	r.Register("user_agent_notification", RenderUserAgentNotification)
	r.Register("user_teammate", RenderUserTeammate)
	r.Register("user_channel", RenderUserChannel)
	r.Register("user_local_command_output", RenderUserLocalCommandOutput)
	r.Register("user_resource_update", RenderUserResourceUpdate)
	r.Register("attachment", RenderAttachment)

	// Tool renderers (9)
	r.Register("tool_result", RenderToolResult)
	r.Register("tool_success", RenderToolSuccess)
	r.Register("tool_error", RenderToolError)
	r.Register("tool_rejected", RenderToolRejected)
	r.Register("tool_canceled", RenderToolCanceled)
	r.Register("plan_approval", RenderPlanApproval)
	r.Register("rejected_plan", RenderRejectedPlan)
	r.Register("grouped_tool_use", RenderGroupedToolUse)
	r.Register("collapsed_read_search", RenderCollapsedReadSearch)

	// System renderers (8)
	r.Register("system_text", RenderSystemText)
	r.Register("system_api_error", RenderSystemAPIError)
	r.Register("rate_limit", RenderRateLimit)
	r.Register("compact_boundary", RenderCompactBoundary)
	r.Register("shutdown", RenderShutdown)
	r.Register("hook_progress", RenderHookProgress)
	r.Register("task_assignment", RenderTaskAssignment)
	r.Register("advisor", RenderAdvisor)

	return r
}

// Register adds a renderer function for a message type.
func (r *RendererRegistry) Register(msgType string, fn RenderFunc) {
	r.renderers[msgType] = fn
}

// Render dispatches to the appropriate renderer for the message type.
// Falls back to genericRenderer for unknown types.
func (r *RendererRegistry) Render(msg DisplayMessage, width int) string {
	if fn, ok := r.renderers[msg.Type]; ok {
		return fn(msg, width)
	}
	return genericRenderer(msg, width)
}

// Count returns the number of registered renderers.
func (r *RendererRegistry) Count() int {
	return len(r.renderers)
}

// HasRenderer returns true if a renderer is registered for the given type.
func (r *RendererRegistry) HasRenderer(msgType string) bool {
	_, ok := r.renderers[msgType]
	return ok
}

// Color palette matching Claude Code visual style.
var (
	userColor      = lipgloss.Color("#6B9BD2")
	assistantColor = lipgloss.Color("#E8B86D")
	errorColor     = lipgloss.Color("#E06C75")
	successColor   = lipgloss.Color("#98C379")
	dimColor       = lipgloss.Color("#5C6370")
	thinkingColor  = lipgloss.Color("#C678DD")
	toolColor      = lipgloss.Color("#61AFEF")
	warningColor   = lipgloss.Color("#E5C07B")
	systemColor    = lipgloss.Color("#ABB2BF")

	userStyle      = lipgloss.NewStyle().Foreground(userColor).Bold(true)
	assistantStyle = lipgloss.NewStyle().Foreground(assistantColor).Bold(true)
	errorStyle     = lipgloss.NewStyle().Foreground(errorColor)
	successStyle   = lipgloss.NewStyle().Foreground(successColor)
	dimStyle       = lipgloss.NewStyle().Foreground(dimColor)
	thinkingStyle  = lipgloss.NewStyle().Foreground(thinkingColor).Italic(true)
	toolStyle      = lipgloss.NewStyle().Foreground(toolColor).Bold(true)
	warningStyle   = lipgloss.NewStyle().Foreground(warningColor)
	systemStyle    = lipgloss.NewStyle().Foreground(systemColor)

	paddingStyle = lipgloss.NewStyle().PaddingLeft(2)
)

// genericRenderer formats unknown message types with role label + content.
func genericRenderer(msg DisplayMessage, _ int) string {
	var sb strings.Builder
	label := msg.Role
	if label == "" {
		label = msg.Type
	}
	sb.WriteString(dimStyle.Render(fmt.Sprintf("[%s]", label)))
	sb.WriteString("\n")
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(msg.Content))
	}
	return sb.String()
}
