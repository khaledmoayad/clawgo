package swarm

import (
	"strings"
	"testing"
)

func TestBuildTeammateSystemPromptDefault(t *testing.T) {
	parts := []string{"You are Claude.", "Be helpful."}
	result := BuildTeammateSystemPrompt(SystemPromptDefault, "", parts, "")

	// Should contain default parts
	if !strings.Contains(result, "You are Claude.") {
		t.Error("expected default prompt parts")
	}
	if !strings.Contains(result, "Be helpful.") {
		t.Error("expected all default prompt parts")
	}

	// Should contain teammate addendum
	if !strings.Contains(result, "Agent Teammate Communication") {
		t.Error("expected teammate addendum")
	}
	if !strings.Contains(result, "SendMessage tool") {
		t.Error("expected SendMessage instructions in addendum")
	}
}

func TestBuildTeammateSystemPromptReplace(t *testing.T) {
	parts := []string{"You are Claude.", "Be helpful."}
	custom := "You are a specialized code reviewer."
	result := BuildTeammateSystemPrompt(SystemPromptReplace, custom, parts, "")

	// Should ONLY contain the custom prompt
	if result != custom {
		t.Errorf("replace mode should return only custom prompt, got %q", result)
	}

	// Should NOT contain default parts or addendum
	if strings.Contains(result, "You are Claude.") {
		t.Error("replace mode should not include default parts")
	}
	if strings.Contains(result, "Teammate Communication") {
		t.Error("replace mode should not include addendum")
	}
}

func TestBuildTeammateSystemPromptAppend(t *testing.T) {
	parts := []string{"You are Claude."}
	custom := "Focus on security-related issues."
	result := BuildTeammateSystemPrompt(SystemPromptAppend, custom, parts, "")

	// Should contain default + addendum + custom (in order)
	if !strings.Contains(result, "You are Claude.") {
		t.Error("expected default prompt")
	}
	if !strings.Contains(result, "Agent Teammate Communication") {
		t.Error("expected teammate addendum")
	}
	if !strings.Contains(result, custom) {
		t.Error("expected custom prompt in append mode")
	}

	// Custom should come after addendum
	addendumIdx := strings.Index(result, "Agent Teammate Communication")
	customIdx := strings.Index(result, custom)
	if customIdx < addendumIdx {
		t.Error("custom prompt should appear after addendum in append mode")
	}
}

func TestBuildTeammateSystemPromptReplaceEmptyCustom(t *testing.T) {
	parts := []string{"You are Claude."}
	// Replace mode with empty custom prompt falls back to default behavior
	result := BuildTeammateSystemPrompt(SystemPromptReplace, "", parts, "")

	if !strings.Contains(result, "You are Claude.") {
		t.Error("replace mode with empty custom should use default parts")
	}
	if !strings.Contains(result, "Teammate Communication") {
		t.Error("replace mode with empty custom should include addendum")
	}
}

func TestBuildTeammateSystemPromptWithAgentDefinition(t *testing.T) {
	parts := []string{"You are Claude."}
	agentPrompt := "You specialize in testing React components."
	result := BuildTeammateSystemPrompt(SystemPromptDefault, "", parts, agentPrompt)

	if !strings.Contains(result, "# Custom Agent Instructions") {
		t.Error("expected custom agent instructions header")
	}
	if !strings.Contains(result, agentPrompt) {
		t.Error("expected agent definition prompt")
	}
}

func TestBuildTeammateSystemPromptAppendWithAgentDef(t *testing.T) {
	parts := []string{"You are Claude."}
	custom := "Be thorough."
	agentPrompt := "You are a security expert."
	result := BuildTeammateSystemPrompt(SystemPromptAppend, custom, parts, agentPrompt)

	// All parts should be present
	if !strings.Contains(result, "You are Claude.") {
		t.Error("missing default prompt")
	}
	if !strings.Contains(result, "Agent Teammate Communication") {
		t.Error("missing addendum")
	}
	if !strings.Contains(result, "Custom Agent Instructions") {
		t.Error("missing agent definition header")
	}
	if !strings.Contains(result, agentPrompt) {
		t.Error("missing agent definition prompt")
	}
	if !strings.Contains(result, custom) {
		t.Error("missing custom prompt")
	}
}

func TestBuildTeammateSystemPromptEmptyMode(t *testing.T) {
	// Empty string mode should behave as default
	parts := []string{"You are Claude."}
	result := BuildTeammateSystemPrompt("", "", parts, "")

	if !strings.Contains(result, "You are Claude.") {
		t.Error("empty mode should use default parts")
	}
	if !strings.Contains(result, "Teammate Communication") {
		t.Error("empty mode should include addendum")
	}
}

func TestTeamEssentialTools(t *testing.T) {
	tools := TeamEssentialTools()
	if len(tools) == 0 {
		t.Fatal("expected non-empty essential tools list")
	}

	// Must include SendMessage for communication
	found := false
	for _, tool := range tools {
		if tool == "SendMessage" {
			found = true
			break
		}
	}
	if !found {
		t.Error("TeamEssentialTools must include SendMessage")
	}

	// Must include task management tools
	required := []string{"TaskCreate", "TaskGet", "TaskList", "TaskUpdate"}
	for _, req := range required {
		found := false
		for _, tool := range tools {
			if tool == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("TeamEssentialTools must include %s", req)
		}
	}
}

func TestMergeToolPermissionsEmpty(t *testing.T) {
	// nil/empty = all tools
	result := MergeToolPermissions(nil)
	if len(result) != 1 || result[0] != "*" {
		t.Errorf("nil input should return [*], got %v", result)
	}

	result = MergeToolPermissions([]string{})
	if len(result) != 1 || result[0] != "*" {
		t.Errorf("empty input should return [*], got %v", result)
	}
}

func TestMergeToolPermissionsWithExplicit(t *testing.T) {
	allowed := []string{"Bash", "Read", "Edit"}
	result := MergeToolPermissions(allowed)

	// Should contain original tools
	contains := func(s string) bool {
		for _, r := range result {
			if r == s {
				return true
			}
		}
		return false
	}

	for _, tool := range allowed {
		if !contains(tool) {
			t.Errorf("result should contain %q", tool)
		}
	}

	// Should also contain team-essential tools
	for _, tool := range TeamEssentialTools() {
		if !contains(tool) {
			t.Errorf("result should contain essential tool %q", tool)
		}
	}
}

func TestMergeToolPermissionsNoDuplicates(t *testing.T) {
	// Include a team-essential tool in the allowed list
	allowed := []string{"Bash", "SendMessage", "Read"}
	result := MergeToolPermissions(allowed)

	// Count SendMessage occurrences
	count := 0
	for _, r := range result {
		if r == "SendMessage" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("SendMessage should appear exactly once, found %d times", count)
	}
}

func TestTeammateSystemPromptAddendumContent(t *testing.T) {
	// Verify the addendum contains critical communication instructions
	if !strings.Contains(TeammateSystemPromptAddendum, "SendMessage") {
		t.Error("addendum must mention SendMessage tool")
	}
	if !strings.Contains(TeammateSystemPromptAddendum, `to: "*"`) {
		t.Error("addendum must mention broadcast syntax")
	}
	if !strings.Contains(TeammateSystemPromptAddendum, "team lead") {
		t.Error("addendum must mention team lead")
	}
}
