package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ModeFromString Tests ---

func TestModeFromString_Default(t *testing.T) {
	assert.Equal(t, ModeDefault, ModeFromString("default"))
}

func TestModeFromString_Auto(t *testing.T) {
	assert.Equal(t, ModeAuto, ModeFromString("auto"))
}

func TestModeFromString_Yolo(t *testing.T) {
	assert.Equal(t, ModeAuto, ModeFromString("yolo"))
}

func TestModeFromString_Plan(t *testing.T) {
	assert.Equal(t, ModePlan, ModeFromString("plan"))
}

func TestModeFromString_Bypass(t *testing.T) {
	assert.Equal(t, ModeBypass, ModeFromString("bypass"))
}

func TestModeFromString_Invalid(t *testing.T) {
	assert.Equal(t, ModeDefault, ModeFromString("invalid"))
}

func TestModeString(t *testing.T) {
	assert.Equal(t, "default", ModeDefault.String())
	assert.Equal(t, "plan", ModePlan.String())
	assert.Equal(t, "auto", ModeAuto.String())
	assert.Equal(t, "bypass", ModeBypass.String())
}

// --- CheckPermission Tests ---

func TestCheckPermission_Bypass(t *testing.T) {
	ctx := &PermissionContext{Mode: ModeBypass}
	result := CheckPermission("bash", false, ctx)
	assert.Equal(t, Allow, result)
}

func TestCheckPermission_Auto(t *testing.T) {
	ctx := &PermissionContext{Mode: ModeAuto}
	result := CheckPermission("bash", false, ctx)
	assert.Equal(t, Allow, result)
}

func TestCheckPermission_ReadOnly(t *testing.T) {
	ctx := &PermissionContext{Mode: ModeDefault}
	result := CheckPermission("file_read", true, ctx)
	assert.Equal(t, Allow, result)
}

func TestCheckPermission_WriteDefault(t *testing.T) {
	ctx := &PermissionContext{Mode: ModeDefault}
	result := CheckPermission("bash", false, ctx)
	assert.Equal(t, Ask, result)
}

func TestCheckPermission_AllowedTool(t *testing.T) {
	ctx := &PermissionContext{
		Mode:         ModeDefault,
		AllowedTools: map[string]bool{"bash": true},
	}
	result := CheckPermission("bash", false, ctx)
	assert.Equal(t, Allow, result)
}

func TestCheckPermission_DisallowedTool(t *testing.T) {
	ctx := &PermissionContext{
		Mode:            ModeDefault,
		DisallowedTools: map[string]bool{"bash": true},
	}
	result := CheckPermission("bash", false, ctx)
	assert.Equal(t, Deny, result)
}

func TestCheckPermission_AlwaysApproved(t *testing.T) {
	ctx := &PermissionContext{
		Mode:           ModeDefault,
		AlwaysApproved: map[string]bool{"bash": true},
	}
	result := CheckPermission("bash", false, ctx)
	assert.Equal(t, Allow, result)
}

func TestCheckPermission_DisallowedOverridesAllowed(t *testing.T) {
	ctx := &PermissionContext{
		Mode:            ModeDefault,
		AllowedTools:    map[string]bool{"bash": true},
		DisallowedTools: map[string]bool{"bash": true},
	}
	result := CheckPermission("bash", false, ctx)
	assert.Equal(t, Deny, result)
}

func TestCheckPermission_DisallowedOverridesBypass(t *testing.T) {
	ctx := &PermissionContext{
		Mode:            ModeBypass,
		DisallowedTools: map[string]bool{"bash": true},
	}
	result := CheckPermission("bash", false, ctx)
	assert.Equal(t, Deny, result)
}

func TestCheckPermission_PlanModeWriteTool(t *testing.T) {
	ctx := &PermissionContext{Mode: ModePlan}
	result := CheckPermission("bash", false, ctx)
	assert.Equal(t, Ask, result)
}

func TestCheckPermission_PlanModeReadOnly(t *testing.T) {
	ctx := &PermissionContext{Mode: ModePlan}
	result := CheckPermission("file_read", true, ctx)
	assert.Equal(t, Allow, result)
}

// --- NewPermissionContext Tests ---

func TestNewPermissionContext(t *testing.T) {
	ctx := NewPermissionContext(ModeAuto, []string{"bash", "file_read"}, []string{"dangerous_tool"})

	assert.Equal(t, ModeAuto, ctx.Mode)
	require.NotNil(t, ctx.AllowedTools)
	assert.True(t, ctx.AllowedTools["bash"])
	assert.True(t, ctx.AllowedTools["file_read"])
	assert.False(t, ctx.AllowedTools["other"])
	require.NotNil(t, ctx.DisallowedTools)
	assert.True(t, ctx.DisallowedTools["dangerous_tool"])
	require.NotNil(t, ctx.AlwaysApproved)
	assert.Empty(t, ctx.AlwaysApproved)
}

func TestNewPermissionContext_NilSlices(t *testing.T) {
	ctx := NewPermissionContext(ModeDefault, nil, nil)

	assert.Equal(t, ModeDefault, ctx.Mode)
	assert.NotNil(t, ctx.AllowedTools)
	assert.NotNil(t, ctx.DisallowedTools)
	assert.NotNil(t, ctx.AlwaysApproved)
}

// --- MarkAlwaysApproved Tests ---

func TestMarkAlwaysApproved(t *testing.T) {
	ctx := NewPermissionContext(ModeDefault, nil, nil)
	ctx.MarkAlwaysApproved("bash")

	assert.True(t, ctx.AlwaysApproved["bash"])
	assert.False(t, ctx.AlwaysApproved["other"])
}
