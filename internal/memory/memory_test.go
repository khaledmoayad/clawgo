package memory

import (
	"crypto/sha256"
	"encoding/hex"
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
