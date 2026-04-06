---
phase: 16-sdk-platform-enterprise
plan: 01
subsystem: sdk
tags: [go, agent-sdk, query-engine, budget-enforcement, events, streaming]

# Dependency graph
requires: []
provides:
  - "Extended QueryEngineConfig with all TS-parity fields (SDK-01)"
  - "Full SDKEvent type vocabulary matching TS SDKMessage union (SDK-02)"
  - "Budget enforcement logic in runLoop (SDK-05)"
  - "StatusCallback integration for compaction notifications"
affects: [16-02, 16-03, 16-04, 16-05, 16-06]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Budget enforcement pattern: check cost after each API response, emit ResultEvent with budget_exceeded stop reason"
    - "System prompt resolution: CustomSystemPrompt overrides, AppendSystemPrompt concatenates"
    - "SDKEvent constructor pattern: typed constructors for each event kind"

key-files:
  created: []
  modified:
    - internal/sdk/engine.go
    - internal/sdk/events.go
    - internal/sdk/engine_test.go
    - internal/sdk/events_test.go

key-decisions:
  - "Event types and constructors added alongside engine config in same commit to unblock compilation"
  - "PermissionMode defaults to ModeAuto (zero value) when not set, matching SDK auto-approve behavior"
  - "Budget check uses >= comparison for MaxBudgetUSD to ensure immediate termination at threshold"

patterns-established:
  - "ResultEvent constructor: captures result text, session ID, cost, turn count, error flag, stop reason"
  - "CompactingEvent/UserMessageEvent constructors for new event vocabulary"

requirements-completed: [SDK-01, SDK-02, SDK-05]

# Metrics
duration: 4min
completed: 2026-04-06
---

# Phase 16 Plan 01: SDK QueryEngine Config and Event Expansion Summary

**Extended QueryEngineConfig with 12 new TS-parity fields, budget enforcement that terminates the loop on cost overage, and 4 new SDKEvent types with constructors**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-06T09:06:03Z
- **Completed:** 2026-04-06T09:10:30Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- QueryEngineConfig now has all fields from TS QueryEngine.ts: CustomSystemPrompt, AppendSystemPrompt, PermissionMode, MaxBudgetUSD, InitialMessages, Verbose, ReplayUserMessages, IncludePartialMsgs, UserSpecifiedModel, FallbackModel, AbortCtx, StatusCallback
- Budget enforcement stops the agentic loop when cumulative cost >= MaxBudgetUSD, emitting a ResultEvent with budget_exceeded stop reason
- SDKEvent vocabulary expanded with EventResult, EventCompacting, EventUserMessage, EventSystemMessage plus constructor functions
- StatusCallback called around micro-compaction for SDK status notifications
- InitialMessages pre-populates conversation history in NewQueryEngine

## Task Commits

Each task was committed atomically:

1. **Task 1: Expand QueryEngineConfig and add budget enforcement**
   - `02c4e33` (test: add failing tests for config expansion and budget enforcement)
   - `4a619bd` (feat: expand QueryEngineConfig with TS-parity fields and budget enforcement)
2. **Task 2: Expand SDKEvent vocabulary to match TS SDKMessage union**
   - `fbc0f23` (test: add comprehensive tests for expanded SDKEvent vocabulary)

_Note: TDD tasks have separate test and implementation commits. Event types were added in Task 1 feat commit to unblock compilation._

## Files Created/Modified
- `internal/sdk/engine.go` - Extended QueryEngineConfig with 12 new fields, budget enforcement in runLoop, system prompt resolution, PermissionMode propagation, StatusCallback around compaction
- `internal/sdk/events.go` - Added EventResult, EventCompacting, EventUserMessage, EventSystemMessage constants; added Result, SessionID, NumTurns, Status, StopReason fields to SDKEvent; added ResultEvent, CompactingEvent, UserMessageEvent constructors
- `internal/sdk/engine_test.go` - 6 new tests: budget enforcement, custom system prompt, append system prompt, permission mode, verbose, initial messages
- `internal/sdk/events_test.go` - 7 new tests: result event, error result, compacting event, user message event, type uniqueness, new type values, extended fields

## Decisions Made
- Event types and constructors were added in the Task 1 implementation commit rather than separately in Task 2, because Task 1's budget enforcement code depends on ResultEvent. This is a natural dependency within the plan.
- PermissionMode uses the permissions.Mode int type (not a string), consistent with the existing codebase permissions package.
- Budget check uses `>=` comparison to ensure the loop terminates as soon as cost reaches or exceeds the threshold, preventing overspend.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added event types in Task 1 instead of Task 2**
- **Found during:** Task 1 (QueryEngineConfig expansion)
- **Issue:** Task 1 budget enforcement code emits EventResult via ResultEvent constructor, but those were planned for Task 2
- **Fix:** Added EventResult/EventCompacting/EventUserMessage/EventSystemMessage constants and ResultEvent/CompactingEvent/UserMessageEvent constructors in the Task 1 implementation commit
- **Files modified:** internal/sdk/events.go
- **Verification:** All tests pass, go vet clean
- **Committed in:** 4a619bd (Task 1 feat commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Necessary to unblock compilation. Task 2 focused on comprehensive test coverage for the already-implemented event vocabulary. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- SDK QueryEngine config and events are fully expanded for TS parity
- Ready for plan 16-02 (SDK session management, resume, etc.)
- All existing SDK tests continue to pass

## Self-Check: PASSED

- All 5 files verified present on disk
- All 3 commit hashes verified in git log
- All 27 SDK tests pass
- go vet clean

---
*Phase: 16-sdk-platform-enterprise*
*Completed: 2026-04-06*
