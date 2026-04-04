package systemprompt

import (
	"strings"
	"testing"
)

func TestGetSystemPromptFullMode(t *testing.T) {
	cfg := SystemPromptConfig{
		EnvInfo: EnvInfoConfig{
			WorkDir:         "/home/user/project",
			IsGitRepo:       true,
			Platform:        "linux",
			Shell:           "zsh",
			OSVersion:       "Linux 6.8.0",
			ModelID:         "claude-opus-4-6",
			KnowledgeCutoff: "May 2025",
		},
		KeepCodingInstr: true,
		UseGlobalCache:  true,
		SimpleMode:      false,
	}

	sections := GetSystemPrompt(cfg)

	// Expected: 7 static + 1 boundary + 2 dynamic = 10
	// Static: Intro, System, DoingTasks, Actions, UsingTools, ToneStyle, OutputEfficiency
	// Boundary: DynamicBoundaryMarker
	// Dynamic: SessionGuidance, EnvInfo
	expectedCount := 10
	if len(sections) != expectedCount {
		t.Errorf("expected %d sections, got %d", expectedCount, len(sections))
		for i, s := range sections {
			t.Logf("  section[%d]: %s...", i, truncate(s, 80))
		}
	}

	// Verify DynamicBoundaryMarker is present
	found := false
	for _, s := range sections {
		if s == DynamicBoundaryMarker {
			found = true
			break
		}
	}
	if !found {
		t.Error("DynamicBoundaryMarker not found in sections")
	}

	// Verify boundary is between static and dynamic
	boundaryIdx := -1
	for i, s := range sections {
		if s == DynamicBoundaryMarker {
			boundaryIdx = i
			break
		}
	}
	if boundaryIdx < 1 {
		t.Error("boundary marker should not be the first section")
	}
	if boundaryIdx >= len(sections)-1 {
		t.Error("boundary marker should not be the last section")
	}
}

func TestGetSystemPromptNoBoundary(t *testing.T) {
	cfg := SystemPromptConfig{
		EnvInfo: EnvInfoConfig{
			WorkDir:         "/home/user/project",
			IsGitRepo:       true,
			Platform:        "linux",
			Shell:           "zsh",
			OSVersion:       "Linux 6.8.0",
			ModelID:         "claude-opus-4-6",
			KnowledgeCutoff: "May 2025",
		},
		KeepCodingInstr: true,
		UseGlobalCache:  false,
		SimpleMode:      false,
	}

	sections := GetSystemPrompt(cfg)

	// Without global cache, boundary marker should be absent
	for _, s := range sections {
		if s == DynamicBoundaryMarker {
			t.Error("DynamicBoundaryMarker should not be present when UseGlobalCache=false")
		}
	}

	// Expected: 7 static + 2 dynamic = 9 (no boundary)
	expectedCount := 9
	if len(sections) != expectedCount {
		t.Errorf("expected %d sections, got %d", expectedCount, len(sections))
	}
}

func TestGetSystemPromptSimpleMode(t *testing.T) {
	cfg := SystemPromptConfig{
		EnvInfo: EnvInfoConfig{
			WorkDir:         "/home/user/project",
			KnowledgeCutoff: "2026-04-04",
		},
		SimpleMode: true,
	}

	sections := GetSystemPrompt(cfg)

	// SimpleMode returns exactly 3 sections
	if len(sections) != 3 {
		t.Errorf("expected 3 sections in SimpleMode, got %d", len(sections))
		for i, s := range sections {
			t.Logf("  section[%d]: %s", i, s)
		}
	}

	// First section should mention Claude Code
	if !strings.Contains(sections[0], "Claude Code") {
		t.Error("simple mode first section should mention 'Claude Code'")
	}

	// Second section should contain the working directory
	if !strings.Contains(sections[1], "/home/user/project") {
		t.Error("simple mode should contain the working directory")
	}

	// Third section should contain the date
	if !strings.Contains(sections[2], "2026-04-04") {
		t.Error("simple mode should contain the date/cutoff")
	}
}

func TestGetSystemPromptOmitDoingTasks(t *testing.T) {
	cfg := SystemPromptConfig{
		EnvInfo: EnvInfoConfig{
			WorkDir:         "/tmp",
			Platform:        "linux",
			Shell:           "bash",
			OSVersion:       "Linux 6.8.0",
			ModelID:         "claude-sonnet-4-6",
			KnowledgeCutoff: "August 2025",
		},
		KeepCodingInstr: false,
		UseGlobalCache:  false,
		SimpleMode:      false,
	}

	sections := GetSystemPrompt(cfg)

	// When KeepCodingInstr=false, DoingTasks section should be omitted
	for _, s := range sections {
		if strings.Contains(s, "# Doing tasks") {
			t.Error("DoingTasks section should be omitted when KeepCodingInstr=false")
		}
	}

	// Expected: 6 static (no DoingTasks) + 2 dynamic = 8
	expectedCount := 8
	if len(sections) != expectedCount {
		t.Errorf("expected %d sections without DoingTasks, got %d", expectedCount, len(sections))
	}
}

func TestGetStaticSections(t *testing.T) {
	cfg := SystemPromptConfig{
		KeepCodingInstr: true,
	}

	static := GetStaticSections(cfg)

	// Should have 7 sections: Intro, System, DoingTasks, Actions, UsingTools, ToneStyle, OutputEfficiency
	if len(static) != 7 {
		t.Errorf("expected 7 static sections, got %d", len(static))
	}

	// First section should be the intro
	if !strings.Contains(static[0], "interactive agent") {
		t.Error("first static section should be the intro")
	}

	// Last section should be output efficiency
	if !strings.Contains(static[len(static)-1], "Output efficiency") {
		t.Error("last static section should be output efficiency")
	}
}

func TestGetDynamicSections(t *testing.T) {
	cfg := SystemPromptConfig{
		EnvInfo: EnvInfoConfig{
			WorkDir:         "/home/user/project",
			IsGitRepo:       true,
			Platform:        "linux",
			Shell:           "zsh",
			OSVersion:       "Linux 6.8.0",
			ModelID:         "claude-opus-4-6",
			KnowledgeCutoff: "May 2025",
		},
	}

	dynamic := GetDynamicSections(cfg)

	// Should have 2 sections: SessionGuidance, EnvInfo
	if len(dynamic) != 2 {
		t.Errorf("expected 2 dynamic sections, got %d", len(dynamic))
	}

	// First should be session guidance
	if !strings.Contains(dynamic[0], "Session-specific guidance") {
		t.Error("first dynamic section should be session guidance")
	}

	// Second should be env info
	if !strings.Contains(dynamic[1], "# Environment") {
		t.Error("second dynamic section should be env info")
	}
}

func TestDynamicBoundaryMarkerValue(t *testing.T) {
	if DynamicBoundaryMarker != "__SYSTEM_PROMPT_DYNAMIC_BOUNDARY__" {
		t.Errorf("DynamicBoundaryMarker should be '__SYSTEM_PROMPT_DYNAMIC_BOUNDARY__', got '%s'", DynamicBoundaryMarker)
	}
}

func TestGetSystemPromptBoundaryPosition(t *testing.T) {
	cfg := SystemPromptConfig{
		EnvInfo: EnvInfoConfig{
			WorkDir:         "/home/user/project",
			IsGitRepo:       true,
			Platform:        "linux",
			Shell:           "zsh",
			OSVersion:       "Linux 6.8.0",
			ModelID:         "claude-opus-4-6",
			KnowledgeCutoff: "May 2025",
		},
		KeepCodingInstr: true,
		UseGlobalCache:  true,
		SimpleMode:      false,
	}

	sections := GetSystemPrompt(cfg)

	// Find boundary position
	boundaryIdx := -1
	for i, s := range sections {
		if s == DynamicBoundaryMarker {
			boundaryIdx = i
			break
		}
	}

	if boundaryIdx == -1 {
		t.Fatal("DynamicBoundaryMarker not found")
	}

	// Boundary should be at index 7 (after 7 static sections)
	expectedIdx := 7
	if boundaryIdx != expectedIdx {
		t.Errorf("boundary at index %d, expected %d", boundaryIdx, expectedIdx)
	}

	// Everything before boundary should be static content
	for i := 0; i < boundaryIdx; i++ {
		if sections[i] == DynamicBoundaryMarker {
			t.Errorf("unexpected boundary at position %d", i)
		}
	}

	// Everything after boundary should be dynamic content
	afterBoundary := sections[boundaryIdx+1:]
	if len(afterBoundary) != 2 {
		t.Errorf("expected 2 sections after boundary, got %d", len(afterBoundary))
	}
}

// truncate shortens a string to maxLen characters for logging.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
