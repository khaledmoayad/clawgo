package compact

import (
	"context"
	"regexp"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/khaledmoayad/clawgo/internal/api"
)

// CompactionResult holds the outcome of a compaction operation.
type CompactionResult struct {
	Summary      string // Extracted summary text
	TokensSaved  int    // Estimated tokens reclaimed
	WasCompacted bool   // True if compaction actually occurred
}

// CompactParams configures a compaction request.
type CompactParams struct {
	Client             *api.Client
	Model              string
	Messages           []api.Message
	SystemPrompt       string
	CustomInstructions string // Extra instructions appended to compaction prompt
}

// CompactConversation sends the full conversation to Claude for summarization.
// It makes a synchronous (non-streaming) API call with the compaction system
// prompt and returns a CompactionResult with the extracted summary.
func CompactConversation(ctx context.Context, params CompactParams) (*CompactionResult, error) {
	msgParams := BuildCompactionMessages(params.Messages)

	// Build the compaction system prompt
	systemPrompt := BaseCompactPrompt
	if params.CustomInstructions != "" {
		systemPrompt += "\n\nAdditional context:\n" + params.CustomInstructions
	}

	req := anthropic.MessageNewParams{
		Model:     params.Model,
		MaxTokens: int64(MaxOutputTokensForSummary),
		Messages:  msgParams,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
	}

	resp, err := params.Client.SDK.Messages.New(ctx, req)
	if err != nil {
		return nil, err
	}

	// Extract text from response
	var rawSummary strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			rawSummary.WriteString(block.Text)
		}
	}

	summary := FormatCompactSummary(rawSummary.String())

	// Estimate tokens saved: rough estimate based on original message length
	// minus summary length (using chars/4 heuristic)
	originalTokens := estimateMessageTokens(params.Messages)
	summaryTokens := len(summary) / 4
	tokensSaved := originalTokens - summaryTokens
	if tokensSaved < 0 {
		tokensSaved = 0
	}

	return &CompactionResult{
		Summary:      summary,
		TokensSaved:  tokensSaved,
		WasCompacted: true,
	}, nil
}

// analysisRe matches <analysis>...</analysis> blocks (including newlines).
var analysisRe = regexp.MustCompile(`(?s)<analysis>.*?</analysis>`)

// summaryTagRe matches <summary>...</summary> blocks.
var summaryTagRe = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)

// whitespaceRe matches sequences of 3+ newlines for cleanup.
var whitespaceRe = regexp.MustCompile(`\n{3,}`)

// FormatCompactSummary strips <analysis> blocks, extracts content from
// <summary> tags, and cleans extra whitespace. If no <summary> tags are
// found, the full text (minus analysis) is returned.
func FormatCompactSummary(raw string) string {
	// Strip analysis blocks
	result := analysisRe.ReplaceAllString(raw, "")

	// Extract summary content if tags are present
	if matches := summaryTagRe.FindStringSubmatch(result); len(matches) > 1 {
		result = matches[1]
	}

	// Clean excessive whitespace
	result = whitespaceRe.ReplaceAllString(result, "\n\n")
	result = strings.TrimSpace(result)

	return result
}

// BuildCompactionMessages converts conversation history to MessageParam slice
// for the compaction API call. It ensures alternating user/assistant roles as
// required by the API. Adjacent messages with the same role are merged.
func BuildCompactionMessages(messages []api.Message) []anthropic.MessageParam {
	if len(messages) == 0 {
		return nil
	}

	var params []anthropic.MessageParam
	for _, m := range messages {
		param := m.ToParam()

		// Ensure alternating roles: if the last param has the same role,
		// merge content blocks into the existing message
		if len(params) > 0 && params[len(params)-1].Role == param.Role {
			params[len(params)-1].Content = append(params[len(params)-1].Content, param.Content...)
			continue
		}
		params = append(params, param)
	}

	// API requires the first message to be from the user
	if len(params) > 0 && params[0].Role != "user" {
		userMsg := anthropic.MessageParam{
			Role:    "user",
			Content: []anthropic.ContentBlockParamUnion{anthropic.NewTextBlock("Summarize the following conversation:")},
		}
		params = append([]anthropic.MessageParam{userMsg}, params...)
	}

	return params
}

// estimateMessageTokens provides a rough token estimate for a slice of messages.
// Uses the chars/4 heuristic which is sufficient for threshold comparisons.
func estimateMessageTokens(messages []api.Message) int {
	total := 0
	for _, m := range messages {
		for _, cb := range m.Content {
			total += len(cb.Text) + len(cb.Content) + len(cb.Thinking)
			if cb.Input != nil {
				total += len(cb.Input)
			}
		}
	}
	return total / 4
}
