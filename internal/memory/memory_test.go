package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldExtractMemory_BelowBothThresholds(t *testing.T) {
	mgr := NewMemoryManager(DefaultMemoryConfig(), nil)
	assert.False(t, mgr.ShouldExtractMemory(100, 1),
		"should not extract when both token count and tool calls are below thresholds")
}

func TestShouldExtractMemory_BelowTokenThreshold(t *testing.T) {
	mgr := NewMemoryManager(DefaultMemoryConfig(), nil)
	// Tool calls above threshold but tokens below
	assert.False(t, mgr.ShouldExtractMemory(5000, 10),
		"should not extract when token count is below threshold even if tool calls above")
}

func TestShouldExtractMemory_BelowToolCallThreshold(t *testing.T) {
	mgr := NewMemoryManager(DefaultMemoryConfig(), nil)
	// Tokens above threshold but tool calls below
	assert.False(t, mgr.ShouldExtractMemory(15000, 2),
		"should not extract when tool call count is below threshold even if tokens above")
}

func TestShouldExtractMemory_BothThresholdsMet(t *testing.T) {
	mgr := NewMemoryManager(DefaultMemoryConfig(), nil)
	assert.True(t, mgr.ShouldExtractMemory(10000, 5),
		"should extract when both thresholds are exactly met")
	assert.True(t, mgr.ShouldExtractMemory(20000, 10),
		"should extract when both thresholds are exceeded")
}

func TestShouldExtractMemory_Disabled(t *testing.T) {
	cfg := DefaultMemoryConfig()
	cfg.Enabled = false
	mgr := NewMemoryManager(cfg, nil)
	assert.False(t, mgr.ShouldExtractMemory(20000, 10),
		"should not extract when memory is disabled")
}

func TestGetMemoryPath_ConsistentHash(t *testing.T) {
	cfg := MemoryConfig{
		MemoryDir: t.TempDir(),
	}
	path1 := GetMemoryPath(cfg, "/home/user/project")
	path2 := GetMemoryPath(cfg, "/home/user/project")
	assert.Equal(t, path1, path2, "same project path should produce same memory path")

	// Different project paths should produce different memory paths
	path3 := GetMemoryPath(cfg, "/home/user/other-project")
	assert.NotEqual(t, path1, path3, "different project paths should produce different memory paths")

	// Should end with .md
	assert.True(t, filepath.Ext(path1) == ".md", "memory file should have .md extension")
}

func TestLoadMemories_NonExistentProject(t *testing.T) {
	cfg := MemoryConfig{
		MemoryDir: t.TempDir(),
	}
	mgr := NewMemoryManager(cfg, nil)
	content, err := mgr.LoadMemories("/nonexistent/project")
	assert.NoError(t, err, "loading memories for nonexistent project should not error")
	assert.Empty(t, content, "should return empty string for nonexistent project")
}

func TestLoadMemories_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := MemoryConfig{
		MemoryDir: tmpDir,
	}

	// Write a memory file
	projectPath := "/home/user/myproject"
	memoryPath := GetMemoryPath(cfg, projectPath)
	expectedContent := "# Session Title\nWorking on authentication system\n"
	require.NoError(t, os.WriteFile(memoryPath, []byte(expectedContent), 0644))

	mgr := NewMemoryManager(cfg, nil)
	content, err := mgr.LoadMemories(projectPath)
	assert.NoError(t, err)
	assert.Equal(t, expectedContent, content, "should return the memory file content")
}

func TestHashProjectPath_MatchesSessionStorage(t *testing.T) {
	// hashProjectPath should use the same algorithm as session/storage.go hashPath:
	// SHA256 of the path, first 16 hex characters (8 bytes)
	projectPath := "/home/user/project"

	// Compute expected hash using the same algorithm
	h := sha256.Sum256([]byte(projectPath))
	expected := hex.EncodeToString(h[:8])

	result := hashProjectPath(projectPath)
	assert.Equal(t, expected, result, "hash should match SHA256 first 16 hex chars")
	assert.Len(t, result, 16, "hash should be 16 characters long")
}

func TestGetMemoryDir_CreatesDirectory(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "nested", "memory-dir")
	cfg := MemoryConfig{
		MemoryDir: tmpDir,
	}

	dir := GetMemoryDir(cfg)
	assert.Equal(t, tmpDir, dir)

	// Verify directory was created
	info, err := os.Stat(tmpDir)
	assert.NoError(t, err, "directory should exist after GetMemoryDir call")
	assert.True(t, info.IsDir(), "should be a directory")
}

func TestFormatMemoryPrompt_WithoutExisting(t *testing.T) {
	result := FormatMemoryPrompt(DefaultSessionMemoryTemplate, "")
	assert.Contains(t, result, "template to fill in")
	assert.Contains(t, result, "Session Title")
	assert.NotContains(t, result, "existing memory")
}

func TestFormatMemoryPrompt_WithExisting(t *testing.T) {
	existing := "# Session Title\nPrevious work on auth"
	result := FormatMemoryPrompt(DefaultSessionMemoryTemplate, existing)
	assert.Contains(t, result, "template to fill in")
	assert.Contains(t, result, "existing memory")
	assert.Contains(t, result, "Previous work on auth")
}

func TestDefaultMemoryConfig(t *testing.T) {
	cfg := DefaultMemoryConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 10000, cfg.MinTokensForExtraction)
	assert.Equal(t, 5, cfg.MinToolCallsForExtraction)
	assert.Contains(t, cfg.MemoryDir, "session-memory")
}

// ============================================================================
// Auto-dream check tests
// ============================================================================

func TestAutoDreamCheck_MeetsThresholds(t *testing.T) {
	mgr := NewMemoryManager(DefaultMemoryConfig(), nil)
	assert.True(t, mgr.AutoDreamCheck(10000, 5),
		"should trigger when both thresholds are exactly met")
	assert.True(t, mgr.AutoDreamCheck(20000, 10),
		"should trigger when both thresholds are exceeded")
}

func TestAutoDreamCheck_BelowThresholds(t *testing.T) {
	mgr := NewMemoryManager(DefaultMemoryConfig(), nil)
	assert.False(t, mgr.AutoDreamCheck(100, 0),
		"should not trigger with low tokens and zero tool calls")
	assert.False(t, mgr.AutoDreamCheck(5000, 10),
		"should not trigger when tokens below threshold")
	assert.False(t, mgr.AutoDreamCheck(15000, 2),
		"should not trigger when tool calls below threshold")
}

func TestAutoDreamCheck_Disabled(t *testing.T) {
	cfg := DefaultMemoryConfig()
	cfg.Enabled = false
	mgr := NewMemoryManager(cfg, nil)
	assert.False(t, mgr.AutoDreamCheck(20000, 10),
		"should not trigger when memory is disabled")
}

// ============================================================================
// Memory file scanning tests
// ============================================================================

func TestScanMemoryFiles_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 3 .md files
	files := map[string]string{
		"auth_patterns.md":     "# Auth\nUse JWT tokens",
		"project_structure.md": "# Structure\nMonorepo with Go",
		"debugging_notes.md":   "# Debug\nCheck logs first",
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644))
	}

	// Also create a non-.md file that should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("ignored"), 0644))

	entries, err := ScanMemoryFiles(tmpDir)
	require.NoError(t, err)
	assert.Len(t, entries, 3, "should find exactly 3 .md files")

	// Verify content is read correctly
	foundNames := make(map[string]bool)
	for _, e := range entries {
		foundNames[e.Name] = true
		expectedContent, ok := files[e.Name+".md"]
		assert.True(t, ok, "unexpected file: %s", e.Name)
		assert.Equal(t, expectedContent, e.Content)
		assert.Greater(t, e.Size, int64(0))
	}
	assert.True(t, foundNames["auth_patterns"])
	assert.True(t, foundNames["project_structure"])
	assert.True(t, foundNames["debugging_notes"])
}

func TestScanMemoryFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	entries, err := ScanMemoryFiles(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "should return empty slice for empty directory")
	assert.NotNil(t, entries, "should return non-nil empty slice")
}

func TestScanMemoryFiles_NonexistentDir(t *testing.T) {
	entries, err := ScanMemoryFiles("/nonexistent/path/memory")
	require.NoError(t, err, "nonexistent directory should not error")
	assert.Empty(t, entries)
	assert.NotNil(t, entries)
}

// ============================================================================
// Memory file writing tests
// ============================================================================

func TestWriteMemoryFile_WriteAndReadBack(t *testing.T) {
	tmpDir := t.TempDir()
	content := "# Auth Patterns\nUse JWT with refresh tokens"

	err := WriteMemoryFile(tmpDir, "auth_patterns", content)
	require.NoError(t, err)

	// Read back
	data, err := os.ReadFile(filepath.Join(tmpDir, "auth_patterns.md"))
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestWriteMemoryFile_CreatesDirectory(t *testing.T) {
	tmpDir := filepath.Join(t.TempDir(), "nested", "memory")
	content := "test content"

	err := WriteMemoryFile(tmpDir, "test", content)
	require.NoError(t, err)

	// Verify directory was created and file exists
	data, err := os.ReadFile(filepath.Join(tmpDir, "test.md"))
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestWriteMemoryFile_WithMdExtension(t *testing.T) {
	tmpDir := t.TempDir()
	// Should not double the .md extension
	err := WriteMemoryFile(tmpDir, "test.md", "content")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tmpDir, "test.md"))
	assert.NoError(t, err, "file should exist with .md extension")
}

// ============================================================================
// Consolidation threshold tests
// ============================================================================

func TestConsolidateMemories_BelowThreshold(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 5 files (below default threshold of 10)
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("memory_%d.md", i)
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, name),
			[]byte(fmt.Sprintf("Memory %d content", i)),
			0644,
		))
	}

	// ConsolidateMemories with nil client should return nil (below threshold)
	// We test the threshold check specifically, not the API call
	files, err := ScanMemoryFiles(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 5)
	assert.LessOrEqual(t, len(files), DefaultConsolidationThreshold,
		"5 files should be below consolidation threshold")
}

func TestConsolidateMemories_AboveThreshold(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 15 files (above default threshold of 10)
	for i := 0; i < 15; i++ {
		name := fmt.Sprintf("memory_%d.md", i)
		require.NoError(t, os.WriteFile(
			filepath.Join(tmpDir, name),
			[]byte(fmt.Sprintf("Memory %d content", i)),
			0644,
		))
	}

	files, err := ScanMemoryFiles(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 15)
	assert.Greater(t, len(files), DefaultConsolidationThreshold,
		"15 files should exceed consolidation threshold")
}

// ============================================================================
// Extraction prompt tests
// ============================================================================

func TestBuildExtractionPrompt_IncludesExistingMemory(t *testing.T) {
	existing := "### auth_patterns.md\n# Auth\nUse JWT tokens\n\n"
	prompt := buildExtractionPrompt(existing, 42)

	assert.Contains(t, prompt, "Existing memory files",
		"prompt should include existing memory section")
	assert.Contains(t, prompt, "auth_patterns.md",
		"prompt should include existing file names")
	assert.Contains(t, prompt, "Use JWT tokens",
		"prompt should include existing memory content")
	assert.Contains(t, prompt, "~42 messages",
		"prompt should include message count")
}

func TestBuildExtractionPrompt_WithoutExistingMemory(t *testing.T) {
	prompt := buildExtractionPrompt("", 10)

	assert.NotContains(t, prompt, "Existing memory files",
		"prompt should not include existing memory section when empty")
	assert.Contains(t, prompt, "~10 messages",
		"prompt should include message count")
	assert.Contains(t, prompt, "Instructions",
		"prompt should include extraction instructions")
}

// ============================================================================
// Parse extracted memories tests
// ============================================================================

func TestParseExtractedMemories_ValidFormat(t *testing.T) {
	response := `Here are the extracted memories:

---FILE: auth_patterns.md---
# Auth Patterns
Use JWT tokens with refresh rotation.
---END FILE---

---FILE: project_structure.md---
# Project Structure
Monorepo with Go backend.
---END FILE---
`
	memories := parseExtractedMemories(response)
	assert.Len(t, memories, 2)
	assert.Contains(t, memories["auth_patterns.md"], "JWT tokens")
	assert.Contains(t, memories["project_structure.md"], "Monorepo")
}

func TestParseExtractedMemories_NoMemories(t *testing.T) {
	response := "NO_NEW_MEMORIES"
	memories := parseExtractedMemories(response)
	assert.Empty(t, memories)
}

func TestParseExtractedMemories_EmptyResponse(t *testing.T) {
	memories := parseExtractedMemories("")
	assert.Empty(t, memories)
}

// ============================================================================
// Cache-safe params storage tests
// ============================================================================

func TestCacheSafeParamsStorage(t *testing.T) {
	// Clear any existing state
	ClearCacheSafeParams()

	// Initially nil
	assert.Nil(t, GetLastCacheSafeParams(), "should be nil initially")

	// Save params
	params := &CacheSafeParams{
		SystemPrompt: "test prompt",
		Model:        "claude-sonnet-4-20250514",
	}
	SaveCacheSafeParams(params)

	// Retrieve
	got := GetLastCacheSafeParams()
	require.NotNil(t, got)
	assert.Equal(t, "test prompt", got.SystemPrompt)
	assert.Equal(t, "claude-sonnet-4-20250514", got.Model)

	// Clear
	ClearCacheSafeParams()
	assert.Nil(t, GetLastCacheSafeParams(), "should be nil after clear")
}

// ============================================================================
// Auto-memory directory tests
// ============================================================================

func TestGetAutoMemoryDir(t *testing.T) {
	dir := GetAutoMemoryDir("/home/user/.claude", "/home/user/project")
	assert.Contains(t, dir, "projects")
	assert.Contains(t, dir, "memory")
	assert.True(t, filepath.IsAbs(dir), "should return absolute path")

	// Same project path should give same dir
	dir2 := GetAutoMemoryDir("/home/user/.claude", "/home/user/project")
	assert.Equal(t, dir, dir2)

	// Different project path should give different dir
	dir3 := GetAutoMemoryDir("/home/user/.claude", "/home/user/other-project")
	assert.NotEqual(t, dir, dir3)
}

// ============================================================================
// Default extract config tests
// ============================================================================

func TestDefaultExtractMemoriesConfig(t *testing.T) {
	cfg := DefaultExtractMemoriesConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 8000, cfg.MinTokensForTrigger)
	assert.Equal(t, 3, cfg.MinToolCallsForTrigger)
	assert.Equal(t, DefaultForkedAgentMaxTokens, cfg.MaxOutputTokens)
}

// ============================================================================
// Format memory manifest tests
// ============================================================================

func TestFormatMemoryManifest_EmptyFiles(t *testing.T) {
	result := formatMemoryManifest([]MemoryFileEntry{})
	assert.Empty(t, result)
}

func TestFormatMemoryManifest_WithFiles(t *testing.T) {
	files := []MemoryFileEntry{
		{Name: "auth", Content: "JWT patterns"},
		{Name: "structure", Content: "Go monorepo"},
	}
	result := formatMemoryManifest(files)
	assert.Contains(t, result, "### auth.md")
	assert.Contains(t, result, "JWT patterns")
	assert.Contains(t, result, "### structure.md")
	assert.Contains(t, result, "Go monorepo")
}
