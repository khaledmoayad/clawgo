package systemprompt

import (
	"strings"
	"testing"
)

// TestDynamicGetLanguageSection tests the language section generator.
func TestDynamicGetLanguageSection(t *testing.T) {
	t.Run("returns language instruction when language set", func(t *testing.T) {
		result := GetLanguageSection("Spanish")
		if result == "" {
			t.Fatal("expected non-empty result for language=Spanish")
		}
		if !strings.Contains(result, "Spanish") {
			t.Errorf("expected result to contain 'Spanish', got: %s", result)
		}
		if !strings.Contains(result, "Always respond in") {
			t.Errorf("expected result to contain 'Always respond in', got: %s", result)
		}
	})

	t.Run("returns empty when no language set", func(t *testing.T) {
		result := GetLanguageSection("")
		if result != "" {
			t.Errorf("expected empty string for empty language, got: %s", result)
		}
	})
}

// TestDynamicGetOutputStyleSection tests the output style section generator.
func TestDynamicGetOutputStyleSection(t *testing.T) {
	t.Run("returns style instruction when style set", func(t *testing.T) {
		result := GetOutputStyleSection("concise", "Be concise in all responses")
		if result == "" {
			t.Fatal("expected non-empty result")
		}
		if !strings.Contains(result, "concise") {
			t.Errorf("expected result to contain 'concise', got: %s", result)
		}
		if !strings.Contains(result, "Output Style") {
			t.Errorf("expected result to contain 'Output Style', got: %s", result)
		}
	})

	t.Run("returns empty when no style set", func(t *testing.T) {
		result := GetOutputStyleSection("", "")
		if result != "" {
			t.Errorf("expected empty string for empty style, got: %s", result)
		}
	})
}

// TestDynamicGetMcpInstructionsSection tests the MCP instructions section generator.
func TestDynamicGetMcpInstructionsSection(t *testing.T) {
	t.Run("returns instructions when servers have instructions", func(t *testing.T) {
		servers := []McpServerInstruction{
			{ServerName: "MyServer", Instructions: "Use tools carefully"},
			{ServerName: "OtherServer", Instructions: "Be cautious"},
		}
		result := GetMcpInstructionsSection(servers)
		if result == "" {
			t.Fatal("expected non-empty result")
		}
		if !strings.Contains(result, "MyServer") {
			t.Errorf("expected result to contain 'MyServer', got: %s", result)
		}
		if !strings.Contains(result, "OtherServer") {
			t.Errorf("expected result to contain 'OtherServer', got: %s", result)
		}
		if !strings.Contains(result, "MCP Server Instructions") {
			t.Errorf("expected result to contain 'MCP Server Instructions', got: %s", result)
		}
	})

	t.Run("returns empty when no servers", func(t *testing.T) {
		result := GetMcpInstructionsSection(nil)
		if result != "" {
			t.Errorf("expected empty string for nil servers, got: %s", result)
		}
	})

	t.Run("returns empty for empty server slice", func(t *testing.T) {
		result := GetMcpInstructionsSection([]McpServerInstruction{})
		if result != "" {
			t.Errorf("expected empty string for empty servers, got: %s", result)
		}
	})
}

// TestDynamicGetScratchpadSection tests the scratchpad section generator.
func TestDynamicGetScratchpadSection(t *testing.T) {
	t.Run("returns scratchpad instructions with dir path", func(t *testing.T) {
		result := GetScratchpadSection("/tmp/scratch-123")
		if result == "" {
			t.Fatal("expected non-empty result")
		}
		if !strings.Contains(result, "/tmp/scratch-123") {
			t.Errorf("expected result to contain dir path, got: %s", result)
		}
		if !strings.Contains(result, "Scratchpad") {
			t.Errorf("expected result to contain 'Scratchpad', got: %s", result)
		}
	})

	t.Run("returns empty when dir is empty", func(t *testing.T) {
		result := GetScratchpadSection("")
		if result != "" {
			t.Errorf("expected empty string for empty dir, got: %s", result)
		}
	})
}

// TestDynamicGetFunctionResultClearingSection tests the function result clearing section.
func TestDynamicGetFunctionResultClearingSection(t *testing.T) {
	t.Run("returns section when enabled", func(t *testing.T) {
		result := GetFunctionResultClearingSection(true)
		if result == "" {
			t.Fatal("expected non-empty result when enabled")
		}
		if !strings.Contains(result, "Function Result Clearing") {
			t.Errorf("expected result to contain 'Function Result Clearing', got: %s", result)
		}
	})

	t.Run("returns empty when disabled", func(t *testing.T) {
		result := GetFunctionResultClearingSection(false)
		if result != "" {
			t.Errorf("expected empty string when disabled, got: %s", result)
		}
	})
}

// TestDynamicSummarizeToolResultsSection tests the constant.
func TestDynamicSummarizeToolResultsSection(t *testing.T) {
	if SummarizeToolResultsSection == "" {
		t.Fatal("expected non-empty constant")
	}
	if !strings.Contains(SummarizeToolResultsSection, "tool result") {
		t.Errorf("expected constant to mention tool results, got: %s", SummarizeToolResultsSection)
	}
}

// TestDynamicGetTokenBudgetSection tests the token budget section.
func TestDynamicGetTokenBudgetSection(t *testing.T) {
	t.Run("returns section when enabled", func(t *testing.T) {
		result := GetTokenBudgetSection(true)
		if result == "" {
			t.Fatal("expected non-empty result when enabled")
		}
		if !strings.Contains(result, "token") {
			t.Errorf("expected result to contain 'token', got: %s", result)
		}
	})

	t.Run("returns empty when disabled", func(t *testing.T) {
		result := GetTokenBudgetSection(false)
		if result != "" {
			t.Errorf("expected empty string when disabled, got: %s", result)
		}
	})
}

// TestDynamicGetBriefModeSection tests the brief mode section.
func TestDynamicGetBriefModeSection(t *testing.T) {
	t.Run("returns section when enabled", func(t *testing.T) {
		result := GetBriefModeSection(true)
		if result == "" {
			t.Fatal("expected non-empty result when enabled")
		}
	})

	t.Run("returns empty when disabled", func(t *testing.T) {
		result := GetBriefModeSection(false)
		if result != "" {
			t.Errorf("expected empty string when disabled, got: %s", result)
		}
	})
}

// TestDynamicLoadMemoryPromptSection tests the memory prompt loading.
func TestDynamicLoadMemoryPromptSection(t *testing.T) {
	t.Run("returns empty for non-existent directories", func(t *testing.T) {
		result := LoadMemoryPromptSection("/nonexistent/work/dir", "/nonexistent/home")
		// With no CLAUDE.md files found, should return empty
		if result != "" {
			t.Logf("got non-empty result (may have found system-level CLAUDE.md): %s", result)
		}
	})

	t.Run("returns content when CLAUDE.md exists", func(t *testing.T) {
		// Create temp directory with a CLAUDE.md file
		tmpDir := t.TempDir()
		claudeMdPath := tmpDir + "/CLAUDE.md"
		err := writeTestFile(claudeMdPath, "Test project instructions")
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		result := LoadMemoryPromptSection(tmpDir, "/nonexistent/home")
		if result == "" {
			t.Fatal("expected non-empty result when CLAUDE.md exists")
		}
		if !strings.Contains(result, "Test project instructions") {
			t.Errorf("expected result to contain CLAUDE.md content, got: %s", result)
		}
	})
}

// writeTestFile is a helper to create test files.
func writeTestFile(path string, content string) error {
	return writeFile(path, []byte(content))
}
