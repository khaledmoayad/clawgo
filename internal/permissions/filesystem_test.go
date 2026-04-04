package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckFileWritePermission_NoGlobsAllows(t *testing.T) {
	result := CheckFileWritePermission("/path/to/file.go", nil, nil)
	assert.Equal(t, Allow, result)
}

func TestCheckFileWritePermission_DenyGlobDenies(t *testing.T) {
	denyGlobs := []string{"**/*.secret", "**/.env"}
	result := CheckFileWritePermission("config/.env", nil, denyGlobs)
	assert.Equal(t, Deny, result)
}

func TestCheckFileWritePermission_AllowGlobAllows(t *testing.T) {
	allowGlobs := []string{"src/**/*.go", "tests/**"}
	result := CheckFileWritePermission("src/main.go", allowGlobs, nil)
	assert.Equal(t, Allow, result)
}

func TestCheckFileWritePermission_AllowGlobDeniesNonMatch(t *testing.T) {
	allowGlobs := []string{"src/**/*.go"}
	result := CheckFileWritePermission("config/settings.json", allowGlobs, nil)
	assert.Equal(t, Deny, result, "should deny when allowGlobs configured but path doesn't match")
}

func TestCheckFileWritePermission_DenyOnlyAllowsNonMatch(t *testing.T) {
	denyGlobs := []string{"**/*.secret"}
	result := CheckFileWritePermission("src/main.go", nil, denyGlobs)
	assert.Equal(t, Allow, result, "should allow when only denyGlobs configured and path doesn't match")
}

func TestCheckFileWritePermission_DenyTakesPrecedence(t *testing.T) {
	allowGlobs := []string{"src/**"}
	denyGlobs := []string{"src/**/*.secret"}
	result := CheckFileWritePermission("src/config.secret", allowGlobs, denyGlobs)
	assert.Equal(t, Deny, result, "deny should take precedence over allow")
}

func TestCheckFileWritePermission_DoublestarPattern(t *testing.T) {
	allowGlobs := []string{"**/internal/**/*.go"}
	result := CheckFileWritePermission("go/internal/classify/bash.go", allowGlobs, nil)
	assert.Equal(t, Allow, result, "should match doublestar patterns")
}

func TestCheckFileWritePermission_EmptyGlobs(t *testing.T) {
	result := CheckFileWritePermission("/any/path.txt", []string{}, []string{})
	assert.Equal(t, Allow, result, "empty slices should behave like nil")
}
