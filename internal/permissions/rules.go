package permissions

// PermissionRuleValue represents the action for a permission rule.
type PermissionRuleValue string

const (
	// RuleAllow means the tool is always allowed.
	RuleAllow PermissionRuleValue = "allow"
	// RuleDeny means the tool is always denied.
	RuleDeny PermissionRuleValue = "deny"
	// RuleAsk means the tool always requires user prompt.
	RuleAsk PermissionRuleValue = "ask"
)

// PermissionRule associates a tool name with a permission action.
type PermissionRule struct {
	ToolName string
	Value    PermissionRuleValue
}

// ToolPermissionRules holds per-tool permission lists from settings.
// These are typically configured in .claude/settings.json.
type ToolPermissionRules struct {
	AlwaysAllow []string // Tools that auto-execute without prompting
	AlwaysDeny  []string // Tools that are blocked
	AlwaysAsk   []string // Tools that always require a prompt
}

// EvaluateToolRules checks tool permission rules in precedence order:
//  1. AlwaysDeny (highest precedence) -> Deny
//  2. AlwaysAllow -> Allow
//  3. AlwaysAsk -> Ask
//  4. No match -> Ask (conservative default)
//
// This is separate from CheckPermission. The query loop will call
// EvaluateToolRules first, then fall through to CheckPermission if
// no explicit rule matches.
func EvaluateToolRules(toolName string, rules *ToolPermissionRules) PermissionResult {
	if rules == nil {
		return Ask
	}

	// 1. Deny takes highest precedence
	for _, t := range rules.AlwaysDeny {
		if t == toolName {
			return Deny
		}
	}

	// 2. Allow
	for _, t := range rules.AlwaysAllow {
		if t == toolName {
			return Allow
		}
	}

	// 3. Ask
	for _, t := range rules.AlwaysAsk {
		if t == toolName {
			return Ask
		}
	}

	// 4. No match -- conservative default
	return Ask
}
