package cli

// CLIFlags holds all CLI flag values matching the TypeScript main.tsx flags.
type CLIFlags struct {
	Model           string
	PermissionMode  string // "default", "plan", "auto", "bypass"
	Resume          bool
	SessionID       string
	Verbose         bool
	MaxTurns        int
	SystemPrompt    string
	OutputFormat    string // "text", "json", "stream-json"
	AllowedTools    []string
	DisallowedTools []string
	MCPConfig       string
	Prompt          string // positional arg (non-interactive mode)
}
