package renderers

import (
	"fmt"
	"strings"
)

// RenderAPIError renders an API error with a red bordered box, status code,
// error message, and retry/suggestion text. Matches Claude Code's
// SystemAPIErrorMessage.tsx which shows formatted errors with retry info
// and suggestion text based on error type.
func RenderAPIError(msg DisplayMessage, width int) string {
	var sb strings.Builder

	statusCode := msg.Metadata["status_code"]
	retryIn := msg.Metadata["retry_in"]
	attempt := msg.Metadata["retry_attempt"]
	maxRetries := msg.Metadata["max_retries"]
	suggestion := msg.Metadata["suggestion"]
	errorType := msg.Metadata["error_type"]

	// Error icon and header
	header := "\u26D4 API Error"
	if statusCode != "" {
		header += fmt.Sprintf(" [%s]", statusCode)
	}
	if errorType != "" {
		header += fmt.Sprintf(" (%s)", errorType)
	}
	sb.WriteString(errorStyle.Render(header))
	sb.WriteString("\n")

	// Error message
	if msg.Content != "" {
		// Truncate very long error messages
		content := msg.Content
		maxLen := 1000
		if len(content) > maxLen {
			content = content[:maxLen] + "\u2026"
		}
		sb.WriteString(paddingStyle.Render(errorStyle.Render(content)))
	}

	// Retry information
	if retryIn != "" {
		sb.WriteString("\n")
		retryMsg := fmt.Sprintf("Retrying in %s...", retryIn)
		if attempt != "" && maxRetries != "" {
			retryMsg = fmt.Sprintf("Retrying in %s (attempt %s/%s)...", retryIn, attempt, maxRetries)
		}
		sb.WriteString(paddingStyle.Render(dimStyle.Render(retryMsg)))
	}

	// Suggestion text based on error type
	if suggestion != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(suggestion)))
	} else {
		// Default suggestions based on status code
		switch statusCode {
		case "401", "403":
			sb.WriteString("\n")
			sb.WriteString(paddingStyle.Render(dimStyle.Render("Please check your API key or run /login")))
		case "429":
			sb.WriteString("\n")
			sb.WriteString(paddingStyle.Render(dimStyle.Render("Rate limited. Will retry automatically.")))
		case "529":
			sb.WriteString("\n")
			sb.WriteString(paddingStyle.Render(dimStyle.Render("API overloaded. Will retry with backoff.")))
		}
	}

	return sb.String()
}

// RenderToolExecutionError renders a tool execution error with the tool name,
// error message, and optional collapsed stack trace. Matches Claude Code's
// FallbackToolUseErrorMessage.tsx behavior.
func RenderToolExecutionError(msg DisplayMessage, width int) string {
	var sb strings.Builder

	toolName := msg.ToolName
	if toolName == "" {
		toolName = msg.Metadata["tool_name"]
	}
	if toolName == "" {
		toolName = "Tool"
	}

	// Tool name + error indicator
	sb.WriteString(errorStyle.Render(fmt.Sprintf("\u2717 %s", toolName)))
	sb.WriteString("\n")

	// Error message
	if msg.Content != "" {
		content := msg.Content
		// Show max 10 lines, matching TS MAX_RENDERED_LINES = 10
		lines := strings.Split(content, "\n")
		maxLines := 10
		if len(lines) > maxLines {
			truncated := strings.Join(lines[:maxLines], "\n")
			remaining := len(lines) - maxLines
			sb.WriteString(paddingStyle.Render(errorStyle.Render(truncated)))
			sb.WriteString("\n")
			sb.WriteString(paddingStyle.Render(dimStyle.Render(
				fmt.Sprintf("... %d more lines (Ctrl+O to expand)", remaining),
			)))
		} else {
			sb.WriteString(paddingStyle.Render(errorStyle.Render(content)))
		}
	}

	// Stack trace (collapsed)
	stackTrace := msg.Metadata["stack_trace"]
	if stackTrace != "" && !msg.IsCollapsed {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render("Stack trace:")))
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(stackTrace)))
	} else if stackTrace != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render("[Stack trace hidden - Ctrl+O to expand]")))
	}

	return sb.String()
}

// RenderAuthError renders authentication-specific error messages with
// clear guidance on how to resolve. Shows "API key invalid", "Token expired",
// "OAuth refresh needed" etc. with links to documentation or suggested
// commands (/login).
func RenderAuthError(msg DisplayMessage, _ int) string {
	var sb strings.Builder

	authType := msg.Metadata["auth_type"]
	header := "\U0001F512 Authentication Error"
	if authType != "" {
		header = fmt.Sprintf("\U0001F512 %s", authType)
	}
	sb.WriteString(errorStyle.Render(header))
	sb.WriteString("\n")

	// Error message
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(errorStyle.Render(msg.Content)))
	}

	// Recovery suggestions based on auth error type
	switch authType {
	case "API key invalid", "invalid_api_key":
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(
			"Set a valid API key with ANTHROPIC_API_KEY or run /login",
		)))
	case "Token expired", "token_expired":
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(
			"Your session has expired. Run /login to reauthenticate.",
		)))
	case "OAuth refresh needed", "oauth_refresh":
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(
			"OAuth token needs refresh. Run /login to reauthorize.",
		)))
	default:
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(
			"Run /login to authenticate or check your API key.",
		)))
	}

	return sb.String()
}

// RenderNetworkError renders network-related errors with connection details
// and retry information. Handles connection refused, timeout, DNS failure.
func RenderNetworkError(msg DisplayMessage, _ int) string {
	var sb strings.Builder

	networkType := msg.Metadata["network_error_type"]
	header := "\U0001F310 Network Error"
	switch networkType {
	case "connection_refused":
		header = "\U0001F310 Connection Refused"
	case "timeout":
		header = "\U0001F310 Connection Timeout"
	case "dns_failure":
		header = "\U0001F310 DNS Resolution Failed"
	}
	sb.WriteString(errorStyle.Render(header))
	sb.WriteString("\n")

	// Error message
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(errorStyle.Render(msg.Content)))
	}

	// Retry information
	retryIn := msg.Metadata["retry_in"]
	if retryIn != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(
			fmt.Sprintf("Retrying in %s...", retryIn),
		)))
	}

	// Suggestion
	suggestion := msg.Metadata["suggestion"]
	if suggestion != "" {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(suggestion)))
	} else {
		sb.WriteString("\n")
		sb.WriteString(paddingStyle.Render(dimStyle.Render(
			"Check your network connection and proxy settings.",
		)))
	}

	return sb.String()
}

// RenderValidationError renders input validation failures with field-level
// detail. Shows which fields failed validation and why.
func RenderValidationError(msg DisplayMessage, _ int) string {
	var sb strings.Builder

	sb.WriteString(warningStyle.Render("\u26A0 Validation Error"))
	sb.WriteString("\n")

	// Error message
	if msg.Content != "" {
		sb.WriteString(paddingStyle.Render(warningStyle.Render(msg.Content)))
	}

	// Field-level details from metadata
	fields := msg.Metadata["fields"]
	if fields != "" {
		sb.WriteString("\n")
		// Fields are passed as "field1:error1|field2:error2" format
		for _, field := range strings.Split(fields, "|") {
			parts := strings.SplitN(field, ":", 2)
			if len(parts) == 2 {
				sb.WriteString(paddingStyle.Render(fmt.Sprintf(
					"  %s %s: %s",
					warningStyle.Render("\u2022"),
					parts[0],
					dimStyle.Render(parts[1]),
				)))
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
