package tui

import (
	"strings"

	"github.com/khaledmoayad/clawgo/internal/tui/diff"
	"github.com/khaledmoayad/clawgo/internal/tui/render"
)

// DisplayMessage represents a message in the conversation display.
type DisplayMessage struct {
	Role        string // "user", "assistant", "tool_use", "tool_result", "thinking", "error", "command"
	Content     string
	ToolName    string // For tool_use/tool_result messages
	DiffContent bool   // True if content is known to be a unified diff (fast path)
}

// OutputModel manages the message display area.
type OutputModel struct {
	messages      []DisplayMessage
	streamingText strings.Builder // Buffer for current streaming response
	isStreaming   bool
	width         int // Terminal width for markdown/diff rendering
}

// SetWidth sets the terminal width used for rendering.
func (m *OutputModel) SetWidth(w int) {
	m.width = w
}

// NewOutputModel creates an output sub-model.
func NewOutputModel() OutputModel {
	return OutputModel{messages: make([]DisplayMessage, 0)}
}

// AddMessage adds a complete message to the display.
func (m *OutputModel) AddMessage(msg DisplayMessage) {
	m.messages = append(m.messages, msg)
}

// StartStreaming begins a new streaming response.
func (m *OutputModel) StartStreaming() {
	m.streamingText.Reset()
	m.isStreaming = true
}

// AppendStreaming appends text to the current streaming message.
func (m *OutputModel) AppendStreaming(text string) {
	m.streamingText.WriteString(text)
}

// FinishStreaming completes the streaming response and adds it as a message.
func (m *OutputModel) FinishStreaming() {
	if m.isStreaming {
		m.messages = append(m.messages, DisplayMessage{
			Role:    "assistant",
			Content: m.streamingText.String(),
		})
		m.streamingText.Reset()
		m.isStreaming = false
	}
}

// View renders all messages.
func (m OutputModel) View() string {
	var sb strings.Builder
	for _, msg := range m.messages {
		sb.WriteString(m.renderMessage(msg))
		sb.WriteString("\n")
	}
	// Render streaming text if active
	if m.isStreaming && m.streamingText.Len() > 0 {
		sb.WriteString(RoleLabel("assistant"))
		sb.WriteString("\n")
		sb.WriteString(MessagePadding.Render(m.streamingText.String()))
		sb.WriteString("\n")
	}
	return sb.String()
}

// renderMessage formats a single message for display.
func (m OutputModel) renderMessage(msg DisplayMessage) string {
	var sb strings.Builder
	switch msg.Role {
	case "user":
		sb.WriteString(RoleLabel("user"))
		sb.WriteString("\n")
		sb.WriteString(MessagePadding.Render(msg.Content))
	case "assistant":
		sb.WriteString(RoleLabel("assistant"))
		sb.WriteString("\n")
		// Render markdown for completed assistant messages; the renderer
		// handles its own indentation via glamour's style.
		width := m.width
		if width <= 0 {
			width = 80
		}
		rendered, err := render.RenderMarkdown(msg.Content, width)
		if err != nil {
			rendered = msg.Content
		}
		sb.WriteString(rendered)
	case "thinking":
		sb.WriteString(ThinkingStyle.Render("Thinking..."))
		sb.WriteString("\n")
		sb.WriteString(MessagePadding.Render(ThinkingStyle.Render(msg.Content)))
	case "tool_use":
		sb.WriteString(ToolNameStyle.Render("Tool: " + msg.ToolName))
		sb.WriteString("\n")
		sb.WriteString(MessagePadding.Render(DimStyle.Render(msg.Content)))
	case "tool_result":
		sb.WriteString(DimStyle.Render("Result:"))
		sb.WriteString("\n")
		// Render diffs with color coding; otherwise try syntax highlighting
		content := msg.Content
		if msg.DiffContent || diff.IsDiffContent(msg.Content) {
			content = diff.RenderDiff(msg.Content, m.width)
		} else {
			content = render.HighlightCodeDefault(msg.Content, "")
		}
		sb.WriteString(MessagePadding.Render(content))
	case "command":
		sb.WriteString(DimStyle.Render(msg.Content))
	case "error":
		sb.WriteString(ErrorStyle.Render("Error: " + msg.Content))
	}
	return sb.String()
}

// Messages returns all displayed messages.
func (m OutputModel) Messages() []DisplayMessage { return m.messages }

// Clear removes all messages.
func (m *OutputModel) Clear() {
	m.messages = m.messages[:0]
	m.streamingText.Reset()
	m.isStreaming = false
}
