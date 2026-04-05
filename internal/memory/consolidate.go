package memory

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/api"
)

// DefaultConsolidationThreshold is the number of memory files above which
// consolidation is triggered. When the auto-memory directory has more files
// than this, ConsolidateMemories merges them into fewer, more coherent files.
const DefaultConsolidationThreshold = 10

// ConsolidationPrompt is the system prompt for the consolidation forked agent.
const ConsolidationPrompt = `You are consolidating session memory files. You have been given multiple memory files that need to be merged into fewer, more coherent files.

Rules:
1. Merge related topics into single files (e.g., all auth-related memories into one "auth_patterns.md")
2. Remove duplicate information
3. Preserve all important details -- do not lose any unique information
4. Keep memories concise and actionable
5. Use clear, descriptive filenames

Output Format:
For each consolidated file, write:

---FILE: <filename>.md---
<consolidated content>
---END FILE---

Aim for 3-7 consolidated files total, organized by topic.`

// ConsolidateMemories merges multiple memory files into fewer, more coherent files.
// Called when the number of memory files exceeds the consolidation threshold.
//
// The consolidation flow:
// 1. Scan all memory files in the directory
// 2. If count <= threshold, skip (no consolidation needed)
// 3. Build a consolidation prompt with all memory contents
// 4. Call RunForkedAgent to produce consolidated memories
// 5. Write consolidated files, remove old files
func ConsolidateMemories(ctx context.Context, client *api.Client, memDir string, cacheSafe *CacheSafeParams) error {
	return ConsolidateMemoriesWithThreshold(ctx, client, memDir, cacheSafe, DefaultConsolidationThreshold)
}

// ConsolidateMemoriesWithThreshold is like ConsolidateMemories but allows
// specifying a custom consolidation threshold. Useful for testing.
func ConsolidateMemoriesWithThreshold(ctx context.Context, client *api.Client, memDir string, cacheSafe *CacheSafeParams, threshold int) error {
	if client == nil {
		return fmt.Errorf("consolidation requires an API client")
	}

	// Scan existing memory files
	files, err := ScanMemoryFiles(memDir)
	if err != nil {
		return fmt.Errorf("failed to scan memory files for consolidation: %w", err)
	}

	// Check threshold
	if len(files) <= threshold {
		return nil // No consolidation needed
	}

	fmt.Fprintf(os.Stderr, "[consolidate] %d files exceed threshold %d, consolidating\n", len(files), threshold)

	// Build the consolidation prompt with all memory contents
	var sb strings.Builder
	sb.WriteString("Here are the memory files to consolidate:\n\n")
	for _, f := range files {
		sb.WriteString(fmt.Sprintf("### %s.md (size: %d bytes)\n", f.Name, f.Size))
		sb.WriteString(f.Content)
		sb.WriteString("\n\n---\n\n")
	}
	sb.WriteString("Please consolidate these into fewer, well-organized files.")

	// Use provided cache-safe params or build minimal ones
	cs := CacheSafeParams{
		SystemPrompt: ConsolidationPrompt,
		Model:        client.Model,
	}
	if cacheSafe != nil {
		cs = *cacheSafe
		// Override system prompt for consolidation
		cs.SystemPrompt = ConsolidationPrompt
	}

	result, err := RunForkedAgent(ctx, client, ForkedAgentParams{
		CacheSafe:       cs,
		UserMessage:     sb.String(),
		MaxOutputTokens: DefaultForkedAgentMaxTokens,
		AbortCtx:        ctx,
		AgentID:         "memory_consolidation",
		ForkReason:      "consolidation",
	})
	if err != nil {
		return fmt.Errorf("consolidation forked agent failed: %w", err)
	}

	// Parse consolidated memories
	consolidated := parseExtractedMemories(result.Response)
	if len(consolidated) == 0 {
		fmt.Fprintf(os.Stderr, "[consolidate] no parseable files in consolidation response, keeping originals\n")
		return nil
	}

	// Remove old files
	for _, f := range files {
		if err := os.Remove(f.Path); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "[consolidate] warning: failed to remove %s: %v\n", f.Path, err)
		}
	}

	// Write consolidated files
	var writeErrors []string
	for name, content := range consolidated {
		if err := WriteMemoryFile(memDir, name, content); err != nil {
			writeErrors = append(writeErrors, fmt.Sprintf("%s: %v", name, err))
		}
	}

	if len(writeErrors) > 0 {
		return fmt.Errorf("failed to write some consolidated files: %s", strings.Join(writeErrors, "; "))
	}

	fmt.Fprintf(os.Stderr, "[consolidate] consolidated %d files into %d\n", len(files), len(consolidated))
	return nil
}
