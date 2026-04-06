---
phase: 16-sdk-platform-enterprise
plan: 06
subsystem: infra
tags: [startup, profiling, self-update, github-api, semver, init-sequence]

# Dependency graph
requires:
  - phase: 16-03
    provides: "MDM settings loading (LoadMDMSettings)"
  - phase: 16-04
    provides: "Platform detection (platform.GetInfo)"
  - phase: 16-05
    provides: "IDE detection framework"
provides:
  - "Ordered startup init sequence (RunInitSequence) with dependency-safe loading"
  - "Startup profiler with nanosecond checkpoint timing"
  - "Self-update command via GitHub releases API with --yes flag"
affects: [app-wiring, cli-commands, deployment]

# Tech tracking
tech-stack:
  added: []
  patterns: [startup-profiler-checkpoint, atomic-binary-replace, semver-comparison]

key-files:
  created:
    - "internal/startup/startup.go"
    - "internal/startup/profiler.go"
    - "internal/startup/startup_test.go"
  modified:
    - "internal/cli/update_cmd.go"
    - "internal/cli/update_cmd_test.go"

key-decisions:
  - "Profiler uses time.Since with monotonic clock for reliable checkpoint timing"
  - "Self-update uses atomic rename (current -> .old, temp -> current) for safe binary replacement"
  - "RunInitSequence returns InitResult struct rather than mutating global state"

patterns-established:
  - "Startup profiler: NewProfiler(enabled) -> Checkpoint(name) -> Report() pattern"
  - "Testable HTTP clients: getLatestReleaseFromURL accepts custom URL, enabling httptest mocking"

requirements-completed: [PLAT-05, PLAT-06]

# Metrics
duration: 5min
completed: 2026-04-06
---

# Phase 16 Plan 06: Startup Sequence + Self-Update Summary

**Ordered init sequence with profiling checkpoints and self-update via GitHub releases API**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-06T09:14:27Z
- **Completed:** 2026-04-06T09:19:26Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Startup init sequence runs config, MDM, env vars, shutdown, platform detection in strict dependency order with profiling
- Profiler records named checkpoints with monotonic nanosecond timing, controllable via CLAUDE_CODE_PROFILE_STARTUP env var
- Self-update checks GitHub releases API, compares semver versions, and downloads/replaces the binary atomically
- Complete TDD coverage: 10 startup tests + 10 update tests (version comparison, mock HTTP, asset matching, CLI integration)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create startup init sequence with profiler** - `0852317` (test: failing), `625e28e` (feat: implementation)
2. **Task 2: Implement self-update via GitHub releases API** - `987d5ee` (test: failing), `faa4708` (feat: implementation)

_Note: TDD tasks have two commits each (test -> feat)_

## Files Created/Modified
- `internal/startup/profiler.go` - Startup profiler with checkpoint recording and formatted report
- `internal/startup/startup.go` - RunInitSequence: ordered init (config -> MDM -> env -> shutdown -> platform -> background)
- `internal/startup/startup_test.go` - 10 tests covering profiler behavior, ordering, idempotency, env var activation
- `internal/cli/update_cmd.go` - Full self-update: GitHub releases fetch, semver compare, platform asset selection, atomic binary replace
- `internal/cli/update_cmd_test.go` - 10 tests covering version comparison, mock HTTP, asset URL matching, CLI --yes flag

## Decisions Made
- Profiler uses Go's monotonic time.Since() for reliable startup timing (no wall-clock skew)
- RunInitSequence returns an InitResult struct with all init products (config, settings, MDM, platform info) rather than setting globals
- Graceful shutdown checkpoint is recorded but actual setup deferred to call site (needs context cancel function)
- Self-update uses same-directory temp file + rename for atomic replacement (avoids cross-filesystem issues)
- isNewerVersion strips v-prefix and pre-release suffixes for clean semver comparison

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- This is the FINAL PLAN of the entire v2.0 milestone
- All 16 phases complete: foundation through SDK/platform/enterprise
- Startup sequence ready to be wired into app.Run() as the structured init entry point
- Self-update command integrated into CLI, ready for first GitHub release

## Self-Check: PASSED

All 5 files verified present. All 4 commits verified in git log.

---
*Phase: 16-sdk-platform-enterprise*
*Completed: 2026-04-06*
