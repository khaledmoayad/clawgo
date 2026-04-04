package permissions

// PermissionResult indicates the outcome of a permission check.
type PermissionResult int

const (
	// Allow means the tool can execute without user prompt.
	Allow PermissionResult = iota
	// Deny means the tool execution is blocked.
	Deny
	// Ask means the user must be prompted for permission.
	Ask
)

// PermissionContext carries permission state for the current session.
type PermissionContext struct {
	Mode            Mode                 // Permission enforcement level
	AllowedTools    map[string]bool      // Tools explicitly allowed in settings
	DisallowedTools map[string]bool      // Tools explicitly denied in settings
	AlwaysApproved  map[string]bool      // Tools the user approved "always allow" this session
	ToolRules       *ToolPermissionRules // Per-tool rules from settings.json (alwaysAllow/alwaysDeny/alwaysAsk)
	AllowGlobs      []string             // File write allow glob patterns from settings
	DenyGlobs       []string             // File write deny glob patterns from settings
}

// NewPermissionContext creates a PermissionContext from settings.
func NewPermissionContext(mode Mode, allowedTools, disallowedTools []string) *PermissionContext {
	allowed := make(map[string]bool, len(allowedTools))
	for _, t := range allowedTools {
		allowed[t] = true
	}

	disallowed := make(map[string]bool, len(disallowedTools))
	for _, t := range disallowedTools {
		disallowed[t] = true
	}

	return &PermissionContext{
		Mode:            mode,
		AllowedTools:    allowed,
		DisallowedTools: disallowed,
		AlwaysApproved:  make(map[string]bool),
	}
}

// MarkAlwaysApproved records that the user chose "always allow" for a tool.
func (pc *PermissionContext) MarkAlwaysApproved(toolName string) {
	pc.AlwaysApproved[toolName] = true
}

// CheckPermission decides whether a tool should be allowed, denied, or require a prompt.
// Logic mirrors the TypeScript permission check order:
//  1. If tool is in DisallowedTools -> Deny
//  2. If Mode is ModeBypass or ModeAuto -> Allow
//  3. If tool is in AllowedTools or AlwaysApproved -> Allow
//  4. If tool is read-only -> Allow
//  5. Otherwise -> Ask
func CheckPermission(toolName string, isReadOnly bool, ctx *PermissionContext) PermissionResult {
	// 1. Disallowed tools are always denied, regardless of mode
	if ctx.DisallowedTools[toolName] {
		return Deny
	}

	// 2. Bypass and auto modes allow everything (that isn't explicitly disallowed)
	if ctx.Mode == ModeBypass || ctx.Mode == ModeAuto {
		return Allow
	}

	// 3. Explicitly allowed or session-approved tools are allowed
	if ctx.AllowedTools[toolName] || ctx.AlwaysApproved[toolName] {
		return Allow
	}

	// 4. Read-only tools are auto-approved
	if isReadOnly {
		return Allow
	}

	// 5. Everything else requires user prompt
	return Ask
}

// CheckPermissionWithRules checks per-tool rules from settings first,
// then falls through to the standard mode-based CheckPermission.
// This integrates EvaluateToolRules (from alwaysAllow/alwaysDeny/alwaysAsk
// in settings.json) with the existing permission enforcement.
func CheckPermissionWithRules(toolName string, isReadOnly bool, ctx *PermissionContext, toolRules *ToolPermissionRules) PermissionResult {
	// First check per-tool rules from settings
	if toolRules != nil {
		ruleResult := EvaluateToolRules(toolName, toolRules)
		if ruleResult != Ask {
			return ruleResult // AlwaysAllow or AlwaysDeny takes precedence
		}
	}
	// Fall through to standard mode-based check
	return CheckPermission(toolName, isReadOnly, ctx)
}
