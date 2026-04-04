package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateToolRules_AlwaysAllowReturnsAllow(t *testing.T) {
	rules := &ToolPermissionRules{
		AlwaysAllow: []string{"BashTool", "ReadTool"},
	}
	result := EvaluateToolRules("BashTool", rules)
	assert.Equal(t, Allow, result)
}

func TestEvaluateToolRules_AlwaysDenyReturnsDeny(t *testing.T) {
	rules := &ToolPermissionRules{
		AlwaysDeny: []string{"DangerousTool"},
	}
	result := EvaluateToolRules("DangerousTool", rules)
	assert.Equal(t, Deny, result)
}

func TestEvaluateToolRules_AlwaysAskReturnsAsk(t *testing.T) {
	rules := &ToolPermissionRules{
		AlwaysAsk: []string{"WriteTool"},
	}
	result := EvaluateToolRules("WriteTool", rules)
	assert.Equal(t, Ask, result)
}

func TestEvaluateToolRules_NoMatchReturnsAsk(t *testing.T) {
	rules := &ToolPermissionRules{
		AlwaysAllow: []string{"ReadTool"},
		AlwaysDeny:  []string{"DangerousTool"},
	}
	result := EvaluateToolRules("UnknownTool", rules)
	assert.Equal(t, Ask, result)
}

func TestEvaluateToolRules_DenyTakesPrecedenceOverAllow(t *testing.T) {
	rules := &ToolPermissionRules{
		AlwaysAllow: []string{"BashTool"},
		AlwaysDeny:  []string{"BashTool"},
	}
	// Deny should win
	result := EvaluateToolRules("BashTool", rules)
	assert.Equal(t, Deny, result)
}

func TestEvaluateToolRules_NilRulesReturnsAsk(t *testing.T) {
	result := EvaluateToolRules("AnyTool", nil)
	assert.Equal(t, Ask, result)
}

func TestEvaluateToolRules_EmptyRulesReturnsAsk(t *testing.T) {
	rules := &ToolPermissionRules{}
	result := EvaluateToolRules("AnyTool", rules)
	assert.Equal(t, Ask, result)
}
