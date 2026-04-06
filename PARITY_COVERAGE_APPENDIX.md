# Parity Coverage Appendix

Generated: 2026-04-04

This appendix contains machine-derived diff inventories used to maximize parity-gap coverage in CLAURST_PARITY_REPORT.md.

## 1) CLI Flag Diff (Claude Code main.tsx options vs ClawGo root flags)

Method: parse Commander option declarations in claude-code/main.tsx, parse Cobra flag registrations in clawgo/internal/cli/root.go, compute set difference.

- Claude Code flags (parsed): 114
- ClawGo flags (parsed): 12
- Missing in ClawGo: 106

### Missing Flags
--add-dir
--advisor
--afk
--agent
--agent-color
--agent-id
--agent-name
--agent-teams
--agent-type
--agents
--allow-dangerously-skip-permissions
--allowed
--append-system-prompt
--append-system-prompt-file
--assistant
--auth-token
--available
--bare
--betas
--brief
--channels
--chrome
--claudeai
--clear-owner
--client-secret
--console
--cowork
--dangerously-load-development-channels
--dangerously-skip-permissions
--dangerously-skip-permissions-with-classifiers
--debug-file
--deep-link-last-fetch
--deep-link-origin
--deep-link-repo
--delegate-permissions
--disable-slash-commands
--disallowed
--dry-run
--effort
--email
--enable-auth-status
--enable-auto-mode
--fallback-model
--file
--force
--fork-session
--from-pr
--hard-fail
--host
--ide
--idle-timeout
--include-hook-events
--include-partial-messages
--init
--init-only
--input-format
--json
--json-schema
--keep-data
--local
--maintenance
--max-budget-usd
--max-sessions
--max-thinking-tokens
--mcp-debug
--messaging-socket-path
--no-chrome
--no-session-persistence
--output
--owner
--parent-session-id
--pending
--permission-prompt-tool
--plan-mode-required
--plugin-dir
--port
--prefill
--proactive
--rc
--remote
--remote-control
--replay-user-messages
--resume-session-at
--rewind-files
--safe
--scope
--sdk-url
--setting-sources
--settings
--sparse
--sso
--strict-mcp-config
--subject
--system-prompt-file
--task-budget
--tasks
--team-name
--teammate-mode
--teleport
--text
--thinking
--tmux
--tools
--unix
--workload
--workspace

## 2) Command Family Diff (commands/ modules vs internal/commands modules)

Method: list immediate entries under claude-code/commands and clawgo/internal/commands, then compute missing names.

- Claude Code command entries: 101
- ClawGo command entries: 51
- Missing command entries in ClawGo: 58

### Missing Command Entries
add-dir
advisor.ts
ant-trace
autofix-pr
backfill-sessions
break-cache
bridge
bridge-kick.ts
brief.ts
btw
bughunter
chrome
commit-push-pr.ts
commit.ts
createMovedToPluginCommand.ts
ctx_viz
debug-tool-call
desktop
extra-usage
good-claude
heapdump
init-verifiers.ts
init.ts
insights.ts
install-github-app
install-slack-app
install.tsx
issue
mobile
mock-limits
oauth-refresh
onboarding
output-style
passes
perf-issue
pr_comments
privacy-settings
rate-limit-options
release-notes
reload-plugins
remote-env
remote-setup
rename
reset-limits
review.ts
sandbox-toggle
security-review.ts
share
statusline.tsx
stickers
summary
teleport
terminalSetup
thinkback
thinkback-play
ultraplan.tsx
version.ts
voice

## 3) Tool Family Diff (normalized naming)

Method: normalize tool family names by lowercasing and stripping a trailing 'Tool' on Claude side; compare to ClawGo internal/tools names.

- Claude Code normalized tool families: 43
- ClawGo normalized tool families: 45
- Missing normalized families in ClawGo: 13

### Missing Normalized Tool Families
askuserquestion
config
fileedit
fileread
filewrite
mcp
mcpauth
remotetrigger
repl
schedulecron
shared
testing
utils

## Notes

- Some command/tool entries are implementation helpers or naming variants, not one-to-one product features.
- Use this appendix alongside CLAURST_PARITY_REPORT.md for severity/prioritization context.

## 4) Runtime CLI Behavior Matrix (installed claude 2.1.92 vs local clawgo binary)

Method: execute the same command matrix against both binaries with a 20s timeout and compare exit code plus leading output behavior.

- Commands tested: 18
- Clear matches: 5
- Behavioral mismatches: 13

### Full Matrix

| # | Command | Claude EC | ClawGo EC | Result | Observation |
|---|---|---:|---:|---|---|
| 01 | `--help` | 0 | 0 | Match | Both return top-level help successfully. |
| 02 | `mcp --help` | 0 | 0 | Match | Both expose MCP command help. |
| 03 | `mcp serve --help` | 0 | 0 | Match | Both expose MCP server help. |
| 04 | `auth --help` | 0 | 0 | Mismatch | Claude shows auth subcommands; ClawGo falls back to root help (no top-level auth command). |
| 05 | `auth status --json` | 0 | 1 | Mismatch | ClawGo rejects `--json`; Claude supports JSON auth status output. |
| 06 | `--print "hello" --output-format stream-json --input-format text` | 1 | 1 | Mismatch | Claude parses print-mode flags (then enforces additional constraints); ClawGo rejects `--print` as unknown. |
| 07 | `--thinking enabled --help` | 0 | 1 | Mismatch | ClawGo rejects `--thinking` as unknown. |
| 08 | `--max-thinking-tokens 1024 --help` | 0 | 1 | Mismatch | ClawGo rejects `--max-thinking-tokens` as unknown. |
| 09 | `--include-hook-events --help` | 0 | 1 | Mismatch | ClawGo rejects `--include-hook-events` as unknown. |
| 10 | `--include-partial-messages --help` | 0 | 1 | Mismatch | ClawGo rejects `--include-partial-messages` as unknown. |
| 11 | `--fallback-model claude-3-5-haiku-20241022 --help` | 0 | 1 | Mismatch | ClawGo rejects `--fallback-model` as unknown. |
| 12 | `--disable-slash-commands --help` | 0 | 1 | Mismatch | ClawGo rejects `--disable-slash-commands` as unknown. |
| 13 | `--bare --help` | 0 | 1 | Mismatch | ClawGo rejects `--bare` as unknown. |
| 14 | `--continue --help` | 0 | 1 | Mismatch | ClawGo rejects `--continue` as unknown. |
| 15 | `--session-id test-session --help` | 0 | 0 | Match | Both accept the session-id flag in help mode. |
| 16 | `--add-dir /tmp --help` | 0 | 1 | Mismatch | ClawGo rejects `--add-dir` as unknown. |
| 17 | `remote-control --help` | 0 | 0 | Match | Both expose remote-control help (Claude has deeper option surface). |
| 18 | `daemon --help` | 0 | 0 | Mismatch | ClawGo has daemon command help; Claude does not expose a top-level daemon command and shows root help. |

### Runtime Confirmation Highlights

- Runtime validation confirms that several documented static CLI gaps are user-visible command failures, not just unimplemented internals.
- The top-level command tree is not equivalent: ClawGo includes `daemon` while missing Claude's `auth` command family at top-level.
- Print-mode and stream-control automation flags remain a high-severity compatibility gap.
