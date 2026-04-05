package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/config"
)

// ExtractMemoriesConfig holds configuration for memory extraction via forked agent.
type ExtractMemoriesConfig struct {
	// Enabled controls whether forked-agent memory extraction is active.
	Enabled bool

	// MinTokensForTrigger is the minimum input tokens to trigger extraction.
	// Conversations below this threshold are too short for meaningful extraction.
	MinTokensForTrigger int

	// MinToolCallsForTrigger is the minimum tool calls to trigger extraction.
	// Conversations with fewer tool calls lack enough interaction context.
	MinToolCallsForTrigger int

	// MaxOutputTokens caps the forked agent's output tokens.
	MaxOutputTokens int64
}

// DefaultExtractMemoriesConfig returns sensible defaults matching Claude Code behavior.
func DefaultExtractMemoriesConfig() ExtractMemoriesConfig {
	return ExtractMemoriesConfig{
		Enabled:                true,
		MinTokensForTrigger:    8000,
		MinToolCallsForTrigger: 3,
		MaxOutputTokens:        DefaultForkedAgentMaxTokens,
	}
}

// ExtractMemoriesParams holds parameters for a single extraction run.
type ExtractMemoriesParams struct {
	// Config controls extraction behavior and thresholds.
	Config ExtractMemoriesConfig

	// CacheSafeParams are the parent's cache-safe params for prompt cache sharing.
	CacheSafeParams *CacheSafeParams

	// Messages is the conversation history to extract memories from.
	Messages []api.Message

	// ProjectPath is the project root path for computing the auto-memory directory.
	ProjectPath string

	// MemoryDir is the auto-memory directory path (~/.claude/projects/<hash>/memory/).
	// If empty, it is computed from ProjectPath.
	MemoryDir string
}

// MemoryFileEntry represents a single memory file in the auto-memory directory.
type MemoryFileEntry struct {
	// Path is the full filesystem path to the memory file.
	Path string

	// Name is the filename without extension.
	Name string

	// Content is the file's text content.
	Content string

	// Size is the file size in bytes.
	Size int64
}

// GetAutoMemoryDir returns the auto-memory directory for a project.
// Path pattern: ~/.claude/projects/<hash>/memory/
// This matches the TypeScript auto-memory directory structure.
func GetAutoMemoryDir(configDir, projectPath string) string {
	hash := hashProjectPath(projectPath)
	return filepath.Join(configDir, "projects", hash, "memory")
}

// GetAutoMemoryDirDefault returns the auto-memory directory using the default config dir.
func GetAutoMemoryDirDefault(projectPath string) string {
	return GetAutoMemoryDir(config.ConfigDir(), projectPath)
}

// ScanMemoryFiles lists all .md memory files in the auto-memory directory.
// Returns an empty slice (not nil) if the directory doesn't exist or has no .md files.
func ScanMemoryFiles(memDir string) ([]MemoryFileEntry, error) {
	entries, err := os.ReadDir(memDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []MemoryFileEntry{}, nil
		}
		return nil, fmt.Errorf("failed to read memory directory %s: %w", memDir, err)
	}

	var files []MemoryFileEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		fullPath := filepath.Join(memDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue // skip files we can't stat
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue // skip files we can't read
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		files = append(files, MemoryFileEntry{
			Path:    fullPath,
			Name:    name,
			Content: string(content),
			Size:    info.Size(),
		})
	}

	if files == nil {
		files = []MemoryFileEntry{}
	}
	return files, nil
}

// WriteMemoryFile writes a memory entry to the auto-memory directory.
// Creates the directory if it doesn't exist.
func WriteMemoryFile(memDir, name, content string) error {
	if err := os.MkdirAll(memDir, 0755); err != nil {
		return fmt.Errorf("failed to create memory directory: %w", err)
	}

	// Ensure name has .md extension
	filename := name
	if filepath.Ext(filename) != ".md" {
		filename = name + ".md"
	}

	path := filepath.Join(memDir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write memory file %s: %w", path, err)
	}
	return nil
}

// formatMemoryManifest creates a formatted manifest of existing memory files
// for inclusion in the extraction prompt. This lets the forked agent know what
// memories already exist so it can update rather than duplicate.
func formatMemoryManifest(files []MemoryFileEntry) string {
	if len(files) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("### %s.md\n", f.Name))
		sb.WriteString(f.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// buildExtractionPrompt constructs the user prompt for memory extraction.
// It includes the existing memory manifest and instructions for the forked agent
// to extract new memories and update existing ones.
func buildExtractionPrompt(existingMemories string, messageCount int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(
		"You are now acting as the memory extraction subagent. Analyze the most recent ~%d messages above and extract durable memories.\n\n",
		messageCount,
	))

	if existingMemories != "" {
		sb.WriteString("## Existing memory files\n\n")
		sb.WriteString(existingMemories)
		sb.WriteString("\nCheck this list before writing -- update an existing file rather than creating a duplicate.\n\n")
	}

	sb.WriteString(`## Instructions

1. Extract important information from the conversation that would help continue this work in a future session.
2. Focus on: decisions made, patterns established, errors encountered and their fixes, user preferences, project structure insights.
3. Do NOT save: trivial greetings, obvious code patterns, temporary debugging info, or sensitive data (API keys, credentials).
4. Write each memory as a concise, actionable note.
5. If updating existing memories, preserve important context while adding new information.

## Output Format

Output your extracted memories in this format:

---FILE: <filename>.md---
<memory content>
---END FILE---

Write one block per memory file. Use descriptive filenames (e.g., "auth_patterns.md", "project_structure.md").
If no new memories worth saving, respond with: NO_NEW_MEMORIES
`)

	return sb.String()
}

// parseExtractedMemories parses the forked agent's response into individual
// memory file entries. Expects the ---FILE: name.md--- / ---END FILE--- format.
func parseExtractedMemories(response string) map[string]string {
	memories := make(map[string]string)

	parts := strings.Split(response, "---FILE: ")
	for _, part := range parts[1:] { // skip first empty part
		endIdx := strings.Index(part, "---")
		if endIdx < 0 {
			continue
		}
		filename := strings.TrimSpace(part[:endIdx])

		contentStart := endIdx + 3 // skip "---"
		// Skip the newline after the closing ---
		if contentStart < len(part) && part[contentStart] == '\n' {
			contentStart++
		}

		endFileIdx := strings.Index(part[contentStart:], "---END FILE---")
		var content string
		if endFileIdx >= 0 {
			content = strings.TrimSpace(part[contentStart : contentStart+endFileIdx])
		} else {
			content = strings.TrimSpace(part[contentStart:])
		}

		if filename != "" && content != "" {
			memories[filename] = content
		}
	}

	return memories
}

// ExtractMemories runs memory extraction using a forked agent.
// Called at session end when auto-dream thresholds are met.
//
// The extraction flow:
// 1. Scan existing memory files in the auto-memory directory
// 2. Build an extraction prompt including the existing memory manifest
// 3. Call RunForkedAgent with the extraction prompt (sharing parent's cache)
// 4. Parse the response for memory entries
// 5. Write updated memory files to the auto-memory directory
func ExtractMemories(ctx context.Context, client *api.Client, params ExtractMemoriesParams) error {
	if client == nil {
		return fmt.Errorf("memory extraction requires an API client")
	}

	memDir := params.MemoryDir
	if memDir == "" {
		memDir = GetAutoMemoryDirDefault(params.ProjectPath)
	}

	// Scan existing memory files
	existingFiles, err := ScanMemoryFiles(memDir)
	if err != nil {
		// Non-fatal: proceed without existing memories
		fmt.Fprintf(os.Stderr, "[extractMemories] warning: failed to scan memory dir: %v\n", err)
		existingFiles = []MemoryFileEntry{}
	}

	// Build the extraction prompt with existing memory manifest
	manifest := formatMemoryManifest(existingFiles)
	userPrompt := buildExtractionPrompt(manifest, len(params.Messages))

	// Build cache-safe params, using provided or falling back to defaults
	cacheSafe := CacheSafeParams{
		SystemPrompt:        MemoryUpdatePrompt,
		Model:               client.Model,
		ForkContextMessages: params.Messages,
	}
	if params.CacheSafeParams != nil {
		cacheSafe = *params.CacheSafeParams
	}

	// Run the forked agent
	maxTokens := params.Config.MaxOutputTokens
	if maxTokens <= 0 {
		maxTokens = DefaultForkedAgentMaxTokens
	}

	result, err := RunForkedAgent(ctx, client, ForkedAgentParams{
		CacheSafe:       cacheSafe,
		UserMessage:     userPrompt,
		MaxOutputTokens: maxTokens,
		AbortCtx:        ctx,
		AgentID:         "memory_extraction",
		ForkReason:      "memory_extraction",
	})
	if err != nil {
		return fmt.Errorf("memory extraction forked agent failed: %w", err)
	}

	// Check for no-op response
	if strings.Contains(result.Response, "NO_NEW_MEMORIES") {
		fmt.Fprintf(os.Stderr, "[extractMemories] no new memories to save\n")
		return nil
	}

	// Parse the response into memory entries
	memories := parseExtractedMemories(result.Response)
	if len(memories) == 0 {
		fmt.Fprintf(os.Stderr, "[extractMemories] no parseable memories in response\n")
		return nil
	}

	// Write each memory file
	var writeErrors []string
	for name, content := range memories {
		if err := WriteMemoryFile(memDir, name, content); err != nil {
			writeErrors = append(writeErrors, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(writeErrors) > 0 {
		return fmt.Errorf("failed to write some memory files: %s", strings.Join(writeErrors, "; "))
	}

	fmt.Fprintf(os.Stderr, "[extractMemories] saved %d memory files to %s\n", len(memories), memDir)
	return nil
}
