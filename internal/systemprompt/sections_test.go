package systemprompt

import (
	"strings"
	"testing"
)

func TestGetIntroSection(t *testing.T) {
	s := GetIntroSection()
	if s == "" {
		t.Fatal("GetIntroSection returned empty string")
	}
	// Must contain "interactive agent" phrasing
	if !strings.Contains(s, "interactive agent") {
		t.Error("intro section missing 'interactive agent'")
	}
	// Must contain the cyber risk warning about security testing
	if !strings.Contains(s, "security testing") {
		t.Error("intro section missing cyber risk instruction about security testing")
	}
	// Must contain URL safety warning
	if !strings.Contains(s, "NEVER generate or guess URLs") {
		t.Error("intro section missing URL safety warning")
	}
}

func TestGetSystemSection(t *testing.T) {
	s := GetSystemSection()
	if s == "" {
		t.Fatal("GetSystemSection returned empty string")
	}
	// Must have the "# System" header
	if !strings.Contains(s, "# System") {
		t.Error("system section missing '# System' header")
	}
	// Must mention tool use (from "outside of tool use" bullet)
	if !strings.Contains(s, "tool use") {
		t.Error("system section missing 'tool use' reference")
	}
	// Must mention system-reminder tags
	if !strings.Contains(s, "system-reminder") {
		t.Error("system section missing 'system-reminder' reference")
	}
	// Must mention hooks
	if !strings.Contains(s, "hooks") {
		t.Error("system section missing hooks reference")
	}
	// Must mention auto-compression
	if !strings.Contains(s, "compress") {
		t.Error("system section missing compression reference")
	}
	// Must mention permission mode
	if !strings.Contains(s, "permission mode") {
		t.Error("system section missing 'permission mode'")
	}
}

func TestGetDoingTasksSection(t *testing.T) {
	s := GetDoingTasksSection()
	if s == "" {
		t.Fatal("GetDoingTasksSection returned empty string")
	}
	if !strings.Contains(s, "# Doing tasks") {
		t.Error("doing tasks section missing header")
	}
	// Must mention software engineering tasks
	if !strings.Contains(s, "software engineering") {
		t.Error("doing tasks section missing 'software engineering'")
	}
	// Must mention security vulnerabilities
	if !strings.Contains(s, "security vulnerabilities") {
		t.Error("doing tasks section missing 'security vulnerabilities'")
	}
	// Must mention not over-engineering
	if !strings.Contains(s, "improvements") {
		t.Error("doing tasks section missing over-engineering guidance")
	}
}

func TestGetActionsSection(t *testing.T) {
	s := GetActionsSection()
	if s == "" {
		t.Fatal("GetActionsSection returned empty string")
	}
	if !strings.Contains(s, "# Executing actions with care") {
		t.Error("actions section missing header")
	}
	// Must mention reversibility
	if !strings.Contains(s, "reversib") {
		t.Error("actions section missing reversibility guidance")
	}
	// Must mention blast radius
	if !strings.Contains(s, "blast radius") {
		t.Error("actions section missing 'blast radius'")
	}
	// Must mention force-pushing
	if !strings.Contains(s, "force-push") {
		t.Error("actions section missing force-push example")
	}
	// Must mention git reset --hard
	if !strings.Contains(s, "git reset --hard") {
		t.Error("actions section missing 'git reset --hard' example")
	}
}

func TestGetUsingToolsSection(t *testing.T) {
	s := GetUsingToolsSection()
	if s == "" {
		t.Fatal("GetUsingToolsSection returned empty string")
	}
	if !strings.Contains(s, "# Using your tools") {
		t.Error("using tools section missing header")
	}
	// Must mention Bash tool guidance
	if !strings.Contains(s, "Bash") {
		t.Error("using tools section missing Bash reference")
	}
	// Must mention dedicated tools
	if !strings.Contains(s, "dedicated tool") {
		t.Error("using tools section missing 'dedicated tool' reference")
	}
	// Must mention parallel tool calls
	if !strings.Contains(s, "parallel") {
		t.Error("using tools section missing parallel execution guidance")
	}
}

func TestGetToneStyleSection(t *testing.T) {
	s := GetToneStyleSection()
	if s == "" {
		t.Fatal("GetToneStyleSection returned empty string")
	}
	if !strings.Contains(s, "# Tone and style") {
		t.Error("tone/style section missing header")
	}
	// Must mention emoji policy
	if !strings.Contains(s, "emoji") {
		t.Error("tone/style section missing emoji policy")
	}
	// Must mention line number format
	if !strings.Contains(s, "file_path:line_number") {
		t.Error("tone/style section missing line number format")
	}
	// Must mention GitHub issue format
	if !strings.Contains(s, "owner/repo#123") {
		t.Error("tone/style section missing GitHub issue format")
	}
	// Must mention colon before tool calls
	if !strings.Contains(s, "colon before tool calls") {
		t.Error("tone/style section missing colon guidance")
	}
}

func TestGetSessionGuidanceSection(t *testing.T) {
	s := GetSessionGuidanceSection()
	if s == "" {
		t.Fatal("GetSessionGuidanceSection returned empty string")
	}
	if !strings.Contains(s, "# Session-specific guidance") {
		t.Error("session guidance section missing header")
	}
	// Must mention AgentTool
	if !strings.Contains(s, "AgentTool") {
		t.Error("session guidance section missing AgentTool reference")
	}
	// Must mention shell ! prefix
	if !strings.Contains(s, "! <command>") || !strings.Contains(s, "!") {
		t.Error("session guidance section missing shell ! prefix guidance")
	}
}

func TestComputeEnvInfo(t *testing.T) {
	cfg := EnvInfoConfig{
		WorkDir:        "/home/user/project",
		IsGitRepo:      true,
		Platform:       "linux",
		Shell:          "zsh",
		OSVersion:      "Linux 6.8.0",
		ModelID:        "claude-opus-4-6",
		KnowledgeCutoff: "May 2025",
	}
	s := ComputeEnvInfo(cfg)
	if s == "" {
		t.Fatal("ComputeEnvInfo returned empty string")
	}
	// Must contain the working directory
	if !strings.Contains(s, "/home/user/project") {
		t.Error("env info missing working directory")
	}
	// Must contain platform
	if !strings.Contains(s, "linux") {
		t.Error("env info missing platform")
	}
	// Must contain shell
	if !strings.Contains(s, "zsh") {
		t.Error("env info missing shell")
	}
	// Must contain OS version
	if !strings.Contains(s, "Linux 6.8.0") {
		t.Error("env info missing OS version")
	}
	// Must contain model ID
	if !strings.Contains(s, "claude-opus-4-6") {
		t.Error("env info missing model ID")
	}
	// Must contain knowledge cutoff
	if !strings.Contains(s, "May 2025") {
		t.Error("env info missing knowledge cutoff")
	}
	// Must contain the # Environment header
	if !strings.Contains(s, "# Environment") {
		t.Error("env info missing '# Environment' header")
	}
	// Must contain git repo info
	if !strings.Contains(s, "git repo") || !strings.Contains(s, "git repository") {
		t.Error("env info missing git repo info")
	}
}

func TestComputeEnvInfoAdditionalDirs(t *testing.T) {
	cfg := EnvInfoConfig{
		WorkDir:                   "/home/user/project",
		IsGitRepo:                 true,
		Platform:                  "linux",
		Shell:                     "zsh",
		OSVersion:                 "Linux 6.8.0",
		ModelID:                   "claude-opus-4-6",
		KnowledgeCutoff:           "May 2025",
		AdditionalWorkingDirs:     []string{"/home/user/other"},
	}
	s := ComputeEnvInfo(cfg)
	if !strings.Contains(s, "/home/user/other") {
		t.Error("env info missing additional working directory")
	}
}

func TestComputeEnvInfoModelFamily(t *testing.T) {
	cfg := EnvInfoConfig{
		WorkDir:        "/tmp",
		Platform:       "darwin",
		Shell:          "bash",
		OSVersion:      "Darwin 25.3.0",
		ModelID:        "claude-sonnet-4-6",
		KnowledgeCutoff: "August 2025",
	}
	s := ComputeEnvInfo(cfg)
	// Must contain model family info
	if !strings.Contains(s, "Claude 4.5/4.6") {
		t.Error("env info missing model family info")
	}
}
