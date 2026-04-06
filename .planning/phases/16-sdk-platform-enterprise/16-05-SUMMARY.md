---
phase: 16-sdk-platform-enterprise
plan: 05
subsystem: platform
tags: [deeplink, uri-parser, ide-detection, cursor, windsurf, zed, git-attribution, co-authored-by]

# Dependency graph
requires:
  - phase: 16-sdk-platform-enterprise
    provides: "IDE detection base (detect.go), attribution tracker (tracker.go, trailer.go)"
provides:
  - "Deep link URI parser and builder (claude-cli://open protocol)"
  - "Extended IDE detection: Cursor, Windsurf, Zed"
  - "Entrypoint() function for IDE context"
  - "FormatCommitMessage() with Co-Authored-By trailer"
affects: [cli-entrypoint, telemetry, git-integration]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Deep link URI parsing with net/url and security validation"
    - "IDE fork detection ordering (Cursor/Windsurf before VS Code)"

key-files:
  created:
    - internal/deeplink/deeplink.go
    - internal/deeplink/deeplink_test.go
  modified:
    - internal/ide/detect.go
    - internal/ide/detect_test.go
    - internal/attribution/trailer.go
    - internal/attribution/attribution_test.go

key-decisions:
  - "Check Cursor/Windsurf env vars before VS Code since they are forks and may set VSCODE_PID"
  - "Strip hidden Unicode (zero-width spaces, BOM, tag chars) from deep link queries for security"
  - "Entrypoint() returns string values matching TS entrypoint context strings"

patterns-established:
  - "Deep link URI protocol: claude-cli://open with q, cwd, repo query params"
  - "IDE fork detection ordering: check specific forks before generic parent IDE"

requirements-completed: [PLAT-07, PLAT-08, PLAT-09]

# Metrics
duration: 4min
completed: 2026-04-06
---

# Phase 16 Plan 05: Deep Links, IDE Detection, and Attribution Summary

**Deep link URI parser with security validation, IDE detection for Cursor/Windsurf/Zed, and FormatCommitMessage with Co-Authored-By trailer**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-06T09:05:46Z
- **Completed:** 2026-04-06T09:09:54Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Complete deep link parser handling claude-cli://open URIs with security validation (control chars, oversized values, hidden Unicode, invalid paths/slugs)
- IDE detection expanded to cover Cursor, Windsurf, and Zed via env vars and process scanning, with fork-aware priority ordering
- FormatCommitMessage appends Co-Authored-By trailer when AI-modified files exist, with duplicate prevention

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement deep link URI parser and builder (TDD RED)** - `2761e6a` (test)
2. **Task 1: Implement deep link URI parser and builder (TDD GREEN)** - `cb5e93b` (feat)
3. **Task 2: Expand IDE detection and enhance commit attribution** - `3f86985` (feat)

_Note: Task 1 used TDD flow with separate test and implementation commits._

## Files Created/Modified
- `internal/deeplink/deeplink.go` - Deep link URI parser and builder with ParseDeepLink/BuildDeepLink
- `internal/deeplink/deeplink_test.go` - 28 tests covering parsing, building, round-trip, security edge cases
- `internal/ide/detect.go` - Added Cursor, Windsurf, Zed IDE types with env var and process detection; added Entrypoint() function
- `internal/ide/detect_test.go` - Added 14 new tests for Cursor, Windsurf, Zed detection and Entrypoint()
- `internal/attribution/trailer.go` - Added FormatCommitMessage() with duplicate-trailer prevention
- `internal/attribution/attribution_test.go` - Added 4 tests for FormatCommitMessage

## Decisions Made
- Check Cursor/Windsurf env vars before VS Code in detectFromEnv() and detectFromProcesses() since they are VS Code forks and may also set VSCODE_PID
- Strip hidden Unicode characters (U+200B-U+200F, U+2028-U+2029, U+FEFF, tag characters U+E0000-U+E007F) from deep link queries to prevent injection
- Repo slug validation uses `^[\w.\-]+/[\w.\-]+$` regex matching TS implementation
- CWD validation supports both Unix (/) and Windows (C:\) absolute paths

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Deep link parser ready for CLI --deep-link-origin flag integration
- IDE detection ready for telemetry entrypoint reporting
- FormatCommitMessage ready for git commit interceptor integration in BashTool

## Self-Check: PASSED

All 6 files verified present. All 3 commits verified in git log.

---
*Phase: 16-sdk-platform-enterprise*
*Completed: 2026-04-06*
