package cli

// CLIFlags holds all CLI flag values matching the TypeScript main.tsx flags.
// Grouped by requirement category for clarity.
type CLIFlags struct {
	// Existing core flags
	Model           string   // --model / -m
	PermissionMode  string   // --permission-mode (default, plan, auto, bypass)
	Resume          bool     // --resume / -r
	SessionID       string   // --session-id
	Verbose         bool     // --verbose (no shorthand; -v is NOT mapped here)
	MaxTurns        int      // --max-turns
	SystemPrompt    string   // --system-prompt
	OutputFormat    string   // --output-format (text, json, stream-json)
	AllowedTools    []string // --allowed-tools / --allowedTools
	DisallowedTools []string // --disallowed-tools / --disallowedTools
	MCPConfig       string   // --mcp-config
	Prompt          string   // positional arg (non-interactive mode)

	// CLI-01: Print mode
	Print bool // --print / -p

	// CLI-02: Session flags
	Continue             bool   // --continue / -c
	ForkSession          bool   // --fork-session
	ResumeSessionAt      string // --resume-session-at
	Name                 string // --name / -n
	Prefill              string // --prefill
	FromPR               string // --from-pr
	NoSessionPersistence bool   // --no-session-persistence

	// CLI-03: Model/performance flags
	Effort       string   // --effort (low, medium, high, max)
	Thinking     string   // --thinking (enabled, adaptive, disabled)
	MaxBudgetUSD float64  // --max-budget-usd
	FallbackModel string  // --fallback-model
	Betas        []string // --betas
	TaskBudget   int      // --task-budget

	// CLI-04: Debug flags
	Debug     bool   // --debug / -d
	DebugFile string // --debug-file
	Bare      bool   // --bare

	// CLI-05: Output flags
	JSONSchema             string // --json-schema
	InputFormat            string // --input-format (text, stream-json)
	IncludeHookEvents      bool   // --include-hook-events
	IncludePartialMessages bool   // --include-partial-messages
	ReplayUserMessages     bool   // --replay-user-messages

	// CLI-06: System prompt flags
	SystemPromptFile       string // --system-prompt-file
	AppendSystemPrompt     string // --append-system-prompt
	AppendSystemPromptFile string // --append-system-prompt-file

	// CLI-07: Agent flags
	Agent            string // --agent
	Agents           string // --agents (JSON string)
	AgentID          string // --agent-id
	AgentName        string // --agent-name
	PlanModeRequired bool   // --plan-mode-required
	Proactive        bool   // --proactive
	Brief            bool   // --brief

	// CLI-08: Permission flags
	DangerouslySkipPermissions      bool   // --dangerously-skip-permissions
	AllowDangerouslySkipPermissions bool   // --allow-dangerously-skip-permissions
	PermissionPromptTool            string // --permission-prompt-tool

	// CLI-09: Tool/plugin/settings flags
	Tools                []string // --tools
	StrictMCPConfig      bool     // --strict-mcp-config
	PluginDir            []string // --plugin-dir (repeatable)
	DisableSlashCommands bool     // --disable-slash-commands
	Settings             string   // --settings (file path or JSON string)
	AddDir               []string // --add-dir
	IDE                  bool     // --ide
}
