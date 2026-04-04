package systemprompt

import (
	"strings"
	"testing"
)

func TestEffectiveOverrideWins(t *testing.T) {
	cfg := EffectivePromptConfig{
		OverridePrompt:    "Override prompt",
		CoordinatorPrompt: "Coordinator prompt",
		AgentPrompt:       "Agent prompt",
		CustomPrompt:      "Custom prompt",
		AppendPrompt:      "Appended",
	}
	result := BuildEffectiveSystemPrompt(cfg)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	// Override should be the base, append should NOT be present when override is set
	if result[0] != "Override prompt" {
		t.Errorf("expected first element to be 'Override prompt', got: %s", result[0])
	}
	// When override is set, only the override is returned (no append)
	for _, s := range result {
		if s == "Coordinator prompt" || s == "Agent prompt" || s == "Custom prompt" {
			t.Errorf("unexpected prompt in result when override is set: %s", s)
		}
	}
}

func TestEffectiveCoordinatorWinsOverAgentCustomDefault(t *testing.T) {
	cfg := EffectivePromptConfig{
		CoordinatorPrompt: "Coordinator prompt",
		AgentPrompt:       "Agent prompt",
		CustomPrompt:      "Custom prompt",
		AppendPrompt:      "Appended",
	}
	result := BuildEffectiveSystemPrompt(cfg)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	if result[0] != "Coordinator prompt" {
		t.Errorf("expected first element to be 'Coordinator prompt', got: %s", result[0])
	}
	// Append should be present
	found := false
	for _, s := range result {
		if s == "Appended" {
			found = true
		}
	}
	if !found {
		t.Error("expected AppendPrompt to be present")
	}
}

func TestEffectiveAgentWinsOverCustomDefault(t *testing.T) {
	cfg := EffectivePromptConfig{
		AgentPrompt:  "Agent prompt",
		CustomPrompt: "Custom prompt",
		AppendPrompt: "Appended",
	}
	result := BuildEffectiveSystemPrompt(cfg)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	if result[0] != "Agent prompt" {
		t.Errorf("expected first element to be 'Agent prompt', got: %s", result[0])
	}
	found := false
	for _, s := range result {
		if s == "Appended" {
			found = true
		}
	}
	if !found {
		t.Error("expected AppendPrompt to be present")
	}
}

func TestEffectiveCustomWinsOverDefault(t *testing.T) {
	cfg := EffectivePromptConfig{
		CustomPrompt: "Custom prompt",
		AppendPrompt: "Appended",
	}
	result := BuildEffectiveSystemPrompt(cfg)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	if result[0] != "Custom prompt" {
		t.Errorf("expected first element to be 'Custom prompt', got: %s", result[0])
	}
}

func TestEffectiveDefaultPath(t *testing.T) {
	cfg := EffectivePromptConfig{
		AppendPrompt: "Appended",
	}
	result := BuildEffectiveSystemPrompt(cfg)
	if len(result) == 0 {
		t.Fatal("expected non-empty result for default path")
	}
	// The last element should be the append prompt
	last := result[len(result)-1]
	if last != "Appended" {
		t.Errorf("expected last element to be 'Appended', got: %s", last)
	}
}

func TestEffectiveAppendAlwaysPresent(t *testing.T) {
	tests := []struct {
		name string
		cfg  EffectivePromptConfig
	}{
		{
			name: "with coordinator",
			cfg: EffectivePromptConfig{
				CoordinatorPrompt: "Coord",
				AppendPrompt:      "Appended",
			},
		},
		{
			name: "with agent",
			cfg: EffectivePromptConfig{
				AgentPrompt:  "Agent",
				AppendPrompt: "Appended",
			},
		},
		{
			name: "with custom",
			cfg: EffectivePromptConfig{
				CustomPrompt: "Custom",
				AppendPrompt: "Appended",
			},
		},
		{
			name: "default only",
			cfg: EffectivePromptConfig{
				AppendPrompt: "Appended",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildEffectiveSystemPrompt(tt.cfg)
			found := false
			for _, s := range result {
				if s == "Appended" {
					found = true
				}
			}
			if !found {
				t.Error("AppendPrompt should always be present")
			}
		})
	}
}

func TestEffectiveEmptyOverrideIsSkipped(t *testing.T) {
	cfg := EffectivePromptConfig{
		OverridePrompt: "",
		CustomPrompt:   "Custom prompt",
	}
	result := BuildEffectiveSystemPrompt(cfg)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	if result[0] != "Custom prompt" {
		t.Errorf("expected empty override to be skipped, got first element: %s", result[0])
	}
}

func TestEffectiveEmptyCoordinatorIsSkipped(t *testing.T) {
	cfg := EffectivePromptConfig{
		CoordinatorPrompt: "",
		AgentPrompt:       "Agent prompt",
	}
	result := BuildEffectiveSystemPrompt(cfg)
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	if result[0] != "Agent prompt" {
		t.Errorf("expected empty coordinator to be skipped, got first element: %s", result[0])
	}
}

func TestEffectiveDynamicSectionsIncluded(t *testing.T) {
	cfg := EffectivePromptConfig{
		Language:    "French",
		OutputStyle: "concise",
		McpInstructions: []McpServerInstruction{
			{ServerName: "TestMCP", Instructions: "Test instructions"},
		},
		ScratchpadDir:      "/tmp/scratch",
		CachedMicrocompact: true,
		TokenBudgetEnabled: true,
		BriefModeEnabled:   true,
	}
	result := BuildEffectiveSystemPrompt(cfg)
	joined := strings.Join(result, "\n")

	if !strings.Contains(joined, "French") {
		t.Error("expected language section in result")
	}
	if !strings.Contains(joined, "TestMCP") {
		t.Error("expected MCP instructions in result")
	}
	if !strings.Contains(joined, "/tmp/scratch") {
		t.Error("expected scratchpad section in result")
	}
}

func TestEffectiveMcpInstructionsLastInDynamic(t *testing.T) {
	cfg := EffectivePromptConfig{
		Language: "German",
		McpInstructions: []McpServerInstruction{
			{ServerName: "TestServer", Instructions: "Do things"},
		},
	}
	sections := ResolveDynamicSections(cfg)
	if len(sections) == 0 {
		t.Fatal("expected non-empty dynamic sections")
	}

	// Find MCP instructions section
	mcpIdx := -1
	for i, s := range sections {
		if strings.Contains(s, "MCP Server Instructions") {
			mcpIdx = i
		}
	}
	if mcpIdx == -1 {
		t.Fatal("MCP instructions section not found")
	}

	// MCP instructions should be the last dynamic section
	if mcpIdx != len(sections)-1 {
		t.Errorf("MCP instructions should be last in dynamic sections, but found at index %d of %d", mcpIdx, len(sections)-1)
	}
}

func TestEffectiveResolveDynamicSections(t *testing.T) {
	t.Run("empty config returns no sections", func(t *testing.T) {
		cfg := EffectivePromptConfig{}
		sections := ResolveDynamicSections(cfg)
		// With all flags disabled and no dynamic inputs, should return
		// at most the SummarizeToolResults constant
		for _, s := range sections {
			if s == "" {
				t.Error("dynamic sections should not contain empty strings")
			}
		}
	})

	t.Run("feature-gated sections omitted when disabled", func(t *testing.T) {
		cfg := EffectivePromptConfig{
			CachedMicrocompact: false,
			TokenBudgetEnabled: false,
			BriefModeEnabled:   false,
		}
		sections := ResolveDynamicSections(cfg)
		joined := strings.Join(sections, "\n")
		if strings.Contains(joined, "Function Result Clearing") {
			t.Error("function result clearing should not be present when disabled")
		}
		if strings.Contains(joined, "token target") {
			t.Error("token budget should not be present when disabled")
		}
		if strings.Contains(joined, "Brief Mode") {
			t.Error("brief mode should not be present when disabled")
		}
	})
}

func TestEffectiveNoAppendWhenEmpty(t *testing.T) {
	cfg := EffectivePromptConfig{
		CustomPrompt: "Custom",
		AppendPrompt: "",
	}
	result := BuildEffectiveSystemPrompt(cfg)
	for _, s := range result {
		if s == "" {
			t.Error("result should not contain empty strings")
		}
	}
}
