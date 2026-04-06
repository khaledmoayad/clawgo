---
phase: 15-protocols
plan: 05
subsystem: daemon
tags: [cron, scheduler, daemon, fsnotify, jitter, tools, cron-create, cron-delete, cron-list]

requires:
  - phase: 15-protocols
    provides: daemon package (tasks.go, scheduler.go, lock.go) with CronTask types
provides:
  - Fully wired CronCreate/CronDelete/CronList tools using daemon package
  - TS-matching cron tool schemas (prompt-based, not command-based)
  - HumanReadableSchedule() cron expression formatter
  - Session task removal (RemoveSessionTask)
  - Job count limits (MaxCronJobs=50, CountAllTasks)
affects: [daemon, tools, query-loop]

tech-stack:
  added: []
  patterns: [prompt-based cron tasks (never shell commands), session-scoped vs durable task stores]

key-files:
  created:
    - internal/tools/croncreate/croncreate_test.go
    - internal/tools/crondelete/crondelete_test.go
    - internal/tools/cronlist/cronlist_test.go
  modified:
    - internal/daemon/tasks.go
    - internal/daemon/scheduler.go
    - internal/daemon/daemon_test.go
    - internal/tools/croncreate/croncreate.go
    - internal/tools/croncreate/prompt.go
    - internal/tools/crondelete/crondelete.go
    - internal/tools/crondelete/prompt.go
    - internal/tools/cronlist/cronlist.go
    - internal/tools/cronlist/prompt.go

key-decisions:
  - "Cron tools fire prompts into the query loop, never execute shell commands -- matches TS CronTask.prompt design"
  - "CronDelete searches both file-backed and session-scoped stores, removes from whichever contains the ID"
  - "HumanReadableSchedule covers common cron patterns with fallback to raw expression for complex ones"

patterns-established:
  - "Cron tool schema: cron/prompt/recurring/durable (matching TS CronCreateTool input)"
  - "Session task lifecycle: AddCronTask(durable=false) -> RemoveSessionTask(id) -> ClearSessionTasks()"

requirements-completed: []

duration: 6min
completed: 2026-04-06
---

# Phase 15 Plan 05: Daemon Cron Tool Wiring Summary

**Wired CronCreate/CronDelete/CronList tools to daemon scheduler with TS-matching prompt-based schemas, job limits, and human-readable schedule formatting**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-06T08:45:25Z
- **Completed:** 2026-04-06T08:52:19Z
- **Tasks:** 2
- **Files modified:** 12

## Accomplishments
- All three cron tools (CronCreate, CronDelete, CronList) now fully functional using the daemon package
- Tool schemas updated from old name/command pattern to TS-matching cron/prompt/recurring/durable pattern
- CronCreate validates cron expressions, enforces 50-job limit, returns human-readable schedule
- CronDelete handles both file-backed and session-scoped task removal
- CronList aggregates both durable and session-scoped tasks with human-readable descriptions
- Fixed failing IsLoadingGate test that confused missed-task surfacing with loading gate behavior

## Task Commits

Each task was committed atomically:

1. **Task 1: Fix IsLoadingGate test** - `ac18341` (fix)
2. **Task 2: Wire cron tools to daemon package** - `1437e03` (feat)

## Files Created/Modified
- `internal/daemon/tasks.go` - Added HumanReadableSchedule(), RemoveSessionTask(), CountAllTasks(), NowMs(), MaxCronJobs
- `internal/daemon/scheduler.go` - Updated nowMs() -> NowMs() references
- `internal/daemon/daemon_test.go` - Fixed IsLoadingGate test, added tests for new helpers
- `internal/tools/croncreate/croncreate.go` - Full implementation with daemon.AddCronTask, validation, job limits
- `internal/tools/croncreate/prompt.go` - Updated schema: cron/prompt/recurring/durable
- `internal/tools/croncreate/croncreate_test.go` - Tests for success, durable, missing fields, invalid cron, max jobs
- `internal/tools/crondelete/crondelete.go` - Full implementation with dual-store deletion
- `internal/tools/crondelete/prompt.go` - Updated schema: id-based deletion
- `internal/tools/crondelete/crondelete_test.go` - Tests for file-backed, session-scoped, not-found
- `internal/tools/cronlist/cronlist.go` - Full implementation aggregating both stores
- `internal/tools/cronlist/prompt.go` - Updated description
- `internal/tools/cronlist/cronlist_test.go` - Tests for empty, mixed task listing

## Decisions Made
- Cron tools fire prompts into the query loop, never execute shell commands -- exact parity with TS CronTask.prompt design
- CronDelete searches both file-backed and session-scoped stores, removes from whichever contains the ID
- HumanReadableSchedule covers common cron patterns (every N minutes, daily at HH:MM, weekly on day, hourly at :MM, monthly on day N) with fallback to raw expression for complex ones
- Fixed test by using recurring task instead of one-shot -- one-shot tasks correctly bypass loading gate via missed-task detection

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed failing TestCronScheduler_IsLoadingGate test**
- **Found during:** Task 1 (initial test run)
- **Issue:** Test used a non-recurring task created 120s ago. FindMissedTasks detected it as missed and fired it via OnFire during load(initial=true), bypassing the check() loop's IsLoading gate
- **Fix:** Changed test to use a recurring task that properly exercises the loading gate through the tick-based check() loop
- **Files modified:** internal/daemon/daemon_test.go
- **Verification:** Test passes, all 26 daemon tests pass
- **Committed in:** ac18341

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Bug fix was necessary for test correctness. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all three cron tools are fully implemented with real daemon package integration.

## Next Phase Readiness
- Cron tools are production-ready, wired to the daemon scheduler with file watching and jitter
- Ready for integration with the REPL query loop (OnFire callback wiring)
- Ready for feature gating (KAIROS flag) when feature flag system is implemented

## Self-Check: PASSED

---
*Phase: 15-protocols*
*Completed: 2026-04-06*
