---
phase: 16-sdk-platform-enterprise
plan: 03
subsystem: errors, config, enterprise
tags: [error-hierarchy, mdm, policy-limits, drop-in-config, windows-registry]

# Dependency graph
requires:
  - phase: 08-error-config
    provides: base ClawGoError type, ConfigError, APIError, ToolError, PermissionError, SessionError
provides:
  - Complete error hierarchy with 13 types matching TS utils/errors.ts
  - MDM drop-in directory support for Linux (/etc/claude-code/managed-settings.d/)
  - Windows HKCU registry fallback for MDM settings
  - PolicyLimits enforcement helpers (GetMaxTurns, ShouldRequireSandbox, etc.)
affects: [query-loop, tool-execution, sandbox-enforcement, enterprise-policy]

# Tech tracking
tech-stack:
  added: []
  patterns: [messageProvider interface for embedded struct message extraction, testable platform helpers with injectable paths]

key-files:
  created:
    - internal/errors/errors_test.go
  modified:
    - internal/errors/errors.go
    - internal/config/mdm.go
    - internal/config/mdm_test.go
    - internal/enterprise/policylimits.go
    - internal/enterprise/enterprise_test.go

key-decisions:
  - "Used messageProvider interface for ErrorMessage instead of errors.As for embedded struct types (Go's errors.As does not traverse value embeddings)"
  - "Made loadMDMLinux and loadMDMWindows testable by extracting path/value-parameterized helpers (loadMDMLinuxFromPaths, loadMDMWindowsFromValues)"
  - "PolicyLimits helpers encapsulate lock and nil checks, returning permissive defaults when no limits fetched"

patterns-established:
  - "Testable platform helpers: extract OS-dependent code into injected-path functions for unit testing"
  - "Interface-based method dispatch: use small interfaces (messageProvider) to work around Go struct embedding limitations with errors.As"

requirements-completed: [PLAT-01, PLAT-02, PLAT-10]

# Metrics
duration: 7min
completed: 2026-04-06
---

# Phase 16 Plan 03: Error Hierarchy, MDM Drop-In, PolicyLimits Summary

**Complete error hierarchy with 13 TS-parity types, MDM drop-in directory merging on Linux, HKCU fallback on Windows, and PolicyLimits enforcement helpers for query loop turn limits**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-06T09:05:42Z
- **Completed:** 2026-04-06T09:12:19Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments
- Error hierarchy expanded from 6 to 13 types with full TS parity: ShellError (exit code), AbortError (context.Canceled wrapping), FallbackTriggeredError (model fallback), TeleportError, OAuthError, MalformedCommandError, ConfigParseError
- IsAbortError, ErrorMessage, ToError, IsENOENT utility functions matching TS patterns
- MDM on Linux now reads base settings AND merges alphabetically-sorted drop-in files from managed-settings.d/
- MDM on Windows now tries HKLM first, falls back to HKCU (matching TS priority)
- PolicyLimitsManager exposes GetMaxTurns, GetCustomMessage, ShouldRequireSandbox, IsWebSearchAllowed, IsFileWriteAllowed

## Task Commits

Each task was committed atomically:

1. **Task 1: Complete error hierarchy with TS-parity error types** - `4978544` (feat)
2. **Task 2: Expand MDM paths and add drop-in directory support** - `58f2ba9` (feat)
3. **Task 3: Enhance PolicyLimits with MaxTurns enforcement helper** - `5462ea5` (feat)

## Files Created/Modified
- `internal/errors/errors.go` - Added 7 new error types (ShellError, AbortError, FallbackTriggeredError, TeleportError, OAuthError, MalformedCommandError, ConfigParseError), IsAbortError, ErrorMessage, ToError, IsENOENT utilities, messageProvider interface
- `internal/errors/errors_test.go` - Comprehensive tests for all 13 error types and utility functions
- `internal/config/mdm.go` - Added linuxMDMDropInDir, windowsRegKeyHKCU constants, loadMDMLinuxFromPaths with drop-in directory merging, loadMDMWindowsFromValues with HKCU fallback
- `internal/config/mdm_test.go` - Tests for drop-in merging, alphabetical precedence, invalid JSON handling, HKCU fallback
- `internal/enterprise/policylimits.go` - Added GetMaxTurns, GetCustomMessage, ShouldRequireSandbox, IsWebSearchAllowed, IsFileWriteAllowed
- `internal/enterprise/enterprise_test.go` - Tests for all enforcement helpers including nil-limits (no fetch) cases

## Decisions Made
- Used messageProvider interface for ErrorMessage to work with types that embed ClawGoError by value (Go's errors.As does not traverse value embeddings automatically)
- Extracted testable helpers (loadMDMLinuxFromPaths, loadMDMWindowsFromValues) to avoid OS-level dependencies in unit tests
- Fixed base ClawGoError.Error() to handle empty Message with non-nil Err (was producing double-colon format)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Error() double-colon format with empty Message**
- **Found during:** Task 1 (error hierarchy)
- **Issue:** Wrap("Name", err) produced "Name: : error" when Message was empty
- **Fix:** Added conditional to check Message before including it in format string
- **Files modified:** internal/errors/errors.go
- **Verification:** TestClawGoError_Error/with_wrapped_error passes
- **Committed in:** 4978544 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Bug fix necessary for correctness. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all implementations are complete and wired.

## Next Phase Readiness
- Error hierarchy ready for use throughout codebase (tool errors, API errors, shell errors)
- MDM drop-in support ready for enterprise deployment
- PolicyLimits enforcement helpers ready for query loop integration (GetMaxTurns)
- All packages vet-clean with passing tests

## Self-Check: PASSED

All 7 files verified present. All 3 task commits verified in git history.

---
*Phase: 16-sdk-platform-enterprise*
*Completed: 2026-04-06*
