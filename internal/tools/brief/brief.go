// Package brief implements the SendUserMessage tool (formerly BriefTool).
// This is Claude's primary visible output channel for communicating with the user.
// It handles messages with optional file attachments and status classification.
package brief

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/tools"
)

// BriefTool is the SendUserMessage tool -- Claude's primary output channel.
type BriefTool struct{}

// New creates a new BriefTool.
func New() *BriefTool { return &BriefTool{} }

func (t *BriefTool) Name() string                { return "SendUserMessage" }
func (t *BriefTool) Description() string          { return toolDescription }
func (t *BriefTool) IsReadOnly() bool             { return true }
func (t *BriefTool) InputSchema() json.RawMessage { return json.RawMessage(inputSchemaJSON) }

// IsConcurrencySafe returns true -- sending messages is a safe operation.
func (t *BriefTool) IsConcurrencySafe(_ json.RawMessage) bool { return true }

// CheckPermissions returns Allow -- sending messages is always permitted.
func (t *BriefTool) CheckPermissions(_ context.Context, _ json.RawMessage, _ *permissions.PermissionContext) (permissions.PermissionResult, error) {
	return permissions.Allow, nil
}

func (t *BriefTool) Call(_ context.Context, inp json.RawMessage, _ *tools.ToolUseContext) (*tools.ToolResult, error) {
	data, err := tools.ParseRawInput(inp)
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}

	message, err := tools.RequireString(data, "message")
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if strings.TrimSpace(message) == "" {
		return tools.ErrorResult("required field \"message\" is missing or empty"), nil
	}

	status, err := tools.RequireString(data, "status")
	if err != nil {
		return tools.ErrorResult(err.Error()), nil
	}
	if status != "normal" && status != "proactive" {
		return tools.ErrorResult(fmt.Sprintf("invalid status %q: must be \"normal\" or \"proactive\"", status)), nil
	}

	// Parse optional attachments
	var attachments []string
	if rawAttachments, ok := data["attachments"]; ok {
		attachmentList, ok := rawAttachments.([]interface{})
		if !ok {
			return tools.ErrorResult("\"attachments\" must be an array of strings"), nil
		}
		for i, item := range attachmentList {
			s, ok := item.(string)
			if !ok {
				return tools.ErrorResult(fmt.Sprintf("attachments[%d]: expected string, got %T", i, item)), nil
			}
			attachments = append(attachments, s)
		}
	}

	sentAt := time.Now().UTC().Format(time.RFC3339)

	suffix := ""
	if len(attachments) > 0 {
		noun := "attachment"
		if len(attachments) > 1 {
			noun = "attachments"
		}
		suffix = fmt.Sprintf(" (%d %s included)", len(attachments), noun)
	}

	return &tools.ToolResult{
		Content: []tools.ContentBlock{{Type: "text", Text: fmt.Sprintf("Message delivered to user.%s", suffix)}},
		Metadata: map[string]any{
			"message":     message,
			"status":      status,
			"attachments": attachments,
			"sentAt":      sentAt,
		},
	}, nil
}
