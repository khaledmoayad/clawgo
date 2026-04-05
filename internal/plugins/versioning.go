// Package plugins -- semver version parsing and constraint checking.
//
// Supports basic semver parsing (major.minor.patch) and constraint
// evaluation: ^, ~, >=, <=, >, <, and exact match. Constraints follow
// the same semantics as npm/cargo version ranges.
package plugins

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a parsed semantic version.
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a semver version string (e.g., "1.2.3").
// Tolerates a leading "v" prefix (e.g., "v1.2.3").
func ParseVersion(v string) (major, minor, patch int, err error) {
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimSpace(v)

	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 1 || len(parts) > 3 {
		return 0, 0, 0, fmt.Errorf("invalid version format: %q", v)
	}

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version in %q: %w", v, err)
	}

	if len(parts) >= 2 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid minor version in %q: %w", v, err)
		}
	}

	if len(parts) >= 3 {
		// Strip pre-release/build metadata for comparison (e.g., "1.0.0-beta")
		patchStr := parts[2]
		if idx := strings.IndexAny(patchStr, "-+"); idx >= 0 {
			patchStr = patchStr[:idx]
		}
		patch, err = strconv.Atoi(patchStr)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid patch version in %q: %w", v, err)
		}
	}

	return major, minor, patch, nil
}

// CompareVersions returns -1, 0, or 1 comparing version a to version b.
// Returns 0 if either version is unparseable.
func CompareVersions(a, b string) int {
	aMaj, aMin, aPat, aErr := ParseVersion(a)
	bMaj, bMin, bPat, bErr := ParseVersion(b)
	if aErr != nil || bErr != nil {
		return 0
	}

	if aMaj != bMaj {
		if aMaj < bMaj {
			return -1
		}
		return 1
	}
	if aMin != bMin {
		if aMin < bMin {
			return -1
		}
		return 1
	}
	if aPat != bPat {
		if aPat < bPat {
			return -1
		}
		return 1
	}
	return 0
}

// SatisfiesConstraint checks if a version satisfies a constraint string.
// Supported constraint formats:
//   - "^1.2.3"  -- compatible: same major, minor.patch >= constraint
//   - "~1.2.3"  -- patch-only: same major.minor, patch >= constraint
//   - ">=1.2.3" -- greater than or equal
//   - "<=1.2.3" -- less than or equal
//   - ">1.2.3"  -- strictly greater
//   - "<1.2.3"  -- strictly less
//   - "1.2.3"   -- exact match
//
// Returns false if either version or constraint cannot be parsed.
func SatisfiesConstraint(version, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return true
	}

	switch {
	case strings.HasPrefix(constraint, "^"):
		return satisfiesCaret(version, constraint[1:])
	case strings.HasPrefix(constraint, "~"):
		return satisfiesTilde(version, constraint[1:])
	case strings.HasPrefix(constraint, ">="):
		return CompareVersions(version, constraint[2:]) >= 0
	case strings.HasPrefix(constraint, "<="):
		return CompareVersions(version, constraint[2:]) <= 0
	case strings.HasPrefix(constraint, ">"):
		return CompareVersions(version, constraint[1:]) > 0
	case strings.HasPrefix(constraint, "<"):
		return CompareVersions(version, constraint[1:]) < 0
	default:
		// Exact match
		return CompareVersions(version, constraint) == 0
	}
}

// satisfiesCaret implements ^major.minor.patch: same major, >= minor.patch.
// For ^0.x.y, same major.minor, >= patch (npm semantics).
func satisfiesCaret(version, constraintVer string) bool {
	vMaj, vMin, vPat, vErr := ParseVersion(version)
	cMaj, cMin, cPat, cErr := ParseVersion(constraintVer)
	if vErr != nil || cErr != nil {
		return false
	}

	// ^0.0.x -- exact match on all three
	if cMaj == 0 && cMin == 0 {
		return vMaj == 0 && vMin == 0 && vPat == cPat
	}

	// ^0.y.z -- same major (0) and minor, patch >= constraint patch
	if cMaj == 0 {
		return vMaj == 0 && vMin == cMin && vPat >= cPat
	}

	// ^X.Y.Z (X > 0) -- same major, version >= constraint
	if vMaj != cMaj {
		return false
	}
	if vMin != cMin {
		return vMin > cMin
	}
	return vPat >= cPat
}

// satisfiesTilde implements ~major.minor.patch: same major.minor, >= patch.
func satisfiesTilde(version, constraintVer string) bool {
	vMaj, vMin, vPat, vErr := ParseVersion(version)
	cMaj, cMin, cPat, cErr := ParseVersion(constraintVer)
	if vErr != nil || cErr != nil {
		return false
	}

	if vMaj != cMaj || vMin != cMin {
		return false
	}
	return vPat >= cPat
}
