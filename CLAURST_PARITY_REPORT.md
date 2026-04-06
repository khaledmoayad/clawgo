# ClawGo Parity Report

## Scope

This document captures the current state of parity between:

- [Claude Code TypeScript reference](/home/ubuntu/claude-code)
- [ClawGo Go rewrite](/home/ubuntu/clawgo)
- [Claurst Rust rewrite](/home/ubuntu/clawgo/.external/claurst_scan)

The goal is to identify where ClawGo is not yet a drop-in replacement for Claude Code, and to use Claurst as a second implementation reference for what a more complete non-TypeScript port already does.

This is not a design proposal. It is a source of context for future implementation work.

## Bottom Line

ClawGo is not yet 1:1 with Claude Code. The gaps are not limited to polish. There are still major differences in CLI surface area, command depth, TUI workflows, query-loop behavior, MCP resource support, plugin/auth flows, and background automation.

Claurst is materially ahead of ClawGo in several of those areas. It is also not fully equivalent to Claude Code, but it demonstrates that many of the missing pieces in ClawGo are already feasible in a non-TypeScript rewrite.

## High-Level Assessment

### ClawGo vs Claude Code

ClawGo currently has the core skeleton of a Claude Code clone, but many user-facing workflows are incomplete or simplified:

- CLI flags are much narrower than the reference.
- Several top-level commands and slash commands are stubs.
- The TUI is functional but lacks the richer transcript, history, selector, autocomplete, and overlay flows.
- The query loop is simpler than the reference and misses important recovery behavior.
- MCP resources are stubbed.
- Plugin/auth/hooks flows are not implemented at the same depth.

### Claurst vs ClawGo

Claurst proves that a Rust rewrite can get significantly further than ClawGo already has in a few areas:

- More complete CLI coverage.
- More complete slash-command implementations.
- Real TUI overlays for history search, rewind selection, and typeahead.
- Actual MCP resource list/read support.
- Background memory consolidation and session memory extraction.
- More advanced query-loop features such as fallback switching, max-token recovery, tool-result budgeting, and reactive compaction.

At the same time, Claurst still has explicit stubs or partial features in a few enterprise or experimental areas, so it is not a perfect gold standard either.

## Quantitative Snapshot

These counts are approximate and are useful mainly for scale comparison.

- Claude Code CLI option registrations in [main.tsx](/home/ubuntu/claude-code/main.tsx): 68
- ClawGo root CLI flags in [root.go](/home/ubuntu/clawgo/internal/cli/root.go): 12
- Claurst CLI arg annotations in [main.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/cli/src/main.rs): 33

- Claude Code slash commands in [commands/](/home/ubuntu/claude-code/commands): 86 command directories in the repository tree
- ClawGo registered slash commands in [all.go](/home/ubuntu/clawgo/internal/commands/all/all.go): 47 registrations
- Claurst slash commands in [all_commands()](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L7180): 83 registered commands
- Claurst named top-level commands in [all_named_commands()](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/named_commands.rs#L1023): 13

These counts are not a perfect feature score, but they do show that ClawGo is still the smallest surface area of the three.

## Findings by Area

## 1. CLI Surface Parity

### 1.1 ClawGo has a much narrower flag set than Claude Code

ClawGo currently exposes only a small subset of the reference CLI options. In [root.go](/home/ubuntu/clawgo/internal/cli/root.go#L164), the root command currently registers just:

- `--model`
- `--permission-mode`
- `--resume`
- `--session-id`
- `--verbose`
- `--max-turns`
- `--system-prompt`
- `--output-format`
- `--allowed-tools`
- `--disallowed-tools`
- `--mcp-config`

The TypeScript reference, by contrast, has many more options in [main.tsx](/home/ubuntu/claude-code/main.tsx), including but not limited to:

- `--print`
- `--debug`
- `--debug-file`
- `--bare`
- `--init`
- `--init-only`
- `--output-format`
- `--input-format`
- `--json-schema`
- `--include-hook-events`
- `--include-partial-messages`
- `--max-budget-usd`
- `--thinking`
- `--max-thinking-tokens`
- `--continue`
- `--fork-session`
- `--from-pr`
- `--resume-session-at`
- `--system-prompt-file`
- `--append-system-prompt`
- `--append-system-prompt-file`
- `--effort`
- `--agent`
- `--betas`
- `--fallback-model`
- `--settings`
- `--add-dir`
- `--ide`
- `--strict-mcp-config`
- `--session-id`
- `--agents`
- `--setting-sources`
- `--plugin-dir`
- `--disable-slash-commands`
- `--chrome`
- `--no-chrome`
- `--file`

Claurst sits between the two. It already covers more than ClawGo in [main.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/cli/src/main.rs#L103), including:

- `--dangerously-skip-permissions`
- `--input-format`
- `--effort`
- `--thinking`
- `--continue`
- `--system-prompt-file`
- `--allowed-tools`
- `--disallowed-tools`
- `--disable-slash-commands`
- `--bare`
- `--max-budget-usd`
- `--fallback-model`

Claurst still does not appear to match the full Claude Code CLI surface, but it is clearly farther along than ClawGo.

### 1.2 Non-interactive behavior is too minimal in ClawGo

ClawGo’s non-interactive mode in [noninteractive.go](/home/ubuntu/clawgo/internal/app/noninteractive.go) streams text directly to stdout and prints cost to stderr. It does not yet implement the richer print-mode contract from Claude Code, where output format, input format, hook event inclusion, and partial-message streaming are all explicit parts of the interface.

This matters because print mode is a major automation surface for scripts, CI, and editor integrations.

### 1.3 Top-level subcommands are still incomplete

Claude Code has a wide command tree in [main.tsx](/home/ubuntu/claude-code/main.tsx#L3894), including dedicated top-level commands for MCP, server, auth, plugin management, session utilities, update flows, IDE integration, and more.

ClawGo’s root command in [root.go](/home/ubuntu/clawgo/internal/cli/root.go) currently wires only:

- `mcp serve`
- `remote-control`
- `daemon`
- completion

Claurst adds more top-level named commands than ClawGo. In [named_commands.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/named_commands.rs#L1023), it registers 13 named commands such as `agents`, `branch`, `tag`, `passes`, `ide`, `pr-comments`, `desktop`, `mobile`, `install-github-app`, `remote-setup`, `stickers`, `ultraplan`, and `add-dir`.

That is still not the full Claude Code command set, but it is more mature than ClawGo.

## 2. Slash Commands and Command Depth

### 2.1 ClawGo has placeholder slash commands

Several ClawGo commands are present but not actually implemented:

- [plugin.go](/home/ubuntu/clawgo/internal/commands/plugin/plugin.go#L18) returns “Plugin system available in Phase 6.”
- [mcp.go](/home/ubuntu/clawgo/internal/commands/mcp/mcp.go#L18) returns “MCP management available in Phase 5.”
- [hooks.go](/home/ubuntu/clawgo/internal/commands/hooks/hooks.go#L18) returns “Hooks system available in Phase 6.”
- [login.go](/home/ubuntu/clawgo/internal/commands/login/login.go#L18) returns “OAuth login available in Phase 3.”
- [logout.go](/home/ubuntu/clawgo/internal/commands/logout/logout.go#L18) returns “OAuth logout available in Phase 3.”
- [upgrade.go](/home/ubuntu/clawgo/internal/commands/upgrade/upgrade.go#L26) says upgrade check is not yet implemented.

This is not a surface-level mismatch. It means that users can discover commands that do not yet do the real work Claude Code does.

### 2.2 Claurst already implements these command families

Claurst has actual slash-command implementations for the same families in [commands/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs):

- [PluginCommand](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L1547)
- [LoginCommand](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L2111)
- [LogoutCommand](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L2125)
- [HooksCommand](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L2520)
- [McpCommand](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L2560)

That makes Claurst a useful implementation reference for the exact shape of these workflows.

### 2.3 ClawGo’s slash-command registry is broad but thin

ClawGo registers 47 slash commands in [all.go](/home/ubuntu/clawgo/internal/commands/all/all.go). That looks large at first glance, but many entries are still simplified or placeholder-backed.

Claurst’s command registry is larger and more feature-dense: [all_commands()](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L7180) registers 83 slash commands, and those commands are generally deeper than the ClawGo versions.

This means breadth alone is misleading. ClawGo has command names, but not always command behavior.

## 3. TUI / UX Parity

### 3.1 ClawGo TUI is structurally simpler

ClawGo’s TUI in [model.go](/home/ubuntu/clawgo/internal/tui/model.go#L16) has just four states:

- input
- streaming
- permission
- viewport

That is enough for a basic REPL, but it is missing the richer overlay system that Claude Code uses.

### 3.2 ClawGo lacks transcript-style workflows and message selection

Claude Code has a full message selector and rewind flow in [screens/REPL.tsx](/home/ubuntu/claude-code/screens/REPL.tsx#L1478) and [components/MessageSelector.tsx](/home/ubuntu/claude-code/components/MessageSelector.tsx).

Claurst implements these concepts in [overlays.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/overlays.rs#L947) and [overlays.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/overlays.rs#L1118), including:

- `MessageSelectorOverlay`
- `RewindFlowOverlay`
- history search overlay
- full rewind confirmation flow

ClawGo does not currently have these overlays. Its TUI can display large content in [StateViewport](/home/ubuntu/clawgo/internal/tui/model.go#L19), but it cannot yet do transcript browsing, message restore, or rewind selection.

### 3.3 ClawGo lacks typeahead / suggestion UI

Claude Code’s prompt input has [PromptInputFooterSuggestions.tsx](/home/ubuntu/claude-code/components/PromptInput/PromptInputFooterSuggestions.tsx) and [useTypeahead.tsx](/home/ubuntu/claude-code/hooks/useTypeahead.tsx), which provide live suggestions for commands, files, agents, and other items.

Claurst has a comparable typeahead system in [prompt_input.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/prompt_input.rs#L1064).

ClawGo’s [input.go](/home/ubuntu/clawgo/internal/tui/input.go) is a plain textarea without that suggestion layer.

### 3.4 ClawGo’s permission dialog is simpler

ClawGo has a permission dialog in [permission.go](/home/ubuntu/clawgo/internal/tui/permission.go), but it is limited to approve, deny, and always-approve behavior.

Claude Code has a much more elaborate permission and elicitation experience, and Claurst also provides richer dialogs in [elicitation_dialog.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/elicitation_dialog.rs) and [bypass_permissions_dialog.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/bypass_permissions_dialog.rs).

### 3.5 Session browser and history workflows are much richer elsewhere

Claurst’s [session_browser.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/session_browser.rs) includes session metadata, rename flow, and browse modes. Its [overlays.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/overlays.rs#L534) includes Ctrl+R history search.

ClawGo does not yet have equivalent interactive history-search or session-browser overlays.

## 4. Query Loop / Agentic Engine

### 4.1 ClawGo’s query loop is much simpler than the reference

ClawGo’s [loop.go](/home/ubuntu/clawgo/internal/query/loop.go#L1) does the basics:

- stream model response
- collect the final message
- execute tool uses after the stream ends
- apply auto-compact on a prompt-too-long error
- return on `end_turn`

That is functional, but it misses a lot of behavior that the reference implements.

### 4.2 The TypeScript reference has advanced streaming and recovery logic

Claude Code’s [query.ts](/home/ubuntu/claude-code/query.ts) includes features such as:

- streaming tool execution while the model is still generating
- stop-hook handling
- max-output-token recovery with retry count tracking
- task budget tracking
- fallback model recovery
- context collapse and reactive compact handling
- orphan/tombstone handling for fallback recovery
- abort handling and cleanup

These are not just implementation details. They affect how conversations behave under load, under errors, and during long tool-heavy sessions.

### 4.3 ClawGo has some of the scaffolding, but not all of the wiring

ClawGo does have a fallback model helper in [client.go](/home/ubuntu/clawgo/internal/api/client.go#L40) and reactive compaction helpers in the compact package, but the main loop in [loop.go](/home/ubuntu/clawgo/internal/query/loop.go#L53) does not fully exercise all of those capabilities in the same way the reference does.

This is why ClawGo still feels less resilient in long or error-prone conversations.

### 4.4 Claurst is significantly closer to the reference in this area

Claurst’s query loop in [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/lib.rs) already includes all of the following:

- [max_tokens_recovery_count](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/lib.rs#L421)
- fallback switching with “switching to fallback” logic in [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/lib.rs#L580)
- reactive compact in [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/lib.rs#L718)
- session memory extraction in [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/lib.rs#L842)
- AutoDream triggering in [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/lib.rs#L905)

That means ClawGo is not just behind Claude Code; it is also behind another rewrite in the same category.

## 5. MCP Support

### 5.1 ClawGo MCP resources are stubbed

The most obvious MCP gap in ClawGo is that resource tools are not implemented:

- [listmcpresources.go](/home/ubuntu/clawgo/internal/tools/listmcpresources/listmcpresources.go#L2)
- [readmcpresource.go](/home/ubuntu/clawgo/internal/tools/readmcpresource/readmcpresource.go#L2)

Both return “No MCP servers connected.”

### 5.2 Claurst implements real MCP resource list/read support

Claurst has concrete resource tools in [mcp_resources.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tools/src/mcp_resources.rs#L18) and [mcp_resources.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tools/src/mcp_resources.rs#L84), with backing manager methods in [mcp/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/mcp/src/lib.rs#L1220) and [mcp/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/mcp/src/lib.rs#L1252).

This is a confirmed blocker for ClawGo parity.

### 5.3 MCP management UX is much deeper in the reference and in Claurst

Claude Code exposes rich `mcp` subcommands such as add, remove, list, get, add-json, and more in [main.tsx](/home/ubuntu/claude-code/main.tsx#L3894).

Claurst also has a true MCP manager in [mcp/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/mcp/src/lib.rs) and [connection_manager.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/mcp/src/connection_manager.rs), while ClawGo currently only exposes `mcp serve` at the top level and a placeholder `/mcp` command.

## 6. Plugin / Auth / Hooks

### 6.1 ClawGo placeholder commands are not acceptable for drop-in parity

The current ClawGo command implementations for plugin/auth/hooks are placeholders, not working user flows.

### 6.2 Claurst already implements real versions of these workflows

Claurst’s `/plugin` command in [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L1547) supports list, enable, disable, info, install, and reload.

Its plugin layer also has actual marketplace helpers in [marketplace.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/plugins/src/marketplace.rs#L43).

Its `/login`, `/logout`, `/hooks`, and `/mcp` flows are all non-placeholder command implementations in [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L2111), [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L2125), [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L2520), and [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs#L2560).

### 6.3 ClawGo still needs the real plumbing

ClawGo currently returns phase placeholders in:

- [plugin.go](/home/ubuntu/clawgo/internal/commands/plugin/plugin.go#L18)
- [mcp.go](/home/ubuntu/clawgo/internal/commands/mcp/mcp.go#L18)
- [login.go](/home/ubuntu/clawgo/internal/commands/login/login.go#L18)
- [logout.go](/home/ubuntu/clawgo/internal/commands/logout/logout.go#L18)
- [hooks.go](/home/ubuntu/clawgo/internal/commands/hooks/hooks.go#L18)

That is a major source of user-visible incompleteness.

### 6.4 ClawGo does not currently provide a usable OAuth flow

ClawGo includes OAuth-related scaffolding code in [oauth.go](/home/ubuntu/clawgo/internal/auth/oauth.go#L39) and token management in [tokens.go](/home/ubuntu/clawgo/internal/auth/tokens.go#L44), but this is not wired into a working user-facing login/logout flow.

The current auth command surface is still placeholder behavior:

- [login.go](/home/ubuntu/clawgo/internal/commands/login/login.go#L18) returns a phase placeholder message.
- [logout.go](/home/ubuntu/clawgo/internal/commands/logout/logout.go#L18) returns a phase placeholder message.

This means that, in practical runtime terms, ClawGo does not yet have OAuth parity with Claude Code.

By contrast, Claurst has a wired OAuth flow via [oauth_flow.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/cli/src/oauth_flow.rs#L66) and auth command handlers in [main.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/cli/src/main.rs#L2038).

## 7. Memory, Automation, and Background Systems

### 7.1 ClawGo does not yet have background memory consolidation

Claude Code has memory-related systems such as session memory extraction and auto-dream style consolidation.

Claurst already implements both:

- session memory extraction in [session_memory.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/session_memory.rs)
- background AutoDream consolidation in [auto_dream.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/auto_dream.rs)

The query loop calls both in [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/lib.rs#L842) and [lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/lib.rs#L905).

ClawGo has no equivalent background consolidation path today.

### 7.2 Claurst also has cron/background automation that ClawGo lacks

Claurst includes cron scheduling support in [cron_scheduler.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/cron_scheduler.rs), plus scheduler-backed commands and tools.

ClawGo currently does not have a comparable background automation subsystem.

### 7.3 Claurst’s session-memory and automation features are real runtime paths

These are not speculative modules. They are wired into the query loop and therefore reflect actual runtime behavior.

That makes them valuable as design references for future ClawGo work.

## 8. Claurst Is Better, But Not Perfect

Claurst is not a complete 1:1 clone of Claude Code either. It has explicit stubs and partials in a few areas.

### 8.1 Explicit stubs or partials in Claurst

Examples include:

- remote settings fetching is stubbed in [remote_settings.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/core/src/remote_settings.rs#L34)
- analytics is intentionally no-op in [analytics.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/core/src/analytics.rs#L189)
- context collapse still includes placeholder summarization in [context_collapse.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/core/src/context_collapse.rs#L116)

### 8.2 Claurst plugin marketplace still needs careful interpretation

Claurst has marketplace search/install/update functions in [marketplace.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/plugins/src/marketplace.rs), but plugin command routing still centers on local plugin workflows in [plugins/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/plugins/src/lib.rs#L257).

So Claurst is not an automatic source of truth for every gap. It is simply a stronger implementation reference than ClawGo in several key areas.

### 8.3 Additional ClawGo gaps identified in follow-up validation

Two additional runtime-impacting gaps were identified during the follow-up audit and were not explicit in the earlier report draft:

- Bridge work handling is still placeholder wiring. [bridge.go](/home/ubuntu/clawgo/internal/bridge/bridge.go#L97) explicitly marks child-session work handling as integration TODO, meaning `remote-control` mode is not yet parity-complete with Claude Code bridge behavior.
- IDE server message dispatch is still a TODO. [server.go](/home/ubuntu/clawgo/internal/server/server.go#L145) documents no-op message handling and a dispatch TODO, so extension/server interaction depth is currently below parity expectations.
- LSP tool is currently a stub response, not a real language-server integration. [lsp.go](/home/ubuntu/clawgo/internal/tools/lsp/lsp.go#L2) and [lsp.go](/home/ubuntu/clawgo/internal/tools/lsp/lsp.go#L48).

### 8.4 Additional ClawGo gaps identified in deeper subagent sweep

The deeper sweep found several more concrete parity gaps that were not explicit in earlier sections:

- Session transcript persistence is only partially wired in ClawGo. REPL input persists user entries in [repl.go](/home/ubuntu/clawgo/internal/app/repl.go#L62), but assistant/tool turns are only appended to in-memory state in [loop.go](/home/ubuntu/clawgo/internal/query/loop.go#L91), and non-interactive mode does not persist transcript entries in [noninteractive.go](/home/ubuntu/clawgo/internal/app/noninteractive.go). This leaves resume durability below Claude Code's transcript persistence model in [sessionStorage.ts](/home/ubuntu/claude-code/utils/sessionStorage.ts#L132) and [sessionStorage.ts](/home/ubuntu/claude-code/utils/sessionStorage.ts#L1128).
- Hook infrastructure exists but is not fully wired into runtime lifecycle paths. ClawGo defines events and hook runners in [types.go](/home/ubuntu/clawgo/internal/hooks/types.go#L12) and [hooks.go](/home/ubuntu/clawgo/internal/hooks/hooks.go#L44), but query/tool execution paths do not invoke lifecycle equivalents like stop hooks or permission-denied hooks. Claude Code executes these in [query.ts](/home/ubuntu/claude-code/query.ts#L1267) and [toolExecution.ts](/home/ubuntu/claude-code/services/tools/toolExecution.ts#L1081).
- OAuth refresh code is present but not wired into active auth flows. ClawGo defines `RefreshIfNeeded()` in [tokens.go](/home/ubuntu/clawgo/internal/auth/tokens.go#L84), but it is not called from runtime paths outside auth tests, so refresh-on-use parity is not present yet.
- Swarm coordination tool contracts are significantly thinner than Claude Code in key places. ClawGo `TaskUpdate` and `SendMessage` schemas are minimal in [taskupdate.go](/home/ubuntu/clawgo/internal/tools/taskupdate/taskupdate.go#L14) and [sendmessage.go](/home/ubuntu/clawgo/internal/tools/sendmessage/sendmessage.go#L14), while Claude Code supports richer structured semantics in [TaskUpdateTool.ts](/home/ubuntu/claude-code/tools/TaskUpdateTool/TaskUpdateTool.ts#L33) and [SendMessageTool.ts](/home/ubuntu/claude-code/tools/SendMessageTool/SendMessageTool.ts#L48).
- `CronCreate`, `CronDelete`, and `CronList` are still stubbed in ClawGo and return “Cron scheduling not yet available.” in [croncreate.go](/home/ubuntu/clawgo/internal/tools/croncreate/croncreate.go#L54), [crondelete.go](/home/ubuntu/clawgo/internal/tools/crondelete/crondelete.go#L50), and [cronlist.go](/home/ubuntu/clawgo/internal/tools/cronlist/cronlist.go#L31). Claude Code has fully implemented scheduler tools in [CronCreateTool.ts](/home/ubuntu/claude-code/tools/ScheduleCronTool/CronCreateTool.ts#L56), [CronDeleteTool.ts](/home/ubuntu/claude-code/tools/ScheduleCronTool/CronDeleteTool.ts#L35), and [CronListTool.ts](/home/ubuntu/claude-code/tools/ScheduleCronTool/CronListTool.ts#L37).
- Brief mode in ClawGo is only partially wired: `Brief` sets metadata but leaves runtime wiring as placeholder in [brief.go](/home/ubuntu/clawgo/internal/tools/brief/brief.go#L54). Claude Code’s brief mode is end-to-end gated and integrated in [BriefTool.ts](/home/ubuntu/claude-code/tools/BriefTool/BriefTool.ts#L136), [commands/brief.ts](/home/ubuntu/claude-code/commands/brief.ts), and message filtering logic in [Messages.tsx](/home/ubuntu/claude-code/components/Messages.tsx#L93).

### 8.5 Residual low-priority gaps from exhaustive file sweep

These are lower-priority than the core parity blockers above, but they are still concrete deltas:

- Cloud credential prefetch parity is incomplete: ClawGo defines the prefetch interface but explicitly notes Bedrock/Vertex prefetchers as future work in [prefetch.go](/home/ubuntu/clawgo/internal/auth/prefetch.go#L19).
- Session browse metadata still has placeholder cost wiring (`EstimatedCost`) in [browse.go](/home/ubuntu/clawgo/internal/session/browse.go#L37).
- Vim-mode coverage is partial in ClawGo: visual/search paths are still stubbed in [vim.go](/home/ubuntu/clawgo/internal/tui/keybind/vim.go#L13) and [vim.go](/home/ubuntu/clawgo/internal/tui/keybind/vim.go#L145), while Claude Code has a full vim state machine and transitions in [useVimInput.ts](/home/ubuntu/claude-code/hooks/useVimInput.ts#L34) and [transitions.ts](/home/ubuntu/claude-code/vim/transitions.ts).

### 8.6 Final reconciliation for uncited marker files

A final non-test marker sweep left three files that were not previously cited directly. They are now explicitly reconciled:

- [internal/tools/lsp/prompt.go](/home/ubuntu/clawgo/internal/tools/lsp/prompt.go) is the prompt companion to the already documented LSP stub behavior in [lsp.go](/home/ubuntu/clawgo/internal/tools/lsp/lsp.go#L2).
- [internal/compact/micro.go](/home/ubuntu/clawgo/internal/compact/micro.go) is implemented micro-compaction logic and is not itself a placeholder gap; it is part of existing compaction behavior already discussed in the query-loop section.
- [internal/sandbox/bwrap_stub.go](/home/ubuntu/clawgo/internal/sandbox/bwrap_stub.go) is an intentional non-Linux build-tag fallback and is not a functional gap on Linux; sandbox selection and fallback behavior are defined in [sandbox.go](/home/ubuntu/clawgo/internal/sandbox/sandbox.go#L37).

## 9. Prioritized Gap List for ClawGo

If the goal is a practical drop-in replacement, these are the highest-priority gaps:

### Tier 0 — Foundational (without these, Claude behaves completely differently)

0a. **System prompt construction** — ClawGo sends NO default system prompt. Claude Code sends ~3,000 lines of carefully structured instructions (intro, coding guidance, actions-with-care, tool usage, tone/style, environment info, session guidance). Without this, Claude doesn't know it's a CLI agent, doesn't know the working directory, doesn't know what model it is, doesn't get coding best practices, and doesn't get security guidance. This is the single biggest gap.

0b. **Tool prompt text alignment** — ALL 15 audited tool prompts diverge from Claude Code. Critical: Bash (8 lines vs 370), WebSearch (missing mandatory source citation), Agent (completely different architecture description), TaskCreate/Update (different purpose). Without matching prompts, Claude will use every tool differently.

0c. **API beta headers** — ClawGo sends zero betas on the messages endpoint. Claude Code sends 18+ (thinking, 1M context, web search, effort, fast mode, prompt caching, structured outputs, etc.). Without these, features like extended thinking, web search, and prompt caching literally don't activate.

0d. **Thinking parameter support** — Claude Code passes `thinking: { type: 'adaptive' }` or `thinking: { type: 'enabled', budget_tokens: N }`. ClawGo doesn't send thinking parameters at all. Extended thinking won't work.

0e. **Bash command security** — ClawGo has basic safe/risky/deny classification (~220 lines). Claude Code has 23+ security validators (~700 lines) including heredoc safety, Unicode detection, ZSH injection prevention, obfuscation detection. ClawGo will auto-approve dangerous injection patterns.

0f. **File state tracking** — Claude Code maintains an LRU cache tracking which files have been read, with timestamps and partial-view flags. Enforces "must read before edit." ClawGo has no file state tracking — model can edit files it hasn't read.

0g. **Tool name mismatches** — 4 tools have WRONG NAMES: Brief should be SendUserMessage, SyntheticOutput should be StructuredOutput, ListMcpResources should be ListMcpResourcesTool, ReadMcpResource should be ReadMcpResourceTool. The model will attempt to call tools that don't exist.

0h. **Tool parameter name mismatches** — 6+ tools have wrong parameter names: Config (setting→key), Skill (skill→name), TeamCreate (team_name→name), TaskGet (taskId→task_id), ExitPlanMode (no input→plan_summary required), TeamDelete (no params→name required). Tool calls will fail with validation errors.

0i. **Message normalization** — Claude Code runs a 14-step pipeline before every API call (consecutive user merging, tool_use/tool_result pairing, thinking orphan filtering, empty content placeholders, etc.). ClawGo does ZERO normalization — raw messages go to the API. Will cause API errors on edge cases.

0j. **Session JSONL format incompatibility** — Completely different schemas (flat TranscriptMessage with UUID chain vs `{type,message}` envelope). Different path hashing (sanitizePath vs sha256[:8]). Sessions saved by one cannot be resumed by the other.

0k. **Semantic input validation** — Claude Code uses `semanticBoolean()`/`semanticNumber()` to coerce model-generated string representations. ClawGo has none — `"true"` and `"42"` strings will fail validation for Bash timeout, run_in_background, PowerShell timeout, TaskOutput block/timeout, etc.

### Tier 1 — Core engine (blocks basic usability)

1. **Query loop: streaming tool execution** — execute tools while model streams, not after. Latency-critical.
2. **Query loop: max output tokens recovery** — escalate 4096→16384 and retry up to 3x.
3. **Query loop: tool result budgeting** — enforce per-tool `maxResultSizeChars`, replace oversized results.
4. **Query loop: token budget tracking** — cumulative spending limits with proactive blocking.
5. **Query loop: stop hooks** — user-defined post-completion hooks that can block/modify continuation.
6. **Query loop: context collapse + snip compaction** — deferred compaction and old-tool-result pruning before auto-compact.
7. **Query loop: state machine** — 7 continue-site transitions instead of linear iteration.
8. **Query loop: thinking rule enforcement + signature stripping** — validate thinking placement per API spec.
9. **`--print` mode** — non-interactive output for CI/SDK/editor integration. Essential automation surface.
10. **Session transcript persistence** — persist assistant/tool turns to JSONL, not just in-memory.

### Tier 2 — CLI contract (blocks script/automation compatibility)

11. **CLI flags: session control** — `--continue`, `--resume-session-at`, `--fork-session`, `--name`, `--prefill`.
12. **CLI flags: model/performance** — `--effort`, `--thinking`, `--max-budget-usd`, `--fallback-model`, `--betas`, `--task-budget`.
13. **CLI flags: debug** — `--debug` (with filter), `--debug-file`, `--bare`.
14. **CLI flags: output** — `--json-schema`, `--input-format`, `--include-hook-events`, `--include-partial-messages`.
15. **CLI flags: system prompt** — `--system-prompt-file`, `--append-system-prompt`, `--append-system-prompt-file`.
16. **CLI flags: tools/agents** — `--agent`, `--agents`, `--tools`, `--add-dir`, `--dangerously-skip-permissions`.
17. **CLI flags: plugins/skills** — `--plugin-dir`, `--disable-slash-commands`, `--strict-mcp-config`.
18. **CLI subcommands** — `auth` (login/status/logout), `server`, `plugin`, `update`, `task`, `ssh`, full `mcp` management.

### Tier 3 — Slash commands and tool depth (blocks user workflows)

19. **Replace all stub commands** — `/plugin`, `/login`, `/logout`, `/hooks`, `/mcp`, `/tasks`, `/agents`, `/plan` currently return placeholder text.
20. **33+ missing slash commands** — `/break-cache`, `/chrome`, `/onboarding`, `/share`, `/teleport`, `/summary`, `/sandbox-toggle`, `/voice`, etc.
21. **Bash tool: `run_in_background`** — background task execution with goroutine-based lifecycle, notifications, and auto-backgrounding.
22. **Bash tool: signal handling + sandbox override** — SIGINT/SIGTERM, `dangerouslyDisableSandbox`, sed edit preview.
23. **Edit tool: `replace_all` parameter** — multi-replacement support, diff display post-edit, file history.
24. **Task tools: full schema** — `subject`, `activeForm`, `metadata`, `blocks`/`blockedBy`, `owner`.
25. **Cron tools: real implementation** — replace all three stubs with scheduler-backed behavior.
26. **LSP tool: real implementation** — full language-server integration instead of stub.

### Tier 4 — Infrastructure plumbing (blocks advanced features)

27. **Forked agents** — entire subsystem (CRITICAL). Backbone of background memory processing, confidence rating, prompt suggestion.
28. **Plugin system** — install/uninstall/enable/disable/update, scopes (local/user/project/managed), marketplace, dependency resolver, versioning, policy.
29. **Memory extraction + consolidation** — session memory extraction via forked agents, background consolidation, auto-dream.
30. **Hook lifecycle** — wire all 20+ event types (SessionStart/End, Stop, Notification, Permission, Config, Instruction, Elicitation, async registry).
31. **MCP transports** — SSE, HTTP, WebSocket transports + SDK control transport + channel permissions + resource protocol support.
32. **OAuth wiring** — connect existing scaffolding to login/logout flow, profile fetching, token refresh in runtime request paths.
33. **CLAUDE.md @include directives** — `@path`, `@./relative`, `@~/home` with circular reference prevention.
34. **Settings system** — enterprise/remote tiers, change detection/watchers, schema validation, complete source tracking.
35. **Permissions system** — dangerous pattern detection, classifier support, denial tracking, rule shadowing, mode transitions.
36. **Git/worktree** — slug validation, directory symlinking, worktree lifecycle hooks, git config parsing.

### Tier 5 — TUI/UX (blocks interactive experience parity)

37. **Virtual scrolling** — prevent lag on long sessions (Claude Code: 1,081-line VirtualMessageList).
38. **Status line** — model/cost/context/vim indicator at bottom of screen.
39. **Message selector (Ctrl+K)** — pick messages for restore/summarize/rewind.
40. **History search (Ctrl+R)** — fuzzy search with preview.
41. **Help system** — multi-tab interactive help with searchable commands.
42. **Typeahead/suggestions** — file paths, shell history, commands, @mentions.
43. **Specialized permission dialogs** — per-tool (BashPermissionRequest with preview, FileEditPermissionRequest with diffs).
44. **Toast notifications** — floating notifications with stacking and auto-dismiss.
45. **Ctrl+F search** — transcript search with match highlighting.
46. **Fullscreen toggle (Ctrl+O)** — maximize message area.
47. **40+ specialized message type renderers** — thinking blocks, bash I/O, hooks, plan approval, rate limit, compact boundary.
48. **Image display** — render image attachments with terminal hyperlinks.
49. **Agent/swarm UI** — 30+ components for agent management and status.
50. **MCP management UI** — server list, tool browser, settings, reconnect.
51. **Task/background status UI** — task dialog, progress, shell status.
52. **Cost/token tracking display** — heatmap, graphs, per-model breakdowns.
53. **Scroll keybindings** — j/k line scroll, g/G top/bottom, search anchoring.
54. **Error display components** — specialized error renderers instead of plain text.
55. **Theme system** — 50+ semantic color tokens, shimmer variants, agent colors, theme switching.
56. **Full vim mode** — operators (yank/delete/change), text objects, motions, search highlighting, undo.

### Tier 6 — Remaining parity

57. **Bedrock/Vertex credential prefetch** — startup prefetch implementations.
58. **Session browse metadata** — per-session cost wiring, browse UI.
59. **Bridge work dispatch** — `handleWork` integration with query loop.
60. **IDE server message dispatch** — extension communication depth.
61. **Skills marketplace** — skill discovery, installation, parameters.
62. **Worktree session state** — memory file clearing, CWD caching, plan slug isolation.
63. **Brief mode end-to-end** — tool behavior, runtime gating, rendering semantics.
64. **Daemon top-level command** — match Claude's externally exposed surface (Claude does not expose `daemon` publicly).

## 10. Recommended Working Assumption

Until proven otherwise, treat ClawGo as:

- a proof-of-concept that can make API calls and run basic tools
- **not functional as a Claude Code replacement** — the model receives no system prompt, no environment context, no coding guidance, and no tool usage instructions
- not sending the correct API protocol (no betas, no thinking, no prompt caching)
- not behaviorally correct even for "implemented" tools (Read has no PDF/image/notebook support, Grep missing 9/14 parameters, Write has no staleness protection)
- not secure for bash execution (missing 95% of security validators)
- not yet 1:1 on TUI, command depth, or query loop resilience
- not yet equivalent on MCP/plugin/auth/memory/hooks infrastructure

The deep audit revealed that the gap is much larger than originally assessed. The previous estimate of ~20-25% feature completeness should be revised to **~10-15%** when accounting for system prompt construction (0%), tool prompt text (0%), API protocol (partial), tool behavioral correctness (many "FULL" tools are actually broken), and security (critical gaps).

Claurst shows that these gaps are not theoretical. Several of them are already implemented in another rewrite and can be used as concrete engineering reference points.

## 11. Source Index

### Claude Code reference

- [main.tsx](/home/ubuntu/claude-code/main.tsx)
- [query.ts](/home/ubuntu/claude-code/query.ts)
- [components/MessageSelector.tsx](/home/ubuntu/claude-code/components/MessageSelector.tsx)
- [components/PromptInput/PromptInputFooterSuggestions.tsx](/home/ubuntu/claude-code/components/PromptInput/PromptInputFooterSuggestions.tsx)
- [hooks/useTypeahead.tsx](/home/ubuntu/claude-code/hooks/useTypeahead.tsx)
- [components/HistorySearchDialog.tsx](/home/ubuntu/claude-code/components/HistorySearchDialog.tsx)

### ClawGo

- [internal/cli/root.go](/home/ubuntu/clawgo/internal/cli/root.go)
- [internal/cli/flags.go](/home/ubuntu/clawgo/internal/cli/flags.go)
- [internal/app/noninteractive.go](/home/ubuntu/clawgo/internal/app/noninteractive.go)
- [internal/query/loop.go](/home/ubuntu/clawgo/internal/query/loop.go)
- [internal/tui/model.go](/home/ubuntu/clawgo/internal/tui/model.go)
- [internal/tui/input.go](/home/ubuntu/clawgo/internal/tui/input.go)
- [internal/tui/permission.go](/home/ubuntu/clawgo/internal/tui/permission.go)
- [internal/tools/listmcpresources/listmcpresources.go](/home/ubuntu/clawgo/internal/tools/listmcpresources/listmcpresources.go)
- [internal/tools/readmcpresource/readmcpresource.go](/home/ubuntu/clawgo/internal/tools/readmcpresource/readmcpresource.go)
- [internal/commands/all/all.go](/home/ubuntu/clawgo/internal/commands/all/all.go)
- [internal/commands/plugin/plugin.go](/home/ubuntu/clawgo/internal/commands/plugin/plugin.go)
- [internal/commands/mcp/mcp.go](/home/ubuntu/clawgo/internal/commands/mcp/mcp.go)
- [internal/commands/login/login.go](/home/ubuntu/clawgo/internal/commands/login/login.go)
- [internal/commands/logout/logout.go](/home/ubuntu/clawgo/internal/commands/logout/logout.go)
- [internal/commands/hooks/hooks.go](/home/ubuntu/clawgo/internal/commands/hooks/hooks.go)

### Claurst reference

- [src-rust/crates/cli/src/main.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/cli/src/main.rs)
- [src-rust/crates/commands/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/lib.rs)
- [src-rust/crates/commands/src/named_commands.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/commands/src/named_commands.rs)
- [src-rust/crates/tui/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/lib.rs)
- [src-rust/crates/tui/src/overlays.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/overlays.rs)
- [src-rust/crates/tui/src/prompt_input.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tui/src/prompt_input.rs)
- [src-rust/crates/query/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/lib.rs)
- [src-rust/crates/query/src/auto_dream.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/auto_dream.rs)
- [src-rust/crates/query/src/session_memory.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/query/src/session_memory.rs)
- [src-rust/crates/plugins/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/plugins/src/lib.rs)
- [src-rust/crates/plugins/src/marketplace.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/plugins/src/marketplace.rs)
- [src-rust/crates/tools/src/mcp_resources.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/tools/src/mcp_resources.rs)
- [src-rust/crates/mcp/src/lib.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/mcp/src/lib.rs)
- [src-rust/crates/core/src/remote_settings.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/core/src/remote_settings.rs)
- [src-rust/crates/core/src/analytics.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/core/src/analytics.rs)
- [src-rust/crates/core/src/context_collapse.rs](/home/ubuntu/clawgo/.external/claurst_scan/src-rust/crates/core/src/context_collapse.rs)

## 14. Extended Gap Inventory (Deep Subagent Sweep, 2026-04-04)

The following gaps were identified by a 6-agent parallel audit that went deeper than the original report. Items already covered above are excluded; everything below is net-new.

### 14.1 CLI Flags — Full Missing Enumeration

The original report noted ClawGo has 12 flags vs Claude Code's 68. The full missing flag list (excluding analytics-only):

**Session control:** `--continue`, `--fork-session`, `--no-session-persistence`, `--resume-session-at`, `--rewind-files`, `--name`, `--prefill`, `--from-pr`

**Print/output mode:** `--print` (CRITICAL — non-interactive mode for CI/SDK/editor integration), `--json-schema` (structured output validation), `--input-format`, `--include-hook-events`, `--include-partial-messages`, `--replay-user-messages`

**Debug:** `--debug` (with category filter), `--debug-to-stderr`, `--debug-file`, `--bare`, `--init`, `--init-only`, `--maintenance`

**Model/performance:** `--effort`, `--thinking`, `--max-thinking-tokens`, `--max-budget-usd`, `--task-budget`, `--betas`, `--fallback-model`

**Permissions:** `--dangerously-skip-permissions`, `--allow-dangerously-skip-permissions`, `--permission-prompt-tool`

**Agents/teams:** `--agent`, `--agents`, `--agent-id`, `--agent-name`, `--team-name`, `--agent-color`, `--plan-mode-required`, `--parent-session-id`, `--teammate-mode`, `--agent-type`, `--proactive`, `--brief`, `--assistant`

**System prompt variants:** `--system-prompt-file`, `--append-system-prompt`, `--append-system-prompt-file`

**Tools/plugins/skills:** `--tools`, `--strict-mcp-config`, `--plugin-dir`, `--disable-slash-commands`

**Settings/config:** `--settings` (JSON or file), `--add-dir`, `--setting-sources`, `--ide`, `--chrome`, `--no-chrome`, `--file`

**Worktree:** `--worktree` (`-w`), `--tmux`

**Deep link:** `--deep-link-origin`, `--deep-link-repo`, `--deep-link-last-fetch`

### 14.2 CLI Subcommands — Full Missing Enumeration

Missing top-level subcommands beyond what the report already notes:

- `auth` (login/status/logout family)
- `server` (HTTP session hosting)
- `ssh` (remote execution with auth tunneling)
- `plugin` / `plugins` (install/manage)
- `setup-token` (long-lived token setup)
- `auto-mode` (auto mode config inspector)
- `update` / `upgrade` (update checking + installation)
- `install` (native build installation)
- `task` (task list management — 5 subcommands)
- `agents` (agent listing/management)
- `doctor` (auto-updater health — separate from slash `/doctor`)
- `mcp list/get/add-json/add-from-claude-desktop/remove/reset-project-choices` (all MCP management subcommands)

### 14.3 Query Loop — Detailed Missing Features

**Streaming tool execution:** Claude Code's `StreamingToolExecutor` class (150+ lines) executes tools as they stream in, buffering completed results and emitting progress. ClawGo waits for the full stream to end before executing any tools, adding latency on tool-heavy turns.

**Max output tokens recovery:** Claude Code detects `max_output_tokens` stop reason, escalates from 4096 to 16384, and retries up to 3x with continuation messages. ClawGo has no recovery for this.

**Tool result budgeting:** Claude Code's `applyToolResultBudget()` enforces per-tool `maxResultSizeChars` and replaces oversized results. ClawGo passes all content through untruncated.

**Token budget tracking:** Claude Code's `createBudgetTracker()` and `checkTokenBudget()` implement cumulative token budgets with continuation prompts and proactive blocking. ClawGo has no budget enforcement.

**Context collapse:** Claude Code drains staged context collapses on overflow before falling back to reactive compact. ClawGo jumps straight to reactive compact.

**Snip compaction:** Claude Code prunes old tool results before auto-compact to reduce redundancy. ClawGo compacts entire history.

**Cached microcompact:** Claude Code integrates prompt cache for incremental cache deletion tracking via `apiMicrocompact.ts`. ClawGo has no prompt cache integration.

**Tool use summary generation:** Claude Code generates concise tool summaries post-execution (via Haiku) for context compression. ClawGo has nothing equivalent.

**Memory prefetch during stream:** Claude Code's `startRelevantMemoryPrefetch()` prefetches relevant past memories while the model streams. Not present in ClawGo.

**Thinking rule enforcement:** Claude Code validates 3 complex rules about thinking block placement per API spec. ClawGo passes through without validation.

**Signature block stripping:** Claude Code strips protected thinking signatures before fallback retry. Not present in ClawGo.

**Streaming fallback orphan cleanup:** Claude Code tombstones orphaned messages when fallback occurs mid-stream. ClawGo retries without cleanup.

**Stop failure hooks:** Claude Code fires `executeStopFailureHooks()` on API error. Not present in ClawGo.

**Compaction warning hooks:** Claude Code fires hooks to warn user when approaching compact threshold. Not present in ClawGo.

**Query loop state machine:** Claude Code tracks 15 fields and 7 continue-site transitions (collapse_drain, reactive_compact, max_output_escalate, max_output_recovery, stop_hook, token_budget, etc.). ClawGo has a simple linear iteration counter.

**Media size error recovery:** Claude Code detects oversized images/PDFs via `isWithheldMediaSizeError()` and strips them via reactive compact. Not present in ClawGo.

**Attachment message handling:** Claude Code queues user commands as attachments for current turn response. Not present in ClawGo.

### 14.4 TUI — Full Missing Feature Inventory

Beyond what the report covers (typeahead, history search, message selector, rewind), the deep audit found these additional missing features:

**Virtual scrolling:** Claude Code has `VirtualMessageList.tsx` (1,081 lines) with message height caching, width awareness, search indexing, and incremental warm cache. ClawGo renders all messages at once — will lag on long sessions.

**Status line:** Claude Code has a full bottom-of-screen status bar (400+ lines) with dynamic model/permission/cost display, vim mode indicator, token budget visualization, context window usage, rate limit warnings, session title, and directory tracking. Not present in ClawGo.

**Help system:** Claude Code has multi-tab interactive help (`HelpV2/`) with searchable command list, keybinding reference, and inline shortcut hints. Not present in ClawGo.

**Toast notifications:** Claude Code's `Notifications.tsx` (1,000+ lines) provides floating toast notifications with stacking and auto-dismiss. Not present in ClawGo.

**Ctrl+F search:** Claude Code integrates search with the virtual message list, with match highlighting and navigation. Not present in ClawGo.

**Fullscreen toggle (Ctrl+O):** Claude Code's `FullscreenLayout.tsx` maximizes the message area and hides scroll chrome. Not present in ClawGo.

**Specialized permission dialogs:** Claude Code has 15+ per-tool permission request components (BashPermissionRequest with command preview, FileEditPermissionRequest with side-by-side diffs, etc.). ClawGo has one generic Y/N/A dialog.

**Image display:** Claude Code stores and renders image attachments with terminal hyperlinks. Not present in ClawGo.

**Thinking block display:** Claude Code has collapsible thinking blocks with token counts and redacted-thinking support. ClawGo has SDK event support but no UI.

**Agent/swarm UI:** Claude Code has 30+ agent components (AgentsList, AgentEditor, AgentDetail, CoordinatorAgentStatus, etc.). Not present in ClawGo.

**MCP management UI:** Claude Code has 12+ MCP components (MCPListPanel, MCPToolListView, MCPSettings, MCPReconnect, ElicitationDialog). Not present in ClawGo.

**Task/background status UI:** Claude Code has BackgroundTasksDialog, BackgroundTaskStatus, RemoteSessionProgress, ShellProgress. Not present in ClawGo.

**Memory file UI:** Claude Code has MemoryFileSelector and MemoryUpdateNotification. Not present in ClawGo.

**Cost/token tracking display:** Claude Code's `Stats.tsx` (500+ lines) with daily heatmap, asciichart graphs, per-model breakdowns. Not present in ClawGo.

**Scroll keybindings:** Claude Code supports j/k for line scroll, g/G for top/bottom, Page Up/Down with search anchor management. ClawGo only has basic arrows.

**Error display components:** Claude Code has FallbackToolUseErrorMessage, FallbackToolUseRejectedMessage, UserToolErrorMessage with formatting. ClawGo shows errors as plain text.

**40+ message type components:** Claude Code has specialized renderers for AssistantThinkingMessage, UserBashInputMessage, UserBashOutputMessage, HookProgressMessage, PlanApprovalMessage, RateLimitMessage, CompactBoundaryMessage, etc. ClawGo has 6 basic message types.

### 14.5 Tools — Detailed Schema and Behavior Gaps

**Bash tool:**
- Missing `run_in_background` parameter — cannot run long-running builds/tests/servers as background tasks
- Missing `dangerouslyDisableSandbox` parameter
- Missing `_simulatedSedEdit` — sed edit preview system with user approval
- No auto-backgrounding logic (TS auto-backgrounds commands exceeding 15s budget in assistant mode)
- No signal handling (SIGINT/SIGTERM interception for graceful termination)
- No task output storage for large command output
- No smart truncation of large outputs

**Edit tool:**
- Missing `replace_all` parameter (only single replacement supported)
- No diff display post-edit (TS uses gitDiff module)
- No file history tracking (TS logs edits to .claude/history)

**TaskCreate:**
- Missing `subject` (title field), `activeForm` (spinner status text), `metadata` (arbitrary key-value tags)

**TaskUpdate:**
- Missing `activeForm`, `metadata`, `owner`, `addBlocks`, `addBlockedBy` (task dependency management)

**EnterWorktree:**
- Missing slug validation (prevent path traversal)
- Missing session state management (TS stores worktree session in appState + sessionStorage)
- Missing memory file clearing (TS clears CLAUDE.md caches on worktree entry)
- Missing directory symlinking (TS symlinks node_modules to avoid duplication)

**ExitWorktree:**
- Missing tmux session management
- Missing CWD cache clearing

### 14.6 Infrastructure — Detailed Subsystem Gaps

**Forked agents (CRITICAL — entirely absent):**
Claude Code has an entire forked agent infrastructure (`forkedAgent.ts`) for background tasks: session memory extraction, confidence rating, prompt suggestion. Agents share prompt cache with parent, have output token capping, and aggregate usage tracking. ClawGo has nothing equivalent. This is the backbone of background memory processing.

**CLAUDE.md @include directives:**
Claude Code supports `@path`, `@./relative`, and `@~/home` include directives in CLAUDE.md files, with circular reference prevention and text file extension validation. ClawGo loads CLAUDE.md files but does not process include directives.

**Plugin system depth:**
Beyond the report's note that plugins are stubbed, the full missing surface is: install/uninstall/enable/disable/update operations, local/user/project/managed scopes, marketplace integration (official marketplace from GCS), dependency resolver with reverse dependency detection, plugin versioning and auto-update, policy enforcement and blocklisting, plugin hooks loading, and plugin data directory management.

**Hook event types:**
ClawGo has PreToolUse and PostToolUse. Claude Code has 20+ event types: SessionStart, SessionEnd, Setup, Stop, Notification, Permission, Config, Instruction, Elicitation, plus async hook registry for background execution. The missing events mean hooks cannot fire on session lifecycle, permission decisions, or configuration changes.

**MCP transports:**
ClawGo only has stdio transport. Claude Code additionally supports SSE (Server-Sent Events), HTTP, and WebSocket transports, plus SDK control transport (for Agent SDK), channel permissions and allowlist, and auth token refresh in MCP context.

**Settings system:**
ClawGo is missing: settings change detection and watchers, settings validation with schema enforcement, complete source tracking (user/project/enterprise/remote/managed), and enterprise settings tier integration.

**Permissions system:**
ClawGo is missing: dangerous bash pattern detection, classifier-based decision support, denial tracking with fallback to prompting, permission rule shadowing detection, and permission mode transitions (next mode recommendations).

**Git/worktree operations:**
ClawGo is missing: worktree slug validation for security (prevent path traversal), directory symlinking (reduce disk usage), worktree lifecycle hooks (executeWorktreeCreateHook/executeWorktreeRemoveHook), git config parsing and caching, and git ref resolution.

**Skills system depth:**
Beyond basic skill loading, Claude Code has skill prompts and suggestions, skill parameters and options, and marketplace integration. ClawGo has basic file loading only.

**Profile fetching:**
Claude Code fetches user profile info from OAuth endpoint (subscription type, rate limit tier, UUID, organization). ClawGo has no profile fetching.

## 12. Machine-Derived Coverage Appendix

For full static diff inventories (flags, command families, and normalized tool-family deltas), see:

- [PARITY_COVERAGE_APPENDIX.md](/home/ubuntu/clawgo/PARITY_COVERAGE_APPENDIX.md)

## 13. Runtime Validation Snapshot

Static source coverage is now paired with a direct runtime command matrix against:

- installed `claude` binary (`2.1.92`)
- local `/home/ubuntu/clawgo/clawgo` binary

Using an 18-command matrix (help/subcommand/flag acceptance probes), only 5 commands were clear matches and 13 were behavioral mismatches.

Highest-signal runtime mismatches:

- `auth --help`: Claude exposes a full `auth` command family; ClawGo falls back to root help (no top-level auth command).
- `auth status --json`: supported in Claude, rejected in ClawGo.
- `--print ... --output-format ... --input-format ...`: Claude parses print-mode contract; ClawGo rejects `--print`.
- `--thinking`, `--max-thinking-tokens`, `--include-hook-events`, `--include-partial-messages`, `--fallback-model`, `--disable-slash-commands`, `--bare`, `--continue`, `--add-dir`: all accepted by Claude help surface and rejected by ClawGo as unknown flags.
- `daemon --help`: ClawGo exposes a top-level daemon command; Claude does not expose this command publicly and falls back to root help.

These runtime results reinforce that the current parity gap is not only implementation depth; the user-facing CLI contract is materially different today. Full matrix details are documented in the runtime section of [PARITY_COVERAGE_APPENDIX.md](/home/ubuntu/clawgo/PARITY_COVERAGE_APPENDIX.md).

## 15. System Prompt Construction (Deep Audit, 2026-04-04)

**ClawGo has 0% parity with Claude Code's system prompt construction.**

ClawGo concatenates the `--system-prompt` CLI flag with CLAUDE.md files — nothing else. Claude Code builds a multi-section system prompt with static/dynamic caching, feature-flag gating, and priority-based resolution. To reimplement it 1:1, read these files in this order:

### 15.1 Implementation Reference — Where to Look

**Entry point:** `constants/prompts.ts` — `getSystemPrompt()`. This is the main function. It returns `string[]` (array of prompt sections). Read this file first.

**Static sections** (assembled in order, cacheable across orgs):
1. `getSimpleIntroSection()` — in `constants/prompts.ts`. Interactive agent intro + cyber risk warning.
2. `getSimpleSystemSection()` — same file. 6 bullets on tool execution, permissions, system tags, hooks, auto-compression.
3. `getSimpleDoingTasksSection()` — same file. Coding guidance (no gold-plating, comments, testing, security). Gated by `outputStyleConfig.keepCodingInstructions`.
4. `getActionsSection()` — same file. Reversibility/blast radius, risky action examples.
5. `getUsingYourToolsSection()` — same file. Bash vs dedicated tools, parallel execution, task management.
6. `getSimpleToneAndStyleSection()` — same file. Emoji policy, line number references, GitHub issue format.

**Dynamic boundary marker:** `__SYSTEM_PROMPT_DYNAMIC_BOUNDARY__` — inserted between static and dynamic sections when `shouldUseGlobalCacheScope()` is true.

**Dynamic sections** (user/session-specific, lazily resolved via `resolveSystemPromptSections()`):
7. `getSessionSpecificGuidanceSection()` — in `constants/prompts.ts`. Agent tool, Explore agent, Skill tool, verification agent guidance.
8. `loadMemoryPrompt()` — in `utils/claudemd.ts`. CLAUDE.md loading (managed → user → project → local).
9. `computeSimpleEnvInfo()` — in `constants/prompts.ts`. Working dir, git status, platform, shell, OS, model ID, knowledge cutoff, Claude family info, fast mode.
10. `getLanguageSection()` — same file. "Always respond in [language]".
11. `getOutputStyleSection()` — same file. Custom output style from settings.
12. `getMcpInstructionsSection()` — same file. **CACHE-BREAKING** (recomputed every turn). MCP server instructions.
13. `getScratchpadInstructions()` — same file. Scratchpad directory guidance.
14. `getFunctionResultClearingSection()` — same file. Tool result lifecycle (CACHED_MICROCOMPACT feature).
15. `SUMMARIZE_TOOL_RESULTS_SECTION` — constant in same file. "Write down important info from tool results."
16. Token budget section — gated by `TOKEN_BUDGET` feature.
17. Brief mode section — gated by `KAIROS`/`KAIROS_BRIEF` feature.

**Effective prompt builder:** `utils/systemPrompt.ts` — `buildEffectiveSystemPrompt()`. Priority: override > coordinator > agent > custom > default > append.

**Query context assembly:** `utils/queryContext.ts` — `fetchSystemPromptParts()`. Wires system prompt + user context + system context together.

**Cache control on system blocks:** `services/api/claude.ts` — `addCacheBreakpoints()`. Applies `cache_control: { type: 'ephemeral' }` markers to system prompt blocks.

**Coordinator/proactive mode:** `constants/prompts.ts` — separate lean prompt path when `PROACTIVE` or `KAIROS` feature active.

**SIMPLE mode:** `constants/prompts.ts` — minimal 3-line prompt when `CLAUDE_CODE_SIMPLE=true`.

### 15.2 What ClawGo Has Today

`internal/app/app.go` lines 209-215: concatenates `params.SystemPrompt` + CLAUDE.md files. No default prompt, no sections, no caching, no feature gating, no effective prompt resolution.

## 16. Tool Prompt Text Parity (Deep Audit, 2026-04-04)

**Zero tools have matching prompt text. All 15 audited tools are DIVERGENT or PARTIAL.**

Tool prompts shape how Claude uses each tool. Differences here cause behavioral divergence even when the tool implementation is correct.

### 16.1 Critical Divergences

**Bash** — ClawGo: 8-line description. Claude Code: ~370 lines covering tool preference guidance, filesystem sandbox documentation, git operations with commit/PR procedures, background task documentation, sleep/polling guidelines, find regex warnings. ClawGo is ~95% shorter.

**WebSearch** — ClawGo: "Performs a web search and returns results." Claude Code: MANDATORY Sources section requirement with specific markdown format, dynamic year injection ("The current month is [currentMonthYear]. You MUST use this year when searching"), domain filtering documentation, US-only availability note. This is a **behavioral blocker** — Claude won't cite sources properly with ClawGo's prompt.

**Agent** — ClawGo: 4-line description about "sub-agent" workers. Claude Code: ~287 lines with fork subagent feature documentation, detailed "Writing the prompt" guidance with examples, worktree isolation, remote isolation, teammate/swarm conditional sections. Completely different tool model.

**TaskCreate/TaskUpdate** — Fundamentally different purpose. Claude Code: task management/tracking (subject, description, activeForm, metadata, blocks/blockedBy, owner). ClawGo: concurrent background task execution (description, type only). Different tool semantics.

**SendMessage** — Claude Code: teammate communication with multiple addressing modes (by name, broadcast "*"), cross-session UDS/bridge support, protocol responses. ClawGo: "Sends a follow-up message to a running worker agent" with only to/message params.

### 16.2 Significant Divergences

**Read** — Missing documentation for PDF support, Jupyter notebooks, image files, screenshots. Missing conditional sections for file format handling.

**Write** — Missing "NEVER create documentation files (*.md) or README files unless explicitly requested" and emoji guidance.

**Edit** — Missing indentation preservation instructions, line number prefix format clarification, compact line prefix format hints. ClawGo supports creating files with empty old_str (TS doesn't document this — semantic difference).

**Grep** — Missing "ALWAYS use Grep for search tasks. NEVER invoke grep or rg as a Bash command" directive, output mode options, multiline support, "Use Agent tool for open-ended searches" guidance, literal braces escaping example.

**WebFetch** — Missing "If an MCP-provided web fetch tool is available, prefer using that tool" directive, cache documentation, redirect handling, GitHub URL special case (use gh CLI), secondary model prompt with content guidelines, 125-char quote limit for non-preapproved domains.

**Glob** — Missing "use Agent tool instead" for open-ended searches.

**EnterWorktree/ExitWorktree** — Different parameter models (ClawGo: branch/path-based; Claude Code: action-based with name). Missing when-to-use guidance.

**NotebookEdit** — Incompatible parameter models. Claude Code: `edit_mode` enum (replace/insert/delete). ClawGo: `command` enum (add_cell/edit_cell/delete_cell/insert_cell). Different API surface.

## 17. Configuration Schema Parity (Deep Audit, 2026-04-04)

**ClawGo handles 2 of ~70 global config fields (3%) and 9 of 100+ settings fields (9%).**

### 17.1 Global Config (`~/.claude/.config.json`)

ClawGo has: `hasCompletedOnboarding`, `primaryApiKey`.

Missing (~68 fields): `numStartups`, `userID`, `theme`, `projects` (per-project config with session metrics, MCP servers, example files, worktree state), `mcpServers`, `autoUpdates`, `verbose`, `preferredNotifChannel`, `editorMode`, `autoCompactEnabled`, `showTurnDuration`, IDE integration fields (autoConnectIde, autoInstallIdeExtension, 5+ IDE state fields), terminal setup fields (shiftEnterKeyBindingInstalled, iTerm2 setup, Apple Terminal), `bypassPermissionsModeAccepted`, `hasAcknowledgedCostThreshold`, `diffTool`, `env`, `tipsHistory`, `oauthAccount`, `memoryUsageCount`, companion fields, survey state, changelog cache, custom API key responses, S1M access caches, and more.

### 17.2 Settings Schema (`settings.json`)

ClawGo has: `model`, `permissionMode`, `customInstructions`, `allowedTools`, `disallowedTools`, `env`, `keyBindings`, `vimMode`, `enabledPlugins`.

Missing (~90+ fields): `apiKeyHelper`, `awsCredentialExport`, `awsAuthRefresh`, `gcpAuthRefresh`, `outputStyle`, `language`, `syntaxHighlightingDisabled`, `attribution` (commit/PR text), `fileSuggestion`, `respectGitignore`, `claudeMdExcludes`, `cleanupPeriodDays`, `plansDirectory`, `autoMemoryEnabled`, `autoMemoryDirectory`, `autoDreamEnabled`, `agent`, `remote.defaultEnvironmentId`, `availableModels`, `modelOverrides`, `alwaysThinkingEnabled`, `effortLevel`, `advisorModel`, `fastMode`, `promptSuggestionEnabled`, full `permissions` schema (allow/deny/ask arrays with tool matchers, defaultMode, disableBypassPermissionsMode, disableAutoMode, additionalDirectories), `allowManagedPermissionRulesOnly`, `enabledPlugins`, `extraKnownMarketplaces`, `strictKnownMarketplaces`, `blockedMarketplaces`, `allowedMcpServers`/`deniedMcpServers` (with serverName/serverCommand/serverUrl matchers), MCP control fields, `hooks` full schema (event matchers, bash/HTTP/agent/prompt hooks), `disableAllHooks`, `allowManagedHooksOnly`, `worktree` config (symlinkDirectories, sparsePaths), `autoUpdatesChannel`, `minimumVersion`, `forceLoginMethod`/`forceLoginOrgUUID`, SSH configs (id/name/host/port/identityFile/startDirectory), `defaultShell`, `sandbox` settings, `otelHeadersHelper`, and more.

### 17.3 Missing Config Files

- **`.claude/.local/settings.json`** — project-local settings (gitignored). Not loaded.
- **`.mcp.json`** (project root) — MCP server configuration. Not parsed. Supports stdio/websocket/sse/http/sdk transport types.
- **`keybindings.json`** — 170+ keybinding actions across 18 contexts. Not loaded.
- **`~/.claude/projects/<hash>/config.json`** — per-project config (metrics, MCP, worktree state). Not loaded.
- **`/etc/claude-code/managed-settings.d/*.json`** — drop-in enterprise settings files (sorted alphabetically). Not loaded.
- **`/etc/claude-code/managed-mcp.json`** — enterprise MCP server allowlist. Not loaded.
- **`~/.claude/agents/`** — custom agent definitions. Not loaded.

### 17.4 Missing Merge Semantics

Claude Code merges settings with: scalar override (higher priority wins), array append/merge (allowedTools, permissions), per-key map merge (env, enabledPlugins). ClawGo does basic struct override without field-level merge semantics.

Missing merge tier: `.claude/.local/settings.json` (between project and flag settings).

### 17.5 Missing Validation

Claude Code validates all settings via full Zod schema with `z.coerce.string()`, regex validation for server names/paths/URLs, cross-field validation via `.refine()`, and `.passthrough()` for unknown field preservation. ClawGo does `json.Unmarshal` with no validation beyond JSON parsing.

## 18. API Protocol Details (Deep Audit, 2026-04-04)

### 18.1 Missing Request Construction

**Thinking parameters** — Claude Code passes `thinking: { type: 'adaptive' }` or `thinking: { type: 'enabled', budget_tokens: N }` based on model support. Supports adaptive thinking (newer models) vs budget-based thinking (older models). Constraint: `max_tokens > thinking.budget_tokens`. Gated by `CLAUDE_CODE_DISABLE_THINKING` and `CLAUDE_CODE_DISABLE_ADAPTIVE_THINKING` env vars. ClawGo: not implemented.

**Beta headers** — Claude Code sends 18+ beta identifiers on every messages request: `claude-code-20250219`, `interleaved-thinking-2025-05-14`, `context-1m-2025-08-07`, `context-management-2025-06-27`, `structured-outputs-2025-12-15`, `web-search-2025-03-05`, `advanced-tool-use-2025-11-20`, `tool-search-tool-2025-10-19`, `effort-2025-11-24`, `task-budgets-2026-03-13`, `prompt-caching-scope-2026-01-05`, `fast-mode-2026-02-01`, `redact-thinking-2026-02-12`, `token-efficient-tools-2026-03-28`, and more. ClawGo: only sends `files-api-2025-04-14,oauth-2025-04-20` on the files endpoint; **zero betas on messages endpoint**.

**Prompt caching / cache control** — Claude Code sets `cache_control: { type: 'ephemeral', ttl?: '1h', scope?: 'global' }` on system blocks. Uses `addCacheBreakpoints()` for static/dynamic boundary. Tracks 1h TTL eligibility via GrowthBook config. ClawGo: no cache control hints in any requests.

**Custom headers** — Claude Code injects: `x-app: 'cli'`, custom `User-Agent`, `X-Claude-Code-Session-Id`, `x-claude-remote-container-id`, `x-claude-remote-session-id`, `x-client-app`, `x-anthropic-additional-protection`, `x-client-request-id` (UUID per request, 1P only). ClawGo: relies on SDK defaults.

**Content block types in requests** — Claude Code handles: text, image (base64/URLs), document (PDF), tool_use, tool_result, thinking, redacted_thinking, advisor_tool_result, connector text. ClawGo: text, thinking, tool_use, tool_result only. Missing: images, documents, redacted_thinking.

### 18.2 Missing Response Processing

**Usage tracking** — Claude Code tracks 7+ usage fields: input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens, server_tool_use (web_search_requests, web_fetch_requests), ephemeral_1h_input_tokens. ClawGo: 4 fields only (input, output, cache_creation, cache_read). Missing: server_tool_use, ephemeral.

**Rate limit header parsing** — Claude Code reads `retry-after`, `anthropic-ratelimit-unified-reset`, `anthropic-ratelimit-unified-overage-disabled-reason` headers. Short retries (<3s) keep fast mode active. ClawGo: no header parsing.

**Error categorization** — Claude Code: 15+ categories with specific handling for media size errors, prompt too long (withheld vs returned), quota errors. ClawGo: 7 basic categories (RateLimit, Overloaded, ServerError, Auth, ClientError, Network, Unknown).

### 18.3 Retry Logic Mismatches

- **Max retries**: Claude Code default 10, ClawGo default 3 — significant difference for resilience
- **Base delay**: Claude Code 500ms, ClawGo 1000ms
- **Max backoff**: Claude Code 5 minutes (for unattended sessions), ClawGo 30 seconds
- **529 handling**: Claude Code tracks consecutive 529s with MAX_529_RETRIES=3 before triggering fallback. ClawGo: no 529-specific logic
- **Retry-after header**: Claude Code honors it. ClawGo: not parsed
- **Stale connection detection**: Claude Code detects ECONNRESET/EPIPE and refreshes client. ClawGo: not implemented

### 18.4 Missing API Features

- **Token counting API** — Claude Code uses it for accurate context window tracking. Not implemented in ClawGo.
- **OAuth token support** — Claude Code supports `Authorization: Bearer {oauth_token}` via `getClaudeAIOAuthTokens()`. ClawGo: API key only.
- **Effort parameter** — Claude Code passes `effort` in request body. Not implemented in ClawGo.
- **Structured outputs** — Claude Code supports `json_schema` parameter for structured output validation. Not implemented in ClawGo.

## 19. Behavioral Correctness of "Implemented" Tools (Deep Audit, 2026-04-04)

**Every tool previously marked "FULL" was found to have significant behavioral gaps.**

### 19.1 Read Tool — CRITICAL

ClawGo: 112 lines, text-only. Claude Code: 1,100+ lines.

Missing: PDF reading with page extraction (`pages` parameter), image file support (JPEG/PNG/GIF/WebP with base64 encoding and token-budget resizing), Jupyter notebook reading (.ipynb cell extraction), file encoding detection and preservation, image compression with token budget limits, partial view flag tracking (blocks edits without explicit read).

### 19.2 Write Tool — CRITICAL

ClawGo: 72 lines. Claude Code: 360+ lines.

Missing: file staleness detection (compares mtime against read timestamp), content change detection (hash comparison on Windows), file history/backup before write, encoding preservation (`utf8` default with detection), git diff computation post-write, LSP server notification (didChange/didSave), CRLF vs LF handling. **The entire validation layer that prevents concurrent edits and file corruption is absent.**

### 19.3 Grep Tool — CRITICAL

ClawGo missing 9 of 14 parameters:

| Parameter | Claude Code | ClawGo |
|---|---|---|
| `output_mode` (content/files_with_matches/count) | Yes | Missing |
| `-B` (before context) | Yes | Missing |
| `-A` (after context) | Yes | Missing |
| `-n` (line numbers, default true) | Yes | Missing |
| `-i` (case insensitive) | Yes | Missing |
| `type` (file type filter) | Yes | Missing |
| `head_limit` (default 250) | `head_limit` | `max_results` (different name) |
| `offset` | Yes | Missing |
| `multiline` (cross-line patterns) | Yes | Missing |
| `glob` | `glob` | `include` (different name) |

### 19.4 WebFetch — SIGNIFICANT

Missing: 15-minute response cache, prompt-based markdown extraction (sends content to secondary model with guidelines), preapproved host list, max markdown length limit. ClawGo has no caching and returns raw converted content.

### 19.5 WebSearch — SIGNIFICANT

Missing: `allowed_domains` and `blocked_domains` parameters. Only accepts `query`.

### 19.6 Agent Tool — CRITICAL

Missing 7+ features: `run_in_background` parameter, `isolation` modes (worktree, remote), `name` parameter, `team_name` for multi-agent coordination, `mode` parameter, `cwd` override, `description` parameter.

### 19.7 AskUserQuestion — CRITICAL

Claude Code: complex multi-question system supporting 1-4 questions each with header, options (label/description/preview), multiSelect, annotations, uniqueness validation. ClawGo: single `question` string parameter, returns text with metadata flag. Trivial stub vs full interactive system.

### 19.8 NotebookEdit — INCOMPATIBLE

Different parameter models that would confuse the model:
- Claude Code: `notebook_path`, `cell_id` (Jupyter ID), `new_source`, `edit_mode` (replace/insert/delete)
- ClawGo: `path`, `index` (positional), `source`, `command` (add_cell/edit_cell/delete_cell/insert_cell), `cell_type`

### 19.9 Glob Tool — MODERATE

Missing: max results cap (100 files) with truncation flag in output. ClawGo returns unbounded result sets.

### 19.10 File State Tracking — ABSENT

Claude Code maintains a `FileStateCache` (LRU, 100 entries, 25MB) tracking per-file: content, timestamp, offset, limit, `isPartialView` flag. The `isPartialView` flag blocks edits on auto-injected files (CLAUDE.md, MEMORY.md) without explicit read. Used to enforce "must read before edit" constraint. **ClawGo has no file state tracking at all** — the model can edit files it hasn't read.

## 20. Error Handling, Security & Miscellaneous (Deep Audit, 2026-04-04)

### 20.1 Error Hierarchy

Claude Code: 9 custom error classes (`ClaudeError`, `MalformedCommandError`, `AbortError`, `ConfigParseError`, `ShellError`, `TeleportOperationError`, `TelemetrySafeError`) with semantic meaning, errno extraction helpers (`isENOENT`, `isAbortError`, `getErrnoCode`), and telemetry-safe error wrapping.

ClawGo: generic `ClawGoError` struct with 5 variants (`ConfigError`, `APIError`, `ToolError`, `PermissionError`, `SessionError`). Missing: `AbortError` pattern (affects cancellation semantics), `TelemetrySafeError` (telemetry messages aren't sanitized), errno extraction helpers, Axios error classification.

### 20.2 Bash Command Security — CRITICAL

Claude Code: 23+ numbered security validators via tree-sitter AST parsing (~700 lines). Checks include: heredoc-in-substitution safety, brace expansion detection, Unicode whitespace & control chars, ZSH-specific syntax (`~[`, `=cmd` equals expansion, `zmodload`/`emulate`/`sysopen`/`ztcp`/`zsocket`), backslash-whitespace escaping, JQ system() function, IFS injection, obfuscated flags, shell metacharacters, proc/environ access, malformed token injection, comment-quote desync, quoted newlines. **Fail-closed design**: any unknown node → "too-complex" → ask.

ClawGo: basic safe/risky/deny classification via mvdan.cc/sh (~220 lines). **Missing 95% of security validators.** No heredoc validation, no brace expansion checks, no Unicode/control char detection, no ZSH syntax detection, no obfuscation pattern checks. **Significant security blind spots** — dangerous commands and injection patterns will be auto-approved.

### 20.3 Input Validation — semanticBoolean/semanticNumber

Claude Code uses `semanticBoolean()` and `semanticNumber()` wrappers that accept model-generated string representations (`"true"`/`"false"`, `"42"`) and coerce them to native types. Uses `z.preprocess()` to preserve schema type hints. Defense: rejects invalid forms (doesn't use `coerce.boolean()` which treats "false" as truthy).

ClawGo: no semantic validation. Model-generated string booleans/numbers may cause validation errors.

### 20.4 Cost Pricing Table

Claude Code: 12 pricing tiers (Haiku 3.5, Haiku 4.5, Sonnet 3.5, Sonnet 4, Sonnet 4.6, Opus 4, Opus 4.1, Opus 4.5, Opus 4.6 + fast mode at 10x). Includes web search cost ($0.01/request).

ClawGo: 4 models only (Sonnet 4, Opus 4, Haiku 3.5, Sonnet 3.5 v2). Missing: Haiku 4.5, Opus 4.5, Opus 4.6, Sonnet 4.6, fast mode multiplier, web search costs.

### 20.5 Platform Detection

Claude Code: returns `macos`/`windows`/`wsl`/`linux`/`unknown`, WSL version detection, Linux distro info, VCS detection (git/hg/svn/perforce/tfs/jujutsu/sapling).

ClawGo: OS/Arch flags only. Missing: WSL detection (critical for shell selection), VCS detection, Linux distro info.

### 20.6 Startup Sequence

Claude Code performs 16 initialization steps: config validation, safe env vars, graceful shutdown setup, event logging + GrowthBook watch, OAuth account population, JetBrains IDE detection, GitHub repo detection, remote settings + policy limits, first start time recording, mTLS configuration, global proxy agents, API preconnection, upstream proxy (CCR), Windows shell setup, LSP manager cleanup, session team cleanup.

ClawGo: `cmd.Execute()` — 1 step. Missing 95% of initialization.

### 20.7 Concurrency in Tool Execution

Claude Code: `Promise.all()` for parallel read-only tool execution. ClawGo: has `RunConcurrentBatch()` with goroutines and `sync.WaitGroup`, but the query loop may not fully exercise it. Need to verify wiring.

## 21. Tool Name and Schema Mismatches (Sonnet Deep Audit, 2026-04-04)

**4 tools have wrong names. 10+ tools have wrong parameter names. Multiple tools have fundamentally different semantics.**

### 21.1 Tool Name Mismatches (model will call wrong tool)

| Tool | Claude Code Name | ClawGo Name | Impact |
|---|---|---|---|
| Brief/SendUserMessage | `SendUserMessage` (primary), `Brief` (alias) | `Brief` | Model may use wrong name |
| StructuredOutput | `StructuredOutput` | `SyntheticOutput` | Model calls non-existent tool |
| ListMcpResources | `ListMcpResourcesTool` | `ListMcpResources` | Model calls non-existent tool |
| ReadMcpResource | `ReadMcpResourceTool` | `ReadMcpResource` | Model calls non-existent tool |

### 21.2 Parameter Name Mismatches (tool call will fail)

| Tool | Claude Code Param | ClawGo Param |
|---|---|---|
| Config | `setting` | `key` |
| Skill | `skill` | `name` |
| TeamCreate | `team_name` | `name` |
| TaskGet | `taskId` (camelCase) | `task_id` (snake_case) |
| ExitPlanMode | no required input | `plan_summary` (required) |
| TeamDelete | no params (deletes current team) | `name` (required) |

### 21.3 Schema Semantic Mismatches

**Brief/SendUserMessage:** Claude Code's primary user-communication tool with `status: enum['normal','proactive']` (required), `attachments: string[]` (optional). ClawGo's Brief is a mode-toggle, not a communication channel. Completely different purpose.

**StructuredOutput/SyntheticOutput:** Claude Code accepts any JSON object (dynamic schema validation per `--json-schema`). ClawGo accepts `content: string` + `format: enum['text','json','markdown']`. Completely different input model.

**TodoWrite:** Claude Code: items have `content` + `status: enum['pending','in_progress','completed']` + `activeForm` (required). ClawGo: items have `id` + `content` + `status: enum['pending','in_progress','done']` + `priority`. Different status enum value (`completed` vs `done`), different required fields, different semantics (Claude Code replaces full list; ClawGo merges by ID).

**ExitPlanMode:** Claude Code reads plan from disk, accepts optional `allowedPrompts` array for permission pre-authorization. ClawGo requires `plan_summary` string input. Fundamentally different execution model.

**EnterPlanMode:** Claude Code `isReadOnly: true`. ClawGo `IsReadOnly: false`. Affects permission classification.

**TaskOutput:** Claude Code has `block: boolean` (default true, with `semanticBoolean()`) and `timeout: number` (default 30000ms) for blocking waits. ClawGo has neither — returns instant snapshot only.

**Config:** Claude Code has no `action` field (GET/SET inferred from `value` presence), `value` accepts `string|boolean|number`. ClawGo has `action: enum['get','set','list']` (required), `value` is string-only.

**Skill:** Claude Code has `args: string` (optional) for passing arguments. ClawGo has no `args`. Claude Code executes skills as forked sub-agents; ClawGo returns file content.

**ToolSearch:** Claude Code supports `select:` prefix for direct tool activation and `max_results` parameter. ClawGo does plain substring search with no result limit control.

### 21.4 Missing Semantic Validation

Claude Code wraps numeric and boolean tool parameters in `semanticNumber()` and `semanticBoolean()` which accept model-generated string representations (`"true"`, `"42"`) and coerce to native types. ClawGo has no coercion — model-generated string booleans/numbers will cause validation errors for Bash `timeout`, `run_in_background`, PowerShell `timeout`, TaskOutput `block`/`timeout`, and others.

## 22. Session Format Incompatibility (Sonnet Deep Audit, 2026-04-04)

**The two JSONL formats are completely incompatible. Sessions cannot be shared between Claude Code and ClawGo.**

### 22.1 Schema Differences

Claude Code JSONL line (TranscriptMessage): flat object with `type`, `uuid`, `parentUuid`, `timestamp`, `sessionId`, `cwd`, `userType`, `entrypoint`, `version`, `gitBranch`, `isSidechain`, `agentId`, `message` (nested).

ClawGo JSONL line: `{"type":"...", "message": {...}}` — two-field envelope, no UUID, no chain, no metadata.

### 22.2 Path Hashing

Claude Code: `sanitizePath()` on cwd (not a hash). ClawGo: `sha256(path)[:8]` (16 hex chars). **Different directories** — sessions saved by one are invisible to the other.

### 22.3 Session ID Format

Claude Code: RFC 4122 UUID v4 (`crypto.randomUUID()`). ClawGo SDK engine: 32-char hex string (not UUID format). ClawGo session storage: UUID-shaped but without version/variant bits set.

### 22.4 Missing Metadata Entry Types

Claude Code has 20+ metadata-only JSONL entry types: `summary`, `custom-title`, `ai-title`, `last-prompt`, `task-summary`, `tag`, `agent-name`, `agent-color`, `pr-link`, `mode`, `worktree-state`, `file-history-snapshot`, `attribution-snapshot`, `content-replacement`, `marble-origami-commit`, `marble-origami-snapshot`, `queue-operation`, `speculation-accept`. ClawGo has zero metadata entry types.

### 22.5 File Permissions

Claude Code: directories `0o700`, files `0o600`. ClawGo: directories `0o755`, files `0o644`. Security difference.

### 22.6 Message Normalization Pipeline

Claude Code runs a 14-step normalization pipeline before every API call: attachment reordering, virtual message stripping, error-block stripping, tool reference handling, consecutive user message merging, system→user conversion, progress filtering, assistant message dedup by ID, thinking orphan filtering, tool_use/tool_result pairing repair, empty content placeholders, tool search processing. **ClawGo does zero normalization** — messages go directly to `ToParam()`.

## 23. MCP Implementation Depth (Sonnet Deep Audit, 2026-04-04)

### 23.1 Client Transport Gaps

Only stdio implemented. SSE, HTTP, WebSocket, SDK, claudeai-proxy, sse-ide, ws-ide, in-process all return `"transport not yet supported"` error.

### 23.2 Connection Lifecycle Gaps

No reconnection mechanism (once closed, session is gone). No connection timeout (delegates to Go context). No stderr capture (forwards to os.Stderr). No SIGINT→SIGTERM→SIGKILL shutdown escalation (uses SIGKILL via exec.CommandContext). No error recovery (no consecutiveConnectionErrors tracking). No session expiry detection. No batched connection concurrency (sequential, one at a time). No `roots` handler.

### 23.3 Tool Discovery Gaps

No name normalization — no `mcp__` prefix applied to discovered tools. No registry integration. No `CLAUDE_AGENT_SDK_MCP_NO_PREFIX` support. No tool capability flag extraction (readOnlyHint, destructiveHint, openWorldHint, title). No Unicode sanitization. No IDE tool filtering. No LRU caching.

### 23.4 Tool Execution Gaps

No timeout wrapper (default ~27.8 hours in Claude Code). No progress notifications. No `_meta` passthrough. No 401 re-auth handling. No session expiry retry. No elicitation/URL elicitation retry (MCP error -32042). No result size limits or file persistence. No image resizing. No content type transformation (only handles text, no audio/image/resource/resource-link).

### 23.5 Config Gaps

No env var expansion in config values (`${VAR}`, `${VAR:-default}`). No `headersHelper` (dynamic header script). No server enable/disable state. No per-project approval flow. No plugin dedup. No stable change detection (SHA-256 hash for reload). No `.mcp.json` project-level config loading.

### 23.6 Enterprise MCP Gaps

No `allowedMcpServers`/`deniedMcpServers` policy enforcement. No `managed-mcp.json`. No OAuth for MCP servers (`ClaudeAuthProvider` with DCR, PKCE, token refresh, keychain storage). No XAA two-leg token exchange. No Claude.ai connector fetch. No official MCP registry prefetch. No channels support.

### 23.7 Missing MCP Subsystems (15+)

Auth cache, needs-auth connection state, disabled connection state, reconnection management, elicitation handler, McpAuthTool, MCP skills, prefetch orchestration, IDE-specific handling, config management API, CCR proxy URL unwrapping, server instructions truncation, proxy support, mTLS, cleanup registry.

## 24. Bridge/Swarm/Daemon Protocol Gaps (Sonnet Deep Audit, 2026-04-04)

### 24.1 Bridge Uses Wrong API Endpoints

| Operation | Claude Code | ClawGo |
|---|---|---|
| Register | `POST /v1/environments/bridge` | `POST /v1/bridge/environments` |
| Poll | `GET /v1/environments/{id}/work/poll` | `GET /v1/bridge/environments/{id}/work` |
| Deregister | `DELETE /v1/environments/bridge/{id}` | `PUT /v1/bridge/environments/{id}/status` |

Different endpoints, different auth schemes, different request/response shapes. **ClawGo's bridge would not work against real Anthropic infrastructure.**

### 24.2 Bridge Work Handler Is a Stub

`handleWork()` is explicitly `// In a complete implementation, this would...` followed by `<-ctx.Done()`. No child process spawning, no WebSocket relay, no result streaming. Missing: work acknowledgment (`/ack`), heartbeat, work stop, session archiving, session reconnect. No CCR v2 support.

### 24.3 Swarm Worker Differences

Workers use incompatible ID format (`agent-{6hex}` vs `agentName@teamName`). Workers use hardcoded minimal system prompt vs full inherited system prompt. Workers hard-capped at 30 turns. No tmux/iTerm2 terminal backends. No on-disk team file. No permission synchronization between leader and workers. Token usage in notifications always zero.

### 24.4 Cron Fires Shell Commands, Not Prompts

Claude Code cron: fires prompts to Claude query loop, routes to teammates. ClawGo cron: fires `sh -c` shell commands. Fundamentally different design. Also: different file location (project-relative vs config-dir), no jitter, no missed task detection, no file hot-reload, no auto-expiry, no teammate routing.

### 24.5 IDE Server Is Echo Stub

`handleMessage()` echoes messages back. No control_request/control_response protocol, no permission handling, no session index, no auth token, no Unix socket.

### 24.6 UDS Has No Peer Discovery

Socket primitives exist but no PID registry, no session enumeration, no address book for routing between sessions.

## 25. Feature Flags and Enterprise (Sonnet Deep Audit, 2026-04-04)

### 25.1 Feature Flags: Infrastructure Exists, Nothing Wired

ClawGo has a working GrowthBook client (`internal/featureflags/`) with polling, caching, and attribute support. **But no other ClawGo package calls `featureflags.IsEnabled()`** — the infrastructure exists but is dead code. Claude Code has 80+ compile-time flags gating major features.

### 25.2 MDM Path Mismatches

| Platform | Claude Code | ClawGo |
|---|---|---|
| Windows registry | `HKLM\SOFTWARE\Policies\ClaudeCode` | `HKLM\SOFTWARE\Policies\Anthropic\ClaudeCode` |
| macOS plist | `com.anthropic.claudecode` | `com.anthropic.claude-code` |

These are functional bugs — enterprise settings from real Claude Code deployments won't be read by ClawGo.

### 25.3 Policy Limits Schema Mismatch

Claude Code: generic named-restriction map (`{ restrictions: { "name": { allowed: bool } } }`). ClawGo: typed struct (`DisabledTools`, `MaxTurns`, etc.). Different polling intervals (1hr vs 30min), different cache strategies (SHA-256 vs ETag).

### 25.4 CLAUDE.md Frontmatter Not Parsed

Claude Code parses YAML frontmatter supporting: `allowed-tools`, `description`, `type`, `paths` (conditional glob loading), `argument-hint`, `model`, `skills`, `hooks`, `effort`, `context`, `agent`, `shell`, `user-invocable`, `hide-from-slash-command-tool`, `version`, `when_to_use`. ClawGo loads file content only.

### 25.5 Missing Feature Categories

Voice mode, Chrome integration, desktop handoff, mobile QR codes, deep link protocol handler (`claude-cli://`), release notes display, onboarding flow, tree-sitter bash analysis, buddy/companion, away summary, ultrathink keyword detection, verification agent, shot statistics, skill improvement hooks, terminal panel, message actions, workflow scripts, Perfetto tracing, anti-distillation, commit attribution — all absent from ClawGo.

## 26. Runtime Behavioral Test Results (2026-04-04)

Live testing against Claude Code v2.1.92 and ClawGo dev binary.

### 26.1 Test Matrix: 31 tests, 12 pass, 19 fail

**Flag acceptance (16 tested, 2 pass, 14 fail):**
Every flag beyond ClawGo's 12 registered flags returns `Error: unknown flag`. Failed: `-p`, `--continue`, `--thinking`, `--effort`, `--bare`, `--debug`, `--fallback-model`, `--input-format`, `--json-schema`, `--add-dir`, `--agent`, `--dangerously-skip-permissions`, `--system-prompt-file`, `--append-system-prompt`, `--max-budget-usd`, `--disable-slash-commands`.

**Subcommands (10 tested, 5 pass, 5 fail):**
- `auth --help`: Claude shows auth command family. ClawGo falls back to root help (no auth command).
- `auth status --json`: Claude returns JSON with login status, email, org, subscription type. ClawGo: `Error: unknown flag: --json`.
- `mcp list`: Claude lists connected MCP servers with health status. ClawGo shows mcp help (no list subcommand).
- `plugin --help`: Claude shows full plugin management tree (install/uninstall/enable/disable/marketplace). ClawGo falls back to root help.
- `update --help`: Claude shows update command. ClawGo falls back to root help.

**Print mode (3 tested, 0 pass, 3 fail):**
All print mode tests fail because ClawGo doesn't have `-p` flag. Claude Code returns text, JSON, or stream-json output.

### 26.2 New Runtime-Only Findings

**Credentials parsing bug (BLOCKER):** ClawGo expects `{"apiKey": "..."}` in `~/.claude/.credentials.json`. Claude Code stores `{"claudeAiOauth": {"accessToken": "...", ...}, "mcpOAuth": {...}}`. ClawGo cannot read existing Claude Code OAuth tokens — it requires `ANTHROPIC_API_KEY` env var as a workaround.

**Duplicate error messages:** ClawGo outputs every error message twice (e.g., `Error: unknown flag: --print\nunknown flag: --print`). Claude Code outputs single error.

**Short flag conflict:** ClawGo `-v` = `--version`. Claude Code `-v` = `--verbose`. Scripts using `-v` for verbose will get version output instead.

**Error message formatting:** Claude Code wraps API errors in user-friendly messages ("Invalid API key . Fix external API key"). ClawGo returns raw JSON API responses ("401 Unauthorized ... invalid x-api-key").

**Rate limiting behavior:** ClawGo hits persistent 429 rate limits where Claude Code succeeds with the same API. Suggests differences in retry logic, request headers, or beta headers that affect rate limit tier.

**Session directory naming:** Claude Code uses `-home-ubuntu-claude-code/` as project hash. ClawGo uses `-home-ubuntu/` — different path hashing produces different directories.

**Missing camelCase flag aliases:** Claude Code accepts both `--allowedTools` and `--allowed-tools`. ClawGo only accepts `--allowed-tools`.

### 26.3 Performance Comparison (Runtime Measured)

| Metric | Claude Code | ClawGo | Ratio |
|---|---|---|---|
| Startup time (`--version`) | 111ms | 42ms | **2.6x faster** |
| Memory (RSS) | 191.8 MB | 28.4 MB | **6.8x less** |
| Binary size | 221 MB | 45 MB | **4.9x smaller** |

ClawGo's performance advantages are real and significant, even though feature parity is far from complete.
