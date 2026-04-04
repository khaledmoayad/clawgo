package permissions

import (
	"github.com/bmatcuk/doublestar/v4"
)

// CheckFileWritePermission checks whether a file write is permitted
// based on configured allow and deny glob patterns.
//
// Logic:
//   - If no globs configured (both nil/empty), return Allow (no restrictions)
//   - Check denyGlobs first: if any match, return Deny
//   - Check allowGlobs: if any match, return Allow
//   - If allowGlobs are configured but none match, return Deny (allowlist mode)
//   - If only denyGlobs are configured and none match, return Allow
func CheckFileWritePermission(filePath string, allowGlobs []string, denyGlobs []string) PermissionResult {
	hasAllowGlobs := len(allowGlobs) > 0
	hasDenyGlobs := len(denyGlobs) > 0

	// No restrictions configured
	if !hasAllowGlobs && !hasDenyGlobs {
		return Allow
	}

	// Check deny globs first (highest precedence)
	for _, pattern := range denyGlobs {
		matched, err := doublestar.Match(pattern, filePath)
		if err == nil && matched {
			return Deny
		}
	}

	// Check allow globs
	if hasAllowGlobs {
		for _, pattern := range allowGlobs {
			matched, err := doublestar.Match(pattern, filePath)
			if err == nil && matched {
				return Allow
			}
		}
		// Allowlist mode: configured but no match -> deny
		return Deny
	}

	// Only deny globs configured and none matched -> allow
	return Allow
}
