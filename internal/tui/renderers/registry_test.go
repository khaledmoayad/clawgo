package renderers

import (
	"strings"
	"testing"
)

func TestRegistryDispatch(t *testing.T) {
	r := NewRegistry()

	// Verify a known type dispatches to the correct renderer (not generic).
	msg := DisplayMessage{
		Type:    "assistant_text",
		Role:    "assistant",
		Content: "Hello, world!",
	}
	result := r.Render(msg, 80)
	if !strings.Contains(result, "Claude") {
		t.Errorf("expected assistant_text renderer to produce 'Claude' label, got: %s", result)
	}
}

func TestRegistryFallback(t *testing.T) {
	r := NewRegistry()

	// Unknown type should use generic renderer.
	msg := DisplayMessage{
		Type:    "unknown_type_xyz",
		Role:    "mystery",
		Content: "fallback content",
	}
	result := r.Render(msg, 80)
	if !strings.Contains(result, "mystery") {
		t.Errorf("expected generic renderer to show role label, got: %s", result)
	}
	if !strings.Contains(result, "fallback content") {
		t.Errorf("expected generic renderer to show content, got: %s", result)
	}
}

func TestRendererCount(t *testing.T) {
	r := NewRegistry()
	count := r.Count()
	if count < 35 {
		t.Errorf("expected at least 35 registered renderers, got %d", count)
	}
}

func TestAssistantTextRenderer(t *testing.T) {
	msg := DisplayMessage{
		Type:    "assistant_text",
		Role:    "assistant",
		Content: "This is **bold** text",
	}
	result := RenderAssistantText(msg, 80)
	// Should contain "Claude" label and the content
	if !strings.Contains(result, "Claude") {
		t.Errorf("expected 'Claude' label, got: %s", result)
	}
	if !strings.Contains(result, "bold") {
		t.Errorf("expected content with 'bold', got: %s", result)
	}
}

func TestAssistantThinkingRenderer(t *testing.T) {
	msg := DisplayMessage{
		Type:    "assistant_thinking",
		Content: "Let me think about this...",
	}
	result := RenderAssistantThinking(msg, 80)
	if !strings.Contains(result, "Thinking") {
		t.Errorf("expected 'Thinking' header, got: %s", result)
	}

	// Test collapsed mode
	collapsed := DisplayMessage{
		Type:        "assistant_thinking",
		Content:     "thinking...",
		IsCollapsed: true,
	}
	resultCollapsed := RenderAssistantThinking(collapsed, 80)
	if !strings.Contains(resultCollapsed, "Thinking") {
		t.Errorf("expected collapsed thinking label, got: %s", resultCollapsed)
	}
	// Collapsed should NOT contain the thinking content
	if strings.Contains(resultCollapsed, "thinking...") {
		t.Errorf("collapsed thinking should not show content, got: %s", resultCollapsed)
	}
}

func TestToolErrorRenderer(t *testing.T) {
	msg := DisplayMessage{
		Type:     "tool_error",
		ToolName: "BashTool",
		Content:  "command not found: foobar",
		IsError:  true,
	}
	result := RenderToolError(msg, 80)
	// Should contain error styling indicator
	if !strings.Contains(result, "BashTool") {
		t.Errorf("expected tool name 'BashTool', got: %s", result)
	}
	if !strings.Contains(result, "command not found") {
		t.Errorf("expected error content, got: %s", result)
	}
}

func TestToolSuccessRenderer(t *testing.T) {
	msg := DisplayMessage{
		Type:     "tool_success",
		ToolName: "FileReadTool",
		Content:  "Read 42 lines",
	}
	result := RenderToolSuccess(msg, 80)
	if !strings.Contains(result, "FileReadTool") {
		t.Errorf("expected tool name, got: %s", result)
	}
	if !strings.Contains(result, "\u2713") {
		t.Errorf("expected checkmark, got: %s", result)
	}
}

func TestRateLimitRenderer(t *testing.T) {
	msg := DisplayMessage{
		Type: "rate_limit",
		Metadata: map[string]string{
			"retry_in": "30 seconds",
		},
	}
	result := RenderRateLimit(msg, 80)
	// Should contain progress bar elements (filled/empty blocks)
	if !strings.Contains(result, "\u2588") && !strings.Contains(result, "\u2591") {
		t.Errorf("expected progress bar elements, got: %s", result)
	}
	if !strings.Contains(result, "Rate limited") {
		t.Errorf("expected rate limit header, got: %s", result)
	}
}

func TestCompactBoundaryRenderer(t *testing.T) {
	msg := DisplayMessage{Type: "compact_boundary"}
	result := RenderCompactBoundary(msg, 80)
	if !strings.Contains(result, "compacted") {
		t.Errorf("expected compact boundary text, got: %s", result)
	}
}

func TestUserBashOutputTruncation(t *testing.T) {
	// Create output with more than 50 lines
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line content"
	}
	msg := DisplayMessage{
		Type:    "user_bash_output",
		Content: strings.Join(lines, "\n"),
	}
	result := RenderUserBashOutput(msg, 80)
	if !strings.Contains(result, "more lines") {
		t.Errorf("expected truncation message, got: %s", result)
	}
}

func TestHasRenderer(t *testing.T) {
	r := NewRegistry()
	if !r.HasRenderer("assistant_text") {
		t.Error("expected HasRenderer to return true for 'assistant_text'")
	}
	if r.HasRenderer("nonexistent_type") {
		t.Error("expected HasRenderer to return false for 'nonexistent_type'")
	}
}

func TestRedactedThinkingRenderer(t *testing.T) {
	msg := DisplayMessage{Type: "assistant_redacted_thinking"}
	result := RenderAssistantRedactedThinking(msg, 80)
	if !strings.Contains(result, "Redacted thinking") {
		t.Errorf("expected '[Redacted thinking]', got: %s", result)
	}
}

func TestShutdownRenderer(t *testing.T) {
	msg := DisplayMessage{
		Type:    "shutdown",
		Content: "Goodbye!",
	}
	result := RenderShutdown(msg, 80)
	if !strings.Contains(result, "Goodbye!") {
		t.Errorf("expected shutdown content, got: %s", result)
	}

	// Test default message
	emptyMsg := DisplayMessage{Type: "shutdown"}
	defaultResult := RenderShutdown(emptyMsg, 80)
	if !strings.Contains(defaultResult, "Session ended") {
		t.Errorf("expected default shutdown message, got: %s", defaultResult)
	}
}

func TestSystemAPIErrorRenderer(t *testing.T) {
	msg := DisplayMessage{
		Type:    "system_api_error",
		Content: "Internal server error",
		Metadata: map[string]string{
			"status_code":   "500",
			"retry_in":      "5s",
			"retry_attempt": "2",
			"max_retries":   "5",
		},
	}
	result := RenderSystemAPIError(msg, 80)
	if !strings.Contains(result, "Internal server error") {
		t.Errorf("expected error message, got: %s", result)
	}
	if !strings.Contains(result, "Retrying") {
		t.Errorf("expected retry info, got: %s", result)
	}
}

func TestToolRejectedRenderer(t *testing.T) {
	msg := DisplayMessage{
		Type:     "tool_rejected",
		ToolName: "BashTool",
		Content:  "Permission denied by user",
	}
	result := RenderToolRejected(msg, 80)
	if !strings.Contains(result, "rejected") {
		t.Errorf("expected 'rejected', got: %s", result)
	}
	if !strings.Contains(result, "BashTool") {
		t.Errorf("expected tool name, got: %s", result)
	}
}

func TestAllRegisteredTypesRender(t *testing.T) {
	r := NewRegistry()

	// Test that every registered type produces non-empty output
	types := []string{
		"assistant_text", "assistant_thinking", "assistant_redacted_thinking",
		"assistant_tool_use",
		"user_text", "user_command", "user_bash_input", "user_bash_output",
		"user_image", "user_plan", "user_prompt", "user_memory_input",
		"user_agent_notification", "user_teammate", "user_channel",
		"user_local_command_output", "user_resource_update", "attachment",
		"tool_result", "tool_success", "tool_error", "tool_rejected",
		"tool_canceled", "plan_approval", "rejected_plan",
		"grouped_tool_use", "collapsed_read_search",
		"system_text", "system_api_error", "rate_limit", "compact_boundary",
		"shutdown", "hook_progress", "task_assignment", "advisor",
	}

	for _, msgType := range types {
		msg := DisplayMessage{
			Type:     msgType,
			Role:     "test",
			Content:  "test content",
			ToolName: "TestTool",
			Metadata: map[string]string{
				"agent_name": "TestAgent",
				"status":     "running",
			},
		}
		result := r.Render(msg, 80)
		if result == "" {
			t.Errorf("renderer for type %q produced empty output", msgType)
		}
	}
}
