package systemprompt

// Temporary stubs for plan 08-01 symbols so the package compiles.
// These will be replaced by the actual implementation from plan 08-01.
// Added by plan 08-02 to unblock parallel execution.

// EnvInfoConfig holds configuration for computing environment info.
type EnvInfoConfig struct {
	WorkDir               string
	IsGitRepo             bool
	Platform              string
	Shell                 string
	OSVersion             string
	ModelID               string
	KnowledgeCutoff       string
	AdditionalWorkingDirs []string
}

// GetIntroSection returns the introduction section of the system prompt.
func GetIntroSection() string { return "" }

// GetSystemSection returns the system section of the system prompt.
func GetSystemSection() string { return "" }

// GetDoingTasksSection returns the doing tasks section.
func GetDoingTasksSection() string { return "" }

// GetActionsSection returns the actions section.
func GetActionsSection() string { return "" }

// GetUsingToolsSection returns the using tools section.
func GetUsingToolsSection() string { return "" }

// GetToneStyleSection returns the tone and style section.
func GetToneStyleSection() string { return "" }

// GetSessionGuidanceSection returns the session guidance section.
func GetSessionGuidanceSection() string { return "" }

// ComputeEnvInfo returns the environment info section.
func ComputeEnvInfo(cfg EnvInfoConfig) string { return "" }
