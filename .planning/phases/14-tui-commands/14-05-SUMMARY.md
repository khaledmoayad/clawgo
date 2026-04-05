---
phase: 14-tui-commands
plan: 05
subsystem: tui
tags: [permission-dialog, toast-notification, bubble-tea, lipgloss, permission-rules]

requires:
  - phase: 14-01
    provides: base TUI model with permission model and styles
provides:
  - "9 specialized permission dialog renderers (bash, file-write, file-edit, filesystem, web-fetch, plan-mode, sandbox, MCP, fallback)"
  - "PermissionDialogType registry mapping tool names to dialog variants"
  - "PermissionRuleListModel for viewing and managing always-allow rules"
  - "NotificationModel with priority queue, fold/merge, invalidation, and auto-dismiss"
  - "Root model wiring: DetailedPermissionRequestMsg, NotificationMsg, ShowPermissionRulesMsg"
affects: [query-loop, tool-execution, settings-persistence]

tech-stack:
  added: []
  patterns: [specialized-dialog-dispatch, notification-priority-queue, fold-merge-pattern]

key-files:
  created:
    - internal/tui/permission_types.go
    - internal/tui/permission_dialogs.go
    - internal/tui/permission_rules.go
    - internal/tui/notification.go
    - internal/tui/permission_dialogs_test.go
    - internal/tui/permission_rules_test.go
    - internal/tui/notification_test.go
    - internal/tui/integration_test.go
  modified:
    - internal/tui/model.go
    - internal/tui/messages.go

key-decisions:
  - "Used type-dispatch (switch on PermissionDialogType) rather than interface polymorphism for dialog rendering -- simpler, no allocation, matches Bubble Tea model patterns"
  - "Notification queue uses priority ordering with FIFO within same priority, matching TS useNotifications behavior"
  - "Permission rules use 4 types (tool, prefix, path, domain) covering all TS PermissionUpdate variants"

patterns-established:
  - "Specialized dialog dispatch: PermissionDialogTypeForTool maps tool names to dialog types, SpecializedPermissionModel.View dispatches to the correct renderer"
  - "Notification fold pattern: RegisterFold(key, func) allows callers to define merge behavior for same-key notifications"
  - "Overlay model pattern: PermissionRuleListModel intercepts all keys when active and is rendered on top of main content"

requirements-completed: [TUI-11, TUI-12]

duration: 12min
completed: 2026-04-05
---

# Phase 14 Plan 05: Specialized Permission Dialogs + Toast Notifications Summary

**9 tool-specific permission dialog renderers with priority-queued toast notification system and permission rule management UI**

## Performance

- **Duration:** 12 min
- **Started:** 2026-04-05T22:01:06Z
- **Completed:** 2026-04-05T22:13:24Z
- **Tasks:** 6
- **Files modified:** 10

## Accomplishments
- 9 specialized permission dialogs that show contextually relevant information per tool type (bash shows command + working dir, file-write shows path + content preview, file-edit shows path + diff preview, etc.)
- Toast notification system with priority queue, fold/merge, invalidation, dedup, and auto-dismiss -- matching the TypeScript useNotifications behavior
- Permission rule management UI for viewing, navigating, and removing always-allow rules
- Full integration into root TUI model with new message types and view rendering
- 59 new tests covering all components

## Task Commits

Each task was committed atomically:

1. **Task 1-2: Permission dialog types and tool-specific renderers** - `248f4a1` (feat)
2. **Task 3: Permission rule management UI** - `00d971c` (feat)
3. **Task 4: Toast notification system** - `8e96000` (feat)
4. **Task 5: Wire into root model** - `f95ed1e` (feat)
5. **Task 6: Comprehensive tests** - `a9c82c0` (test)

## Files Created/Modified
- `internal/tui/permission_types.go` - Permission dialog type registry and PermissionRequestDetails struct
- `internal/tui/permission_dialogs.go` - SpecializedPermissionModel with 9 tool-specific renderers
- `internal/tui/permission_rules.go` - PermissionRuleListModel for rule display and management
- `internal/tui/notification.go` - NotificationModel with priority queue and fold/merge
- `internal/tui/model.go` - Root model updated with new sub-models and message handlers
- `internal/tui/messages.go` - New message types: DetailedPermissionRequestMsg, NotificationMsg, ShowPermissionRulesMsg
- `internal/tui/permission_dialogs_test.go` - 19 tests for dialog types and renderers
- `internal/tui/permission_rules_test.go` - 12 tests for rule list model
- `internal/tui/notification_test.go` - 18 tests for notification system
- `internal/tui/integration_test.go` - 10 integration tests for model wiring

## Decisions Made
- Used type-dispatch (switch on PermissionDialogType) rather than interface polymorphism for dialog rendering -- simpler, no allocation, matches Bubble Tea model patterns
- Notification queue uses priority ordering with FIFO within same priority, matching TS useNotifications behavior
- Permission rules use 4 types (tool, prefix, path, domain) covering all TS PermissionUpdate variants

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- lipgloss v2 Underline() applies per-character ANSI codes which breaks simple string.Contains checks in tests -- resolved by using ANSI-stripping regexp in the web fetch dialog test
- Pre-existing build errors in `internal/tui/renderers/task.go` (function signature mismatch) -- out of scope, not caused by this plan's changes

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Permission dialogs ready for query loop integration -- when the query loop creates PermissionRequestDetails and sends DetailedPermissionRequestMsg, the TUI will render the correct specialized dialog
- Notification system ready for startup notifications, rate limit warnings, update notifications, etc.
- Permission rules UI ready to be wired to settings persistence for add/remove operations

## Self-Check: PASSED

All 8 created files verified present. All 5 task commits verified in git log.

---
*Phase: 14-tui-commands*
*Completed: 2026-04-05*
