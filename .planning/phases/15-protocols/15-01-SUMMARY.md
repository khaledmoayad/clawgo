---
phase: 15-protocols
plan: "01"
subsystem: api
tags: [error-classification, cache-break, fingerprint, quota, rate-limit, anthropic-api]

# Dependency graph
requires:
  - phase: 09-query-loop
    provides: query loop with streaming, error handling, and recovery
provides:
  - Comprehensive API error classification with 20+ error types
  - Prompt cache break detection across system, tools, model, betas, effort
  - Request fingerprinting for dedup and diagnostics
  - Quota tracking with unified rate limit header parsing
  - Structured error forwarding to TUI via APIErrorMsg
affects: [16-tui-polish, query-loop, api-client]

# Tech tracking
tech-stack:
  added: []
  patterns: [structured-error-classification, cache-break-detection, request-fingerprinting, quota-tracking]

key-files:
  created:
    - internal/api/cachebreak.go
    - internal/api/cachebreak_test.go
    - internal/api/fingerprint.go
    - internal/api/fingerprint_test.go
    - internal/api/quota.go
    - internal/api/quota_test.go
  modified:
    - internal/api/classify.go
    - internal/api/classify_test.go
    - internal/query/loop.go
    - internal/tui/messages.go

key-decisions:
  - "Used SHA-256 truncated to 16 hex chars for fingerprints (compact yet collision-resistant)"
  - "DJB2 hash for cache break string hashing (matching Claude Code's implementation)"
  - "Beta order normalization in cache break detection to prevent false positives"
  - "Structured APIErrorMsg carries both error info and quota status to TUI"

patterns-established:
  - "Error classification: raw errors -> ClassifyAPIError() -> APIErrorInfo struct -> TUI display"
  - "Cache break detection: CacheBreakDetector tracks request state across turns"
  - "Quota extraction: unified rate limit headers -> QuotaStatus -> GetRateLimitMessage()"

requirements-completed: []

# Metrics
duration: 8min
completed: 2026-04-06
---

# Phase 15 Plan 01: API Protocol Enhancement Summary

**Comprehensive API error classification with 20+ error types, prompt cache break detection, request fingerprinting, and quota tracking with unified rate limit header parsing**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-06T08:43:53Z
- **Completed:** 2026-04-06T08:51:26Z
- **Tasks:** 5
- **Files modified:** 8

## Accomplishments
- Full error taxonomy with ClassifyAPIError() converting raw API errors into structured APIErrorInfo with category, user message, error details, and recovery flag -- covering 20+ error types matching Claude Code
- Prompt cache break detection via CacheBreakDetector tracking system hash, tools hash, model, betas, effort, and cache control across consecutive API calls
- Request fingerprinting producing stable SHA-256 hashes for deduplication logging and diagnostic capture, skipping volatile fields
- Quota tracking parsing all anthropic-ratelimit-unified-* headers with user-facing rate limit messages for 5-hour, 7-day, and 7-day-opus rate limit types
- Query loop integration: all stream errors now classified via ClassifyAPIError() and forwarded to TUI as structured APIErrorMsg

## Task Commits

Each task was committed atomically:

1. **Task 1: Enhanced API Error Classification** - `fca6f45` (fix -- test compilation fix for pre-existing code)
2. **Task 2: Prompt Cache Break Detection** - `c943490` (feat)
3. **Task 3: Request Fingerprinting** - `9607734` (feat)
4. **Task 4: Quota and Rate Limit Display** - `7d0bc35` (feat)
5. **Task 5: Wire Error Classification into Query Loop** - `a7b61ca` (feat)

## Files Created/Modified
- `internal/api/classify.go` - Comprehensive API error classification (20+ error types, user-facing messages)
- `internal/api/classify_test.go` - 20+ tests covering all error categories
- `internal/api/cachebreak.go` - CacheBreakDetector with DJB2 hashing and per-tool schema tracking
- `internal/api/cachebreak_test.go` - 12 tests for all break detection scenarios
- `internal/api/fingerprint.go` - SHA-256 request fingerprinting for dedup and diagnostics
- `internal/api/fingerprint_test.go` - 9 tests for stability, collision resistance, edge cases
- `internal/api/quota.go` - QuotaStatus extraction and rate limit message generation
- `internal/api/quota_test.go` - 17 tests for all rate limit types and overage statuses
- `internal/query/loop.go` - Query loop error handling now uses structured classification
- `internal/tui/messages.go` - Added APIErrorMsg carrying classified error info + quota status

## Decisions Made
- Used SHA-256 truncated to 16 hex chars for request fingerprints (compact yet collision-resistant, matching typical request dedup needs)
- Implemented DJB2 hash for cache break string hashing to match Claude Code's djb2Hash() implementation
- Beta order is normalized (sorted) before comparison in cache break detection to prevent false positives from reordered headers
- APIErrorMsg carries both error info and optional quota status, allowing the TUI to display rate limit context alongside the error message
- Token gap from prompt-too-long errors is extracted and available for future compaction heuristics

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed HTTPError field name in classify tests**
- **Found during:** Task 1 (verification of pre-existing classify tests)
- **Issue:** Tests used `Message` field but HTTPError struct only has `StatusCode` and `Body`
- **Fix:** Changed `Message:` to `Body:` in 8 test cases
- **Files modified:** internal/api/classify_test.go
- **Verification:** All 20+ classify tests compile and pass
- **Committed in:** fca6f45

---

**Total deviations:** 1 auto-fixed (1 bug fix)
**Impact on plan:** Essential fix for test compilation. No scope creep.

## Issues Encountered
None beyond the pre-existing test field name mismatch.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Error classification ready for TUI consumption via APIErrorMsg
- Cache break detection available for query loop diagnostic logging
- Request fingerprinting ready for dedup and diagnostic capture
- Quota tracking ready for rate limit display in status bar

---
*Phase: 15-protocols*
*Completed: 2026-04-06*
