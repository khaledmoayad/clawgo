// Package api provides the Anthropic API client layer for ClawGo.
//
// classify.go implements comprehensive API error classification, converting raw
// API errors into structured error info with categories, user-facing messages,
// and error details. This mirrors Claude Code's services/api/errors.ts taxonomy.
package api

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// --- Error message constants (matching Claude Code) ---

const (
	APIErrorMessagePrefix          = "API Error"
	PromptTooLongErrorMessage      = "Prompt is too long"
	CreditBalanceTooLowMessage     = "Credit balance is too low"
	InvalidAPIKeyMessage           = "Not logged in \u00b7 Please run /login"
	InvalidAPIKeyExternalMessage   = "Invalid API key \u00b7 Fix external API key"
	OrgDisabledEnvKeyWithOAuth     = "Your ANTHROPIC_API_KEY belongs to a disabled organization \u00b7 Unset the environment variable to use your subscription instead"
	OrgDisabledEnvKey              = "Your ANTHROPIC_API_KEY belongs to a disabled organization \u00b7 Update or unset the environment variable"
	TokenRevokedMessage            = "OAuth token revoked \u00b7 Please run /login"
	CCRAuthErrorMessage            = "Authentication error \u00b7 This may be a temporary network issue, please try again"
	Repeated529ErrorMessage        = "Repeated 529 Overloaded errors"
	CustomOffSwitchMessage         = "Opus is experiencing high load, please use /model to switch to Sonnet"
	APITimeoutErrorMessage         = "Request timed out"
	OAuthOrgNotAllowedMessage      = "Your account does not have access to Claude Code. Please run /login."
	RefusalMessage                 = "Claude Code is unable to respond to this request, which appears to violate our Usage Policy (https://www.anthropic.com/legal/aup)."
)

// APIErrorType is a fine-grained classification string for analytics/diagnostics.
// Matches Claude Code's classifyAPIError() return values.
type APIErrorType string

const (
	ErrTypeAborted             APIErrorType = "aborted"
	ErrTypeAPITimeout          APIErrorType = "api_timeout"
	ErrTypeRepeated529         APIErrorType = "repeated_529"
	ErrTypeCapacityOffSwitch   APIErrorType = "capacity_off_switch"
	ErrTypeRateLimit           APIErrorType = "rate_limit"
	ErrTypeServerOverload      APIErrorType = "server_overload"
	ErrTypePromptTooLong       APIErrorType = "prompt_too_long"
	ErrTypePDFTooLarge         APIErrorType = "pdf_too_large"
	ErrTypePDFPasswordProtected APIErrorType = "pdf_password_protected"
	ErrTypeImageTooLarge       APIErrorType = "image_too_large"
	ErrTypeToolUseMismatch     APIErrorType = "tool_use_mismatch"
	ErrTypeUnexpectedToolResult APIErrorType = "unexpected_tool_result"
	ErrTypeDuplicateToolUseID  APIErrorType = "duplicate_tool_use_id"
	ErrTypeInvalidModel        APIErrorType = "invalid_model"
	ErrTypeCreditBalanceLow    APIErrorType = "credit_balance_low"
	ErrTypeInvalidAPIKey       APIErrorType = "invalid_api_key"
	ErrTypeTokenRevoked        APIErrorType = "token_revoked"
	ErrTypeOAuthOrgNotAllowed  APIErrorType = "oauth_org_not_allowed"
	ErrTypeAuthError           APIErrorType = "auth_error"
	ErrTypeBedrockModelAccess  APIErrorType = "bedrock_model_access"
	ErrTypeSSLCertError        APIErrorType = "ssl_cert_error"
	ErrTypeConnectionError     APIErrorType = "connection_error"
	ErrTypeServerError         APIErrorType = "server_error"
	ErrTypeClientError         APIErrorType = "client_error"
	ErrTypeRefusal             APIErrorType = "refusal"
	ErrTypeUnknown             APIErrorType = "unknown"
)

// APIErrorInfo holds classified error information suitable for display and recovery.
// This is the structured form produced by ClassifyAPIError() and consumed by the
// query loop and TUI for user-facing error messages.
type APIErrorInfo struct {
	// Type is the fine-grained error type for analytics.
	Type APIErrorType

	// Category is the coarse error category (rate_limit, auth, etc.)
	Category ErrorCategory

	// UserMessage is the user-facing error message.
	UserMessage string

	// ErrorDetails contains the raw error message for programmatic parsing
	// (e.g., token counts from prompt-too-long errors).
	ErrorDetails string

	// IsRecoverable indicates whether the query loop should attempt recovery
	// (e.g., retry after backoff, reactive compaction, model fallback).
	IsRecoverable bool

	// StatusCode is the HTTP status code if available (0 if not an HTTP error).
	StatusCode int
}

// promptTooLongRegex extracts actual/limit token counts from prompt-too-long errors.
// Matches patterns like "prompt is too long: 137500 tokens > 135000 maximum".
var promptTooLongRegex = regexp.MustCompile(`(?i)prompt is too long[^0-9]*(\d+)\s*tokens?\s*>\s*(\d+)`)

// pdfPageLimitRegex matches PDF page limit errors.
var pdfPageLimitRegex = regexp.MustCompile(`maximum of \d+ PDF pages`)

// ParsePromptTooLongTokenCounts extracts actual and limit token counts from a
// prompt-too-long error message. Returns (0, 0) if the message doesn't match.
func ParsePromptTooLongTokenCounts(rawMessage string) (actual int, limit int) {
	matches := promptTooLongRegex.FindStringSubmatch(rawMessage)
	if len(matches) < 3 {
		return 0, 0
	}
	fmt.Sscanf(matches[1], "%d", &actual)
	fmt.Sscanf(matches[2], "%d", &limit)
	return actual, limit
}

// GetPromptTooLongTokenGap returns how many tokens over the limit a prompt-too-long
// error reports, or 0 if the message can't be parsed. Used by reactive compact to
// calculate how aggressively to compact.
func GetPromptTooLongTokenGap(rawMessage string) int {
	actual, limit := ParsePromptTooLongTokenCounts(rawMessage)
	if actual == 0 || limit == 0 {
		return 0
	}
	gap := actual - limit
	if gap > 0 {
		return gap
	}
	return 0
}

// IsMediaSizeError checks if an error message indicates a media (image/PDF) size
// rejection that can be recovered by stripping media from messages.
func IsMediaSizeError(raw string) bool {
	if strings.Contains(raw, "image exceeds") && strings.Contains(raw, "maximum") {
		return true
	}
	if strings.Contains(raw, "image dimensions exceed") && strings.Contains(raw, "many-image") {
		return true
	}
	if pdfPageLimitRegex.MatchString(raw) {
		return true
	}
	return false
}

// IsPromptTooLongError checks if an error message indicates the prompt is too long.
func IsPromptTooLongError(raw string) bool {
	return strings.Contains(strings.ToLower(raw), "prompt is too long")
}

// ClassifyAPIError converts a raw API error into a structured APIErrorInfo with
// fine-grained classification, user-facing message, and recovery information.
// This mirrors Claude Code's getAssistantMessageFromError() + classifyAPIError().
func ClassifyAPIError(err error, model string, nonInteractive bool) *APIErrorInfo {
	if err == nil {
		return nil
	}

	msg := err.Error()
	info := &APIErrorInfo{
		ErrorDetails: msg,
	}

	// Extract HTTP status code from HTTPError or SDK APIError
	var httpErr *HTTPError
	hasHTTPErr := errors.As(err, &httpErr)
	if hasHTTPErr {
		info.StatusCode = httpErr.StatusCode
	}

	statusCode := 0
	if hasHTTPErr {
		statusCode = httpErr.StatusCode
	}

	// Also check the Anthropic SDK error type
	var sdkErr interface{ Error() string }
	if errors.As(err, &sdkErr) {
		// The SDK's Error type embeds status code; we already handle it
		// via the CategorizeError path. Here we just extract the status code.
	}

	// Check abort errors
	if strings.Contains(msg, "Request was aborted") || errors.Is(err, errAborted) {
		info.Type = ErrTypeAborted
		info.Category = ErrUnknown
		info.UserMessage = "Request was aborted"
		info.IsRecoverable = false
		return info
	}

	// Timeout errors
	if strings.Contains(strings.ToLower(msg), "timeout") {
		info.Type = ErrTypeAPITimeout
		info.Category = ErrNetwork
		info.UserMessage = APITimeoutErrorMessage
		info.IsRecoverable = true
		return info
	}

	// Repeated 529 errors
	if strings.Contains(msg, Repeated529ErrorMessage) {
		info.Type = ErrTypeRepeated529
		info.Category = ErrOverloaded
		info.UserMessage = Repeated529ErrorMessage
		info.IsRecoverable = false
		return info
	}

	// Emergency capacity off switch
	if strings.Contains(msg, CustomOffSwitchMessage) {
		info.Type = ErrTypeCapacityOffSwitch
		info.Category = ErrRateLimit
		info.UserMessage = CustomOffSwitchMessage
		info.IsRecoverable = false
		return info
	}

	// Rate limit (429)
	if statusCode == 429 {
		info.Type = ErrTypeRateLimit
		info.Category = ErrRateLimit
		info.IsRecoverable = true

		// Check for extra usage required for 1M context
		if strings.Contains(msg, "Extra usage is required for long context") {
			hint := "--model to switch to standard context"
			if !nonInteractive {
				hint = "/extra-usage to enable, or /model to switch to standard context"
			}
			info.UserMessage = fmt.Sprintf("%s: Extra usage is required for 1M context \u00b7 %s", APIErrorMessagePrefix, hint)
			return info
		}

		// Generic rate limit message (will be overridden by quota info if available)
		detail := msg
		if strings.HasPrefix(detail, "429 ") {
			detail = strings.TrimPrefix(detail, "429 ")
		}
		info.UserMessage = fmt.Sprintf("%s: Request rejected (429) \u00b7 %s", APIErrorMessagePrefix, detail)
		return info
	}

	// Server overload (529)
	if statusCode == 529 || strings.Contains(msg, `"type":"overloaded_error"`) {
		info.Type = ErrTypeServerOverload
		info.Category = ErrOverloaded
		info.UserMessage = fmt.Sprintf("%s: Server is overloaded (529)", APIErrorMessagePrefix)
		info.IsRecoverable = true
		return info
	}

	// Prompt too long
	if strings.Contains(strings.ToLower(msg), "prompt is too long") {
		info.Type = ErrTypePromptTooLong
		info.Category = ErrClientError
		info.UserMessage = PromptTooLongErrorMessage
		info.IsRecoverable = true // reactive compaction can recover
		return info
	}

	// PDF page limit
	if pdfPageLimitRegex.MatchString(msg) {
		info.Type = ErrTypePDFTooLarge
		info.Category = ErrClientError
		if nonInteractive {
			info.UserMessage = "PDF too large. Try reading the file a different way (e.g., extract text with pdftotext)."
		} else {
			info.UserMessage = "PDF too large. Double press esc to go back and try again, or use pdftotext to convert to text first."
		}
		return info
	}

	// PDF password protected
	if strings.Contains(msg, "The PDF specified is password protected") {
		info.Type = ErrTypePDFPasswordProtected
		info.Category = ErrClientError
		if nonInteractive {
			info.UserMessage = "PDF is password protected. Try using a CLI tool to extract or convert the PDF."
		} else {
			info.UserMessage = "PDF is password protected. Please double press esc to edit your message and try again."
		}
		return info
	}

	// PDF invalid
	if strings.Contains(msg, "The PDF specified was not valid") {
		info.Type = ErrTypeClientError
		info.Category = ErrClientError
		if nonInteractive {
			info.UserMessage = "The PDF file was not valid. Try converting it to text first (e.g., pdftotext)."
		} else {
			info.UserMessage = "The PDF file was not valid. Double press esc to go back and try again with a different file."
		}
		return info
	}

	// Image too large
	if statusCode == 400 && strings.Contains(msg, "image exceeds") && strings.Contains(msg, "maximum") {
		info.Type = ErrTypeImageTooLarge
		info.Category = ErrClientError
		if nonInteractive {
			info.UserMessage = "Image was too large. Try resizing the image or using a different approach."
		} else {
			info.UserMessage = "Image was too large. Double press esc to go back and try again with a smaller image."
		}
		return info
	}

	// Many-image dimension errors
	if statusCode == 400 && strings.Contains(msg, "image dimensions exceed") && strings.Contains(msg, "many-image") {
		info.Type = ErrTypeImageTooLarge
		info.Category = ErrClientError
		if nonInteractive {
			info.UserMessage = "An image in the conversation exceeds the dimension limit for many-image requests (2000px). Start a new session with fewer images."
		} else {
			info.UserMessage = "An image in the conversation exceeds the dimension limit for many-image requests (2000px). Run /compact to remove old images from context, or start a new session."
		}
		return info
	}

	// Request too large (413)
	if statusCode == 413 {
		info.Type = ErrTypeClientError
		info.Category = ErrClientError
		if nonInteractive {
			info.UserMessage = "Request too large. Try with a smaller file."
		} else {
			info.UserMessage = "Request too large. Double press esc to go back and try with a smaller file."
		}
		return info
	}

	// Tool use/result mismatch
	if statusCode == 400 && strings.Contains(msg, "`tool_use` ids were found without `tool_result` blocks immediately after") {
		info.Type = ErrTypeToolUseMismatch
		info.Category = ErrClientError
		if nonInteractive {
			info.UserMessage = "API Error: 400 due to tool use concurrency issues."
		} else {
			info.UserMessage = "API Error: 400 due to tool use concurrency issues. Run /rewind to recover the conversation."
		}
		return info
	}

	// Unexpected tool_result
	if statusCode == 400 && strings.Contains(msg, "unexpected `tool_use_id` found in `tool_result`") {
		info.Type = ErrTypeUnexpectedToolResult
		info.Category = ErrClientError
		info.UserMessage = fmt.Sprintf("%s: Unexpected tool_result in conversation", APIErrorMessagePrefix)
		return info
	}

	// Duplicate tool_use IDs
	if statusCode == 400 && strings.Contains(msg, "`tool_use` ids must be unique") {
		info.Type = ErrTypeDuplicateToolUseID
		info.Category = ErrClientError
		rewind := ""
		if !nonInteractive {
			rewind = " Run /rewind to recover the conversation."
		}
		info.UserMessage = fmt.Sprintf("API Error: 400 duplicate tool_use ID in conversation history.%s", rewind)
		return info
	}

	// Invalid model name
	if statusCode == 400 && strings.Contains(strings.ToLower(msg), "invalid model name") {
		info.Type = ErrTypeInvalidModel
		info.Category = ErrClientError
		switchCmd := "/model"
		if nonInteractive {
			switchCmd = "--model"
		}
		info.UserMessage = fmt.Sprintf("The model %s is not available. Run %s to pick a different model.", model, switchCmd)
		return info
	}

	// Credit balance too low
	if strings.Contains(strings.ToLower(msg), "credit balance is too low") {
		info.Type = ErrTypeCreditBalanceLow
		info.Category = ErrClientError
		info.UserMessage = CreditBalanceTooLowMessage
		return info
	}

	// Organization disabled
	if statusCode == 400 && strings.Contains(strings.ToLower(msg), "organization has been disabled") {
		info.Type = ErrTypeAuthError
		info.Category = ErrAuth
		info.UserMessage = OrgDisabledEnvKey
		return info
	}

	// Invalid API key
	if strings.Contains(strings.ToLower(msg), "x-api-key") {
		info.Type = ErrTypeInvalidAPIKey
		info.Category = ErrAuth
		info.UserMessage = InvalidAPIKeyMessage
		return info
	}

	// OAuth token revoked
	if statusCode == 403 && strings.Contains(msg, "OAuth token has been revoked") {
		info.Type = ErrTypeTokenRevoked
		info.Category = ErrAuth
		if nonInteractive {
			info.UserMessage = "Your account does not have access to Claude. Please login again or contact your administrator."
		} else {
			info.UserMessage = TokenRevokedMessage
		}
		return info
	}

	// OAuth org not allowed
	if (statusCode == 401 || statusCode == 403) && strings.Contains(msg, "OAuth authentication is currently not allowed for this organization") {
		info.Type = ErrTypeOAuthOrgNotAllowed
		info.Category = ErrAuth
		if nonInteractive {
			info.UserMessage = "Your organization does not have access to Claude. Please login again or contact your administrator."
		} else {
			info.UserMessage = OAuthOrgNotAllowedMessage
		}
		return info
	}

	// Generic auth errors (401/403)
	if statusCode == 401 || statusCode == 403 {
		info.Type = ErrTypeAuthError
		info.Category = ErrAuth
		if nonInteractive {
			info.UserMessage = fmt.Sprintf("Failed to authenticate. %s: %s", APIErrorMessagePrefix, msg)
		} else {
			info.UserMessage = fmt.Sprintf("Please run /login \u00b7 %s: %s", APIErrorMessagePrefix, msg)
		}
		return info
	}

	// Model not found (404)
	if statusCode == 404 {
		info.Type = ErrTypeInvalidModel
		info.Category = ErrClientError
		switchCmd := "/model"
		if nonInteractive {
			switchCmd = "--model"
		}
		info.UserMessage = fmt.Sprintf("There's an issue with the selected model (%s). It may not exist or you may not have access to it. Run %s to pick a different model.", model, switchCmd)
		return info
	}

	// Status code based fallbacks
	if statusCode >= 500 {
		info.Type = ErrTypeServerError
		info.Category = ErrServerError
		info.UserMessage = fmt.Sprintf("%s: Server error (%d)", APIErrorMessagePrefix, statusCode)
		info.IsRecoverable = true
		return info
	}
	if statusCode >= 400 {
		info.Type = ErrTypeClientError
		info.Category = ErrClientError
		info.UserMessage = fmt.Sprintf("%s: %s", APIErrorMessagePrefix, msg)
		return info
	}

	// Network/connection errors
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) {
		if strings.Contains(strings.ToLower(msg), "ssl") || strings.Contains(strings.ToLower(msg), "tls") || strings.Contains(strings.ToLower(msg), "certificate") {
			info.Type = ErrTypeSSLCertError
		} else {
			info.Type = ErrTypeConnectionError
		}
		info.Category = ErrNetwork
		info.UserMessage = fmt.Sprintf("%s: %s", APIErrorMessagePrefix, msg)
		info.IsRecoverable = true
		return info
	}

	// Generic fallback
	info.Type = ErrTypeUnknown
	info.Category = ErrUnknown
	info.UserMessage = fmt.Sprintf("%s: %s", APIErrorMessagePrefix, msg)
	return info
}

// GetRefusalMessage returns an error message for API refusal stop reasons.
// Returns nil if the stop reason is not "refusal".
func GetRefusalMessage(stopReason string, model string, nonInteractive bool) *APIErrorInfo {
	if stopReason != "refusal" {
		return nil
	}
	userMsg := RefusalMessage
	if nonInteractive {
		userMsg += " Try rephrasing the request or attempting a different approach."
	} else {
		userMsg += " Please double press esc to edit your last message or start a new session."
	}
	return &APIErrorInfo{
		Type:        ErrTypeRefusal,
		Category:    ErrClientError,
		UserMessage: fmt.Sprintf("%s: %s", APIErrorMessagePrefix, userMsg),
	}
}

// sentinel error for abort detection
var errAborted = errors.New("request aborted")
