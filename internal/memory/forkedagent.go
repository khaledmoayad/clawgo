package memory

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/khaledmoayad/clawgo/internal/api"
)

// DefaultForkedAgentMaxTokens is the default output token cap for forked agents.
// Keeps extraction responses bounded while allowing thorough memory extraction.
const DefaultForkedAgentMaxTokens int64 = 4096

// CacheSafeParams holds parameters that must be identical between the fork and
// parent API requests to share the parent's prompt cache. The Anthropic API
// cache key is composed of: system prompt, tools, model, messages (prefix),
// and thinking config.
type CacheSafeParams struct {
	// SystemPrompt is the system prompt text that must match the parent.
	SystemPrompt string

	// UserContext is user context key/value pairs prepended to messages.
	UserContext map[string]string

	// SystemContext is system context key/value pairs appended to system prompt.
	SystemContext map[string]string

	// Model is the model name to use (must match parent for cache sharing).
	Model string

	// ForkContextMessages are parent conversation messages for prompt cache sharing.
	// These are prepended to the fork's messages to maintain the same prefix.
	ForkContextMessages []api.Message
}

// ForkedAgentParams configures a forked agent execution.
type ForkedAgentParams struct {
	// CacheSafe contains parameters that must match the parent for cache sharing.
	CacheSafe CacheSafeParams

	// UserMessage is the prompt to send to the forked agent after the context messages.
	UserMessage string

	// MaxOutputTokens caps the output tokens for the forked agent.
	// Defaults to DefaultForkedAgentMaxTokens if <= 0.
	MaxOutputTokens int64

	// AbortCtx is the context for cancellation. If nil, context.Background() is used.
	AbortCtx context.Context

	// AgentID is a unique identifier for this fork instance (for logging/tracking).
	AgentID string

	// ForkReason describes why this fork was created (e.g., "memory_extraction",
	// "prompt_suggestion", "consolidation").
	ForkReason string
}

// ForkedAgentResult contains the output from a forked agent execution.
type ForkedAgentResult struct {
	// Response is the text response extracted from the model's reply.
	Response string

	// Usage tracks token consumption for the forked agent call.
	Usage ForkedAgentUsage

	// Messages contains the full message history (context + prompt + response).
	Messages []api.Message
}

// ForkedAgentUsage tracks token consumption for a forked agent call.
type ForkedAgentUsage struct {
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

// RunForkedAgent executes a forked agent query. It builds messages from the
// cache-safe params (parent context messages + user prompt), calls the API
// with a capped output token limit, and returns the text response with usage
// tracking.
//
// The forked agent shares the parent's prompt cache by using identical
// cache-safe parameters (system prompt, model, message prefix). This avoids
// re-processing the conversation history, making forks cheap.
func RunForkedAgent(ctx context.Context, client *api.Client, params ForkedAgentParams) (*ForkedAgentResult, error) {
	if client == nil {
		return nil, fmt.Errorf("forked agent requires an API client")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	maxTokens := params.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = DefaultForkedAgentMaxTokens
	}

	model := params.CacheSafe.Model
	if model == "" {
		model = client.Model
	}

	// Build the messages array: parent context messages + user prompt
	var msgParams []anthropic.MessageParam
	for _, msg := range params.CacheSafe.ForkContextMessages {
		msgParams = append(msgParams, msg.ToParam())
	}

	// Add the user prompt as the final message
	msgParams = append(msgParams, anthropic.MessageParam{
		Role: anthropic.MessageParamRoleUser,
		Content: []anthropic.ContentBlockParamUnion{
			anthropic.NewTextBlock(params.UserMessage),
		},
	})

	// Build system prompt
	systemPrompt := params.CacheSafe.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful assistant."
	}

	// Call the API with capped output tokens
	resp, err := client.SDK.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: maxTokens,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: msgParams,
	})
	if err != nil {
		return nil, fmt.Errorf("forked agent API call failed (%s): %w", params.ForkReason, err)
	}

	// Extract text content from response
	var responseText string
	for _, block := range resp.Content {
		if block.Type == "text" {
			responseText += block.Text
		}
	}

	// Build usage tracking from response
	usage := ForkedAgentUsage{
		InputTokens:              int(resp.Usage.InputTokens),
		OutputTokens:             int(resp.Usage.OutputTokens),
		CacheCreationInputTokens: int(resp.Usage.CacheCreationInputTokens),
		CacheReadInputTokens:     int(resp.Usage.CacheReadInputTokens),
	}

	// Log usage to stderr for debugging (best-effort, non-fatal)
	fmt.Fprintf(os.Stderr, "[forkedAgent:%s] completed: input=%d output=%d cacheRead=%d cacheCreate=%d\n",
		params.ForkReason, usage.InputTokens, usage.OutputTokens,
		usage.CacheReadInputTokens, usage.CacheCreationInputTokens)

	return &ForkedAgentResult{
		Response: responseText,
		Usage:    usage,
		Messages: append(params.CacheSafe.ForkContextMessages, api.Message{
			Role: "assistant",
			Content: []api.ContentBlock{
				{Type: api.ContentText, Text: responseText},
			},
		}),
	}, nil
}

// Module-level cache for sharing cache-safe params between the main loop
// and forked agents. Written after each turn by stop hooks, read by memory
// extraction and other forked agents.
var (
	cacheMu             sync.RWMutex
	lastCacheSafeParams *CacheSafeParams
)

// SaveCacheSafeParams stores the cache-safe params from the latest turn.
// Called by stop hooks after each API response so forked agents can share
// the parent's prompt cache.
func SaveCacheSafeParams(params *CacheSafeParams) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	lastCacheSafeParams = params
}

// GetLastCacheSafeParams returns the most recently saved cache-safe params.
// Returns nil if no params have been saved yet (e.g., before the first turn).
func GetLastCacheSafeParams() *CacheSafeParams {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return lastCacheSafeParams
}

// ClearCacheSafeParams resets the stored cache-safe params.
// Useful for testing and session cleanup.
func ClearCacheSafeParams() {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	lastCacheSafeParams = nil
}
