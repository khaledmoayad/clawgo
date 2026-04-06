---
phase: 15-protocols
plan: 03
subsystem: swarm
tags: [agent-id, deterministic-ids, backend-abstraction, teammate-executor, system-prompt, swarm]

requires:
  - phase: 15-protocols
    provides: swarm types and manager (types.go, swarm.go)
provides:
  - Deterministic agent ID system (agentName@teamName format)
  - TeammateExecutor backend abstraction interface
  - System prompt inheritance with default/replace/append modes
  - Teammate system prompt addendum for team communication
  - Team-essential tool permission merging
affects: [15-protocols, sdk-platform]

tech-stack:
  added: []
  patterns: [deterministic IDs for reconnection, backend abstraction for execution modes, system prompt composition with addendum]

key-files:
  created:
    - internal/swarm/agent_id.go
    - internal/swarm/agent_id_test.go
    - internal/swarm/runner.go
    - internal/swarm/runner_test.go
    - internal/swarm/prompt.go
    - internal/swarm/prompt_test.go
  modified:
    - internal/swarm/types.go
    - internal/swarm/swarm.go
    - internal/swarm/swarm_test.go

key-decisions:
  - "Deterministic agent IDs (agentName@teamName) instead of random hex -- enables reconnection, human-readable debugging, and predictable routing"
  - "Renamed permission_sync.go GenerateRequestID conflict to FormatRequestID/ParseAgentRequestID to coexist with permission-specific ID format"
  - "TeammateExecutor as Go interface rather than function table -- idiomatic Go pattern for backend abstraction"
  - "sendNotification uses defer recover() to handle closed channel race with Manager.Close()"

patterns-established:
  - "agentName@teamName deterministic ID format matching TS agentId.ts"
  - "TeammateExecutor interface for backend-agnostic teammate lifecycle"
  - "BuildTeammateSystemPrompt with mode-based composition (default/replace/append)"

requirements-completed: []

duration: 7min
completed: 2026-04-06
---

# Phase 15 Plan 03: Swarm Runner and Agent ID System Summary

**Deterministic agent IDs (agentName@teamName), TeammateExecutor backend abstraction, and system prompt inheritance with default/replace/append modes for swarm teammates**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-06T08:44:26Z
- **Completed:** 2026-04-06T08:51:50Z
- **Tasks:** 4 + 1 bugfix
- **Files created:** 6
- **Files modified:** 3

## Accomplishments

- **agent_id.go**: Deterministic agent ID system porting TS `utils/agentId.ts` -- FormatAgentID, ParseAgentID, SanitizeAgentName, FormatRequestID, ParseAgentRequestID with full round-trip test coverage
- **runner.go**: Backend abstraction layer porting TS `backends/types.ts` -- TeammateExecutor interface (Spawn/SendMessage/Terminate/Kill/IsActive), TeammateSpawnConfig with all TS fields, BackendType enum, SystemPromptMode enum, TeammateSpawnResult/TeammateMessage types
- **prompt.go**: System prompt inheritance porting TS `inProcessRunner.ts` prompt resolution -- BuildTeammateSystemPrompt with default/replace/append modes, TeammateSystemPromptAddendum matching TS `teammatePromptAddendum.ts`, TeamEssentialTools list, MergeToolPermissions helper
- **swarm.go/types.go updates**: Worker struct gains AgentName/TeamName fields, SpawnWorker uses deterministic IDs, duplicate name detection, system prompt uses BuildTeammateSystemPrompt with teammate addendum

## Task Commits

Each task was committed atomically:

1. **Task 1: Deterministic agent ID system** - `5a18d00` (feat)
2. **Task 2: TeammateExecutor backend abstraction** - `a9ec48a` (feat)
3. **Task 3: System prompt inheritance** - `a93dacd` (feat)
4. **Task 4: Migrate Worker IDs and integrate** - `ccc29c9` (feat)
5. **Bugfix: sendNotification closed-channel recovery** - `a6286da` (fix)

## Files Created/Modified

- `internal/swarm/agent_id.go` - FormatAgentID, ParseAgentID, SanitizeAgentName, FormatRequestID, ParseAgentRequestID
- `internal/swarm/agent_id_test.go` - Round-trip, edge case, and format tests
- `internal/swarm/runner.go` - TeammateExecutor interface, BackendType, SystemPromptMode, spawn/message/result types
- `internal/swarm/runner_test.go` - Interface compliance, type constant, and mock executor tests
- `internal/swarm/prompt.go` - BuildTeammateSystemPrompt, TeammateSystemPromptAddendum, TeamEssentialTools, MergeToolPermissions
- `internal/swarm/prompt_test.go` - All prompt modes, agent definitions, tool merging tests
- `internal/swarm/types.go` - Worker struct updated with AgentName/TeamName fields
- `internal/swarm/swarm.go` - SpawnWorker signature updated, deterministic IDs, duplicate detection, prompt integration, sendNotification recovery
- `internal/swarm/swarm_test.go` - Updated for new SpawnWorker signature, added duplicate/empty-name tests

## Decisions Made

- Used deterministic agent IDs (agentName@teamName) matching TypeScript `agentId.ts` rather than keeping random hex IDs -- enables reconnection after crashes, human-readable debugging, and predictable message routing
- Renamed to `FormatRequestID`/`ParseAgentRequestID` to avoid name collision with `permission_sync.go`'s `GenerateRequestID` which uses a different format (`perm-{ts}-{random}`)
- Used Go interface for `TeammateExecutor` rather than function table or struct-with-closures -- idiomatic Go pattern for polymorphic backends
- Added `defer recover()` in `sendNotification` to handle race condition where worker goroutine sends notification after `Manager.Close()` has already closed `notifyCh`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed send-on-closed-channel panic in sendNotification**
- **Found during:** Task 4 verification
- **Issue:** Worker goroutine's deferred panic recovery could race with Manager.Close(), causing a send on a closed notifyCh channel
- **Fix:** Added defer recover() in sendNotification to silently handle the closed channel case
- **Files modified:** internal/swarm/swarm.go
- **Commit:** a6286da

**2. [Rule 3 - Blocking] Resolved GenerateRequestID naming conflict with permission_sync.go**
- **Found during:** Task 1
- **Issue:** permission_sync.go already exported GenerateRequestID() with no args; new agent-level function had same name with different signature
- **Fix:** Renamed to FormatRequestID/ParseAgentRequestID to distinguish from permission-specific ID format
- **Files modified:** internal/swarm/agent_id.go, internal/swarm/agent_id_test.go
- **Commit:** 5a18d00

## Issues Encountered
None beyond the auto-fixed deviations above.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all functions are fully implemented with real logic, not placeholders.

## Next Phase Readiness
- Agent ID system ready for use by AgentTool, SendMessageTool, and all teammate interaction paths
- TeammateExecutor interface ready for InProcessBackend implementation (uses Spawn/SendMessage/Terminate/Kill/IsActive)
- System prompt builder ready for integration with worker query loop and custom agent definitions
- Permission sync (from 15-04) already uses compatible ID formats

## Self-Check: PASSED
