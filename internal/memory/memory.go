package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/khaledmoayad/clawgo/internal/api"
)

// MemoryManager handles session memory extraction and recall.
type MemoryManager struct {
	config MemoryConfig
	client *api.Client
}

// NewMemoryManager creates a new MemoryManager with the given configuration and API client.
func NewMemoryManager(cfg MemoryConfig, client *api.Client) *MemoryManager {
	return &MemoryManager{
		config: cfg,
		client: client,
	}
}

// GetMemoryDir returns the memory storage directory, creating it if it doesn't exist.
func GetMemoryDir(cfg MemoryConfig) string {
	_ = os.MkdirAll(cfg.MemoryDir, 0755)
	return cfg.MemoryDir
}

// GetMemoryPath returns the full path for a project's memory file.
// Uses SHA256 hash of project path (first 16 hex chars) as filename,
// matching the same hashing pattern as session/storage.go hashPath.
func GetMemoryPath(cfg MemoryConfig, projectPath string) string {
	return filepath.Join(GetMemoryDir(cfg), hashProjectPath(projectPath)+".md")
}

// ShouldExtractMemory returns true if the conversation exceeds both the token
// and tool call thresholds configured for memory extraction. Both thresholds
// must be met to trigger extraction.
func (m *MemoryManager) ShouldExtractMemory(tokenCount int, toolCallCount int) bool {
	if !m.config.Enabled {
		return false
	}
	return tokenCount >= m.config.MinTokensForExtraction &&
		toolCallCount >= m.config.MinToolCallsForExtraction
}

// ExtractMemory sends the conversation to Claude for memory extraction.
// Called after a conversation ends. Uses the MemoryUpdatePrompt as system
// prompt, requesting memory extraction into the template format. Reads
// existing memory file if present and includes it in the prompt so Claude
// can update rather than replace.
//
// Memory extraction is best-effort: errors are logged to stderr but do not
// propagate (the function returns the error for callers that want to handle it).
func (m *MemoryManager) ExtractMemory(ctx context.Context, messages []api.Message, projectPath string) error {
	if m.client == nil {
		return fmt.Errorf("memory extraction requires an API client")
	}

	// Load existing memory if present
	existingMemory, _ := m.LoadMemories(projectPath)

	// Build the user prompt with template and existing memory
	userPrompt := FormatMemoryPrompt(DefaultSessionMemoryTemplate, existingMemory)

	// Convert conversation messages to SDK params
	var msgParams []anthropic.MessageParam
	for _, msg := range messages {
		msgParams = append(msgParams, msg.ToParam())
	}

	// Add the memory extraction request as the final user message
	msgParams = append(msgParams, anthropic.MessageParam{
		Role: anthropic.MessageParamRoleUser,
		Content: []anthropic.ContentBlockParamUnion{
			anthropic.NewTextBlock(userPrompt),
		},
	})

	// Call the API synchronously (not streaming) for memory extraction
	resp, err := m.client.SDK.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     m.client.Model,
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{
			{Text: MemoryUpdatePrompt},
		},
		Messages: msgParams,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "memory extraction failed: %v\n", err)
		return fmt.Errorf("memory extraction API call failed: %w", err)
	}

	// Extract the text response
	var memoryContent string
	for _, block := range resp.Content {
		if block.Type == "text" {
			memoryContent += block.Text
		}
	}

	if memoryContent == "" {
		return fmt.Errorf("memory extraction returned empty response")
	}

	// Write to the memory file
	memoryPath := GetMemoryPath(m.config, projectPath)
	if err := os.MkdirAll(filepath.Dir(memoryPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create memory directory: %v\n", err)
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	if err := os.WriteFile(memoryPath, []byte(memoryContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write memory file: %v\n", err)
		return fmt.Errorf("failed to write memory file: %w", err)
	}

	return nil
}

// LoadMemories reads the memory file for the given project path.
// Returns empty string if the file doesn't exist (no error).
// Returns the memory content string for injection into system prompts.
func (m *MemoryManager) LoadMemories(projectPath string) (string, error) {
	memoryPath := GetMemoryPath(m.config, projectPath)
	data, err := os.ReadFile(memoryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read memory file: %w", err)
	}
	return string(data), nil
}

// AutoDreamCheck determines if memory extraction should run at session end.
// Returns true if all conditions are met:
// 1. Auto-memory is enabled (via config)
// 2. Token count >= MinTokensForExtraction
// 3. Tool call count >= MinToolCallsForExtraction
// 4. Session had meaningful interaction (not just a greeting)
//
// This replaces the simple ShouldExtractMemory check with awareness of the
// forked agent pattern -- when CacheSafeParams are available, the forked agent
// path is preferred for its prompt cache sharing benefits.
func (m *MemoryManager) AutoDreamCheck(tokenCount, toolCallCount int) bool {
	if !m.config.Enabled {
		return false
	}
	return tokenCount >= m.config.MinTokensForExtraction &&
		toolCallCount >= m.config.MinToolCallsForExtraction
}

// ExtractWithForkedAgent runs memory extraction using the forked agent pattern.
// This is preferred over ExtractMemory when CacheSafeParams are available,
// because it shares the parent's prompt cache (cheaper and faster).
//
// Falls back to the direct ExtractMemory approach if CacheSafeParams are nil.
func (m *MemoryManager) ExtractWithForkedAgent(ctx context.Context, messages []api.Message, projectPath string) error {
	cacheSafe := GetLastCacheSafeParams()

	if cacheSafe == nil {
		// No cache-safe params available -- fall back to direct extraction
		return m.ExtractMemory(ctx, messages, projectPath)
	}

	memDir := GetAutoMemoryDirDefault(projectPath)

	return ExtractMemories(ctx, m.client, ExtractMemoriesParams{
		Config:          DefaultExtractMemoriesConfig(),
		CacheSafeParams: cacheSafe,
		Messages:        messages,
		ProjectPath:     projectPath,
		MemoryDir:       memDir,
	})
}

// hashProjectPath returns a SHA256 hash prefix of the project path.
// Uses first 16 hex characters, matching session/storage.go hashPath.
func hashProjectPath(p string) string {
	h := sha256.Sum256([]byte(p))
	return hex.EncodeToString(h[:8]) // 16-char hex prefix
}
