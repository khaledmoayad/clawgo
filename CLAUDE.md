<!-- GSD:project-start source:PROJECT.md -->
## Project

**ClawGo**

A complete Go rewrite of Claude Code — Anthropic's CLI for Claude. ClawGo is a drop-in replacement that replicates every feature of the TypeScript/React original: interactive REPL, streaming LLM queries, tool system, MCP server/client, Agent SDK, OAuth, hooks, subagents, IDE extensions, and all other capabilities. Built with Bubble Tea for the TUI and distributed as a single compiled binary.

**Core Value:** Exact feature parity with Claude Code — users can swap the binary and everything works identically, with the added benefits of a single compiled Go binary (faster startup, lower memory, no runtime dependencies).

### Constraints

- **Feature parity**: Every feature must work identically to the TypeScript version — no shortcuts, no "we'll add it later"
- **Binary compat**: CLI flags, env vars, config file formats must be identical
- **Platform**: Must support macOS, Linux, Windows — same as original
- **API compat**: Must speak the same Anthropic Messages API protocol
- **Config compat**: Must read/write the same `~/.claude/` config structure
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- TypeScript (.ts / .tsx) - Entire codebase (~1884 source files)
- Pure TypeScript ports replace native dependencies where possible (yoga-layout, file-index, color-diff in `native-ts/`)
## Runtime
- Bun (primary runtime) - Detected via `process.versions.bun`, `bun:bundle` imports, and `Bun.embeddedFiles`
- Node.js (fallback/compatibility) - Minimum version 18 enforced in `setup.ts:70-79`
- Bun compile-time feature flags via `feature()` from `bun:bundle` for dead code elimination (e.g., `BUDDY`, `COORDINATOR_MODE`, `KAIROS`, `VOICE_MODE`, `UDS_INBOX`, `WEB_BROWSER_TOOL`, `CONTEXT_COLLAPSE`, `HISTORY_SNIP`, etc.)
- Build-time macros: `MACRO.VERSION`, `MACRO.PACKAGE_URL` inlined at build time
- npm (published as `@anthropic-ai/claude-code` on npm registry)
- No `package.json` or lockfile present in this extracted source (source was extracted from a sourcemap `.map` file in the npm package)
## Frameworks
- React 19+ (with React Compiler) - Used for terminal UI rendering (`react/compiler-runtime` imports throughout)
- Custom Ink fork (`ink/` directory) - Forked and internalized React-based terminal renderer
- Commander.js via `@commander-js/extra-typings` - CLI argument parsing and command structure (`main.tsx:22`)
- Not detected in this source tree (test files not included in the extracted source)
- Bun bundler - Compiles to standalone executable with embedded files
- Biome - Linting (biome-ignore directives throughout codebase)
- ESLint - Custom rules (e.g., `custom-rules/no-top-level-side-effects`, `custom-rules/no-process-env-top-level`)
## Key Dependencies
- `@anthropic-ai/sdk` - Anthropic API client (Messages API with beta features, streaming, token counting)
- `@modelcontextprotocol/sdk` - MCP (Model Context Protocol) client and server implementation
- `@anthropic-ai/claude-agent-sdk` - Agent SDK types for permission modes
- `@anthropic-ai/sandbox-runtime` - Sandboxed command execution (bubblewrap-based)
- `chalk` - Terminal color output
- `react` / `react-reconciler` - Terminal UI framework
- `auto-bind` - Method binding for Ink components
- `signal-exit` - Process exit handling
- `axios` - HTTP client used throughout for API calls (Anthropic API, Datadog, GrowthBook, settings sync, files API, etc.)
- `ws` - WebSocket client for voice streaming STT, bridge communication
- `https-proxy-agent` - HTTP/HTTPS proxy support
- `undici` - Lazily loaded for proxy and fetch configuration
- `zod` (v4, via `zod/v4`) - Schema validation for API responses, settings, configs
- `lodash-es` - Utility functions (memoize, throttle, noop, mapValues, pickBy, uniqBy, isEqual)
- `diff` - Text diffing for file edit tool, structured patches
- `marked` - Markdown parsing and rendering
- `xss` - XSS sanitization for OAuth/IdP redirect pages
- `execa` - Child process execution (used for git, gh CLI, shell commands, etc.)
- `@aws-sdk/client-bedrock-runtime` - AWS Bedrock integration (~279KB, lazy-loaded)
- `google-auth-library` - GCP Vertex AI authentication
- `@azure/identity` (referenced, for Azure Foundry) - DefaultAzureCredential
- `@growthbook/growthbook` - Feature flags and A/B testing (remote eval)
- `@opentelemetry/api`, `@opentelemetry/sdk-metrics`, `@opentelemetry/sdk-trace-base`, `@opentelemetry/sdk-logs` - OpenTelemetry telemetry (traces, metrics, logs)
- `@opentelemetry/resources`, `@opentelemetry/semantic-conventions`, `@opentelemetry/core` - OTel resource detection
- `@ant/computer-use-mcp` - Computer use (screen control) MCP tools
- `@ant/computer-use-swift` - macOS Swift computer use API
- `@ant/claude-for-chrome-mcp` - Chrome browser integration tools
- `@anthropic-ai/mcpb` - MCP manifest types (DXT)
## Configuration
- `ANTHROPIC_API_KEY` - Direct API key
- `ANTHROPIC_AUTH_TOKEN` - OAuth/bearer token
- `ANTHROPIC_BASE_URL` / `CLAUDE_CODE_API_BASE_URL` - API endpoint override
- `CLAUDE_CODE_USE_BEDROCK` / `CLAUDE_CODE_SKIP_BEDROCK_AUTH` - AWS Bedrock mode
- `CLAUDE_CODE_USE_VERTEX` / `ANTHROPIC_VERTEX_PROJECT_ID` / `CLOUD_ML_REGION` - GCP Vertex AI
- `CLAUDE_CODE_USE_FOUNDRY` / `ANTHROPIC_FOUNDRY_RESOURCE` / `ANTHROPIC_FOUNDRY_API_KEY` - Azure Foundry
- `ANTHROPIC_MODEL` - Model override
- `ANTHROPIC_SMALL_FAST_MODEL` - Small/fast model override (Haiku)
- `CLAUDE_CONFIG_DIR` - Config directory override (default: `~/.claude`)
- `USER_TYPE` - Build-time define (`ant` for internal builds)
- `NODE_ENV` - Standard Node env
- `CLAUDE_CODE_REMOTE` - Remote session mode
- `CLAUDE_CODE_CONTAINER_ID` / `CLAUDE_CODE_REMOTE_SESSION_ID` - Container identification
- `CLAUDE_CODE_ENTRYPOINT` - Entry mode (`local-agent`, `claude-desktop`, etc.)
- `CLAUDE_CODE_SIMPLE` / `CLAUDE_CODE_DISABLE_THINKING` / `DISABLE_COMPACT` - Feature disabling
- `CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC` - Privacy/traffic reduction
- `CLAUDE_CODE_BUBBLEWRAP` / `IS_SANDBOX` - Sandbox detection
- `~/.claude/.config.json` or `~/.claude.json` - Global config (legacy fallback)
- `.claude/settings.json` - Project-level settings
- `CLAUDE.md` - Project memory/instructions file
- Settings sources: user, project, enterprise (policySettings), remoteManagedSettings
- macOS: Keychain (`utils/secureStorage/macOsKeychainStorage.ts`)
- Linux: Plain text fallback (`utils/secureStorage/plainTextStorage.ts`) with TODO for libsecret
- Keychain prefetch on startup for OAuth tokens (`utils/secureStorage/keychainPrefetch.ts`)
## Platform Requirements
- Bun runtime (for `bun:bundle` feature flags, `Bun.embeddedFiles`, etc.)
- Node.js >= 18 (fallback compatibility)
- Git (heavily used for worktree, branch, diff operations)
- Optional: `gh` CLI (GitHub integration), `tmux` (worktree sessions)
- Published as npm package (`@anthropic-ai/claude-code`)
- Bun-compiled standalone executable (single binary with embedded files)
- Supports: macOS (darwin), Linux, Windows (win32) via platform detection in `utils/platform.ts`
- Remote mode: Docker/container environments (Claude Code Remote / CCR)
- Sandbox: bubblewrap-based sandboxing on Linux, Docker containers
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Language and Runtime
## Naming Patterns
- `camelCase.ts` for modules, utilities, and logic files: `costHook.ts`, `lazySchema.ts`, `semanticBoolean.ts`
- `PascalCase.ts` or `PascalCase.tsx` for classes and major abstractions: `QueryEngine.ts`, `Tool.ts`, `Task.ts`
- Tool directories use PascalCase: `BashTool/`, `FileReadTool/`, `GlobTool/`
- Tool directory files follow a consistent pattern: `{ToolName}.ts` or `{ToolName}.tsx`, `UI.tsx`, `prompt.ts`, `utils.ts`
- Command directories use kebab-case: `add-dir/`, `break-cache/`, `install-github-app/`
- Command entry files are always `index.ts` (or `index.js`, `index.tsx`)
- `camelCase` for all functions: `getTaskIdPrefix()`, `isTerminalTaskStatus()`, `formatFileSize()`
- Getter functions prefixed with `get`: `getClaudeConfigHomeDir()`, `getCwd()`, `getSessionId()`
- Boolean check functions prefixed with `is` or `has`: `isAbortError()`, `isENOENT()`, `hasConsoleBillingAccess()`
- React hooks prefixed with `use`: `useCanUseTool()`, `useAppState()`, `useTerminalSize()`
- `camelCase` for local variables and module-level state
- `UPPER_SNAKE_CASE` for constants: `MAX_STATUS_CHARS`, `PROGRESS_THRESHOLD_MS`, `BASH_SEARCH_COMMANDS`
- Branded types use `PascalCase`: `SessionId`, `AgentId`
- `PascalCase` for all types: `TaskType`, `TaskStatus`, `ToolUseContext`, `PermissionMode`
- Type-only imports use `import type`: `import type { AppState } from './state/AppState.js'`
- Union types defined with `type =` and string literals: `type TaskType = 'local_bash' | 'local_agent' | ...`
- Branded types use intersection with `__brand`: `type SessionId = string & { readonly __brand: 'SessionId' }`
- Deprecated functions/APIs append `_DEPRECATED` to name: `writeFileSync_DEPRECATED()`, `getSettings_DEPRECATED()`, `execSyncWithDefaults_DEPRECATED()`
- Experimental features append `_EXPERIMENTAL`: `criticalSystemReminder_EXPERIMENTAL`
- Sensitive analytics metadata uses a verbose type name: `AnalyticsMetadata_I_VERIFIED_THIS_IS_NOT_CODE_OR_FILEPATHS` (forces developer to acknowledge data safety)
- PII-safe metadata uses: `AnalyticsMetadata_I_VERIFIED_THIS_IS_PII_TAGGED`
## Code Style
- 2-space indentation
- Single quotes for string literals
- No semicolons at end of statements (except in React Compiler output which adds them)
- Trailing commas in multi-line constructs
- Biome is the primary linter/formatter (biome-ignore comments present throughout)
- ESLint also used for custom rules (eslint-disable comments reference custom rules)
- `custom-rules/no-process-env-top-level` -- Avoid reading process.env at module top level
- `custom-rules/no-process-exit` -- Avoid direct process.exit calls
- `custom-rules/no-sync-fs` -- Avoid synchronous filesystem operations
- `custom-rules/no-top-level-side-effects` -- No side effects at module top level
- `custom-rules/prefer-use-keybindings` -- Prefer useKeybindings hook over useInput
- `custom-rules/prefer-use-terminal-size` -- Prefer useTerminalSize hook
- `custom-rules/no-direct-json-operations` -- Prefer wrapped JSON operations from `utils/slowOperations.ts`
- `custom-rules/no-direct-ps-commands` -- Avoid direct ps command usage
- `biome-ignore lint/suspicious/noConsole` -- Console usage requires explicit justification
- `biome-ignore-all assist/source/organizeImports` -- Used when import order matters (ANT-ONLY markers)
## Import Organization
- Always use `.js` extension in import paths (TypeScript files imported as `.js` for ESM compatibility)
- Use `import type` for type-only imports to enable tree-shaking
- Lodash imported as individual functions: `import memoize from 'lodash-es/memoize.js'` (not full lodash)
- React imported as namespace: `import * as React from 'react'` or `import React from 'react'`
- Destructured hook imports: `import { useEffect, useState, useCallback } from 'react'`
- `src/` alias used in some files (e.g., `from 'src/services/analytics/index.js'`)
- Most files use relative imports (`../`, `../../`)
- Deeper nesting tends to use `src/` prefix
- Feature-gated code uses `require()` for conditional loading (dynamic import alternative)
- Wrapped in `eslint-disable @typescript-eslint/no-require-imports` blocks
- Used extensively for ant-only features, experimental features, and optional capabilities
## Error Handling
- Custom error hierarchy rooted in `ClaudeError` at `utils/errors.ts`
- Domain-specific errors: `ShellError`, `AbortError`, `ConfigParseError`, `MalformedCommandError`, `TeleportOperationError`
- `TelemetrySafeError_I_VERIFIED_THIS_IS_NOT_CODE_OR_FILEPATHS` -- Error with message verified safe for telemetry logging
- All custom errors set `this.name` to the class name
- `toError(e: unknown): Error` -- Normalize any caught value to Error instance
- `errorMessage(e: unknown): string` -- Extract message from unknown error
- `isAbortError(e: unknown): boolean` -- Check for abort-related errors (multiple sources)
- `isENOENT(e: unknown): boolean` -- Check for file-not-found errors
- `getErrnoCode(e: unknown): string | undefined` -- Extract errno code safely
- Use type-safe error checking functions instead of casting: `isENOENT(e)` not `(e as NodeJS.ErrnoException).code === 'ENOENT'`
- Silently swallow errors only when explicitly acceptable (with comment)
- `logError()` from `utils/log.ts` for error logging to file and telemetry
- `logForDiagnosticsNoPII()` for diagnostic logging that must not contain PII
## Logging
- `logError(error)` from `utils/log.ts` -- Error logging with telemetry
- `logForDebugging(message, ...)` from `utils/debug.ts` -- Debug-mode logging
- `logForDiagnosticsNoPII(level, event, data?)` from `utils/diagLogs.ts` -- PII-free diagnostic logs
- `logEvent(name, metadata)` from `services/analytics/index.ts` -- Analytics event logging
- `console.log/warn/error` requires biome-ignore comment justification
- Never log PII (file paths, user data) to diagnostics or analytics
- Use `AnalyticsMetadata_I_VERIFIED_THIS_IS_NOT_CODE_OR_FILEPATHS` type to mark safe analytics data
- Debug logging gated behind `isDebugMode()` check or `--debug` CLI flag
## Validation
- All tool input schemas use `z.strictObject({})` for strict validation
- Schemas wrapped in `lazySchema()` for deferred construction (reduces startup time):
- `semanticNumber()` and `semanticBoolean()` wrappers handle model-generated string coercion:
- Output schemas also use Zod for structured tool results
## Component Patterns (Terminal UI)
- Functional components only (no class components)
- React Compiler active -- most `.tsx` files import `react/compiler-runtime` and use `_c()` memoization
- Props defined as inline `type Props = { ... }` above the component
- Components export named functions: `export function App(props: Props)`
- Custom store pattern at `state/store.ts` using `createStore()` with listener-based subscriptions
- `AppState` is the global state type at `state/AppStateStore.ts`
- `useAppState(selector)` hook for reading state with selectors
- `setAppState(updater)` for immutable state updates: `setAppState(prev => ({ ...prev, ... }))`
- No CSS -- terminal rendering via Ink's `Box` and `Text` components
- Themed wrappers: `ThemedBox`, `ThemedText` at `components/design-system/`
- Theme provider at `components/design-system/ThemeProvider.tsx`
- Colors via theme tokens, not hardcoded values
- `ink.ts` at root re-exports all Ink primitives (`Box`, `Text`, `Button`, etc.) wrapped with ThemeProvider
- Components import from `../../ink.js` rather than individual Ink files
## Common Abstractions
- All tools defined via `buildTool()` factory from `Tool.ts`
- Each tool in its own directory under `tools/{ToolName}/`
- Standard files per tool: `{ToolName}.ts(x)`, `UI.tsx`, `prompt.ts`, optionally `utils.ts`, `types.ts`, `constants.ts`
- Tools implement: `name`, `description()`, `prompt()`, `inputSchema`, `checkPermissions()`, `call()`, render methods
- Commands defined as objects satisfying `Command` type from `commands.ts`
- `index.ts` exports metadata + lazy `load()` function for deferred code loading
- Implementation in separate file (e.g., `clear.ts`, `compact.ts`)
- Commands have `type: 'local' | 'prompt'`, `name`, `description`, optional `aliases`
- `lodash-es/memoize.js` used extensively for expensive computations
- Custom `memoizeWithLRU()` for bounded caching with eviction
- `lazySchema()` for Zod schemas
- Lazy `require()` for feature-gated modules
- Command `load()` functions for deferred module loading
- Pure utility functions in `utils/` directory (160+ files)
- Grouped by domain: `utils/permissions/`, `utils/model/`, `utils/settings/`, `utils/telemetry/`, `utils/swarm/`
- No side effects in utility modules (enforced by custom ESLint rule)
## Module Design
- Named exports strongly preferred over default exports
- Default exports used primarily for command `index.ts` files (convention for command registration)
- Re-exports used to maintain backwards compatibility: `export { type X } from './path.js'`
- Barrel files used sparingly (mostly `ink.ts`, `commands.ts`, `tools.ts`)
- Types extracted to `types/` directory to break circular dependencies
- `types/permissions.ts` header: "Pure permission type definitions extracted to break import cycles"
- Comments document why re-exports exist: "Re-export for backwards compatibility"
- Lazy `require()` used to break circular dependencies between tools and state
## Comments
- JSDoc on exported functions with non-obvious behavior
- Inline comments explaining "why" not "what" for non-trivial logic
- `// biome-ignore` and `// eslint-disable` with mandatory justification
- Architecture decisions documented in comments when they break normal patterns
- `TODO:` for known future work (sparingly used)
- `// ANT-ONLY` markers for internal-only code
- `// Dead code elimination:` prefix for feature-gated import blocks
- `// DCE:` abbreviation for dead code elimination comments
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- Single-process Node.js/Bun application with an agentic conversation loop
- React (Ink) renders the terminal UI; state managed via a custom lightweight store (not Redux)
- Tools, commands, skills, and plugins are registered at startup and injected into the query context
- Feature flags (`bun:bundle` `feature()`) gate internal/experimental code paths with dead code elimination at build time
- Supports multiple entry points: interactive CLI (REPL), MCP server, Agent SDK, bridge/remote control, and daemon workers
## Layers
- Purpose: Parse CLI args, dispatch to the correct runtime mode
- Location: `entrypoints/cli.tsx` (bootstrap entrypoint), `main.tsx` (full CLI with Commander.js)
- Contains: Fast-path version/dump checks, bridge mode, daemon workers, MCP server startup, SDK entry
- Depends on: Bootstrap, Init, Main
- Used by: The Bun runtime directly
- Purpose: Initialize global state, configs, telemetry, auth, environment variables
- Location: `bootstrap/state.ts` (global mutable state singleton), `entrypoints/init.ts` (initialization sequence)
- Contains: Session IDs, cost tracking counters, project root, telemetry providers, MDM settings prefetch, keychain prefetch
- Depends on: `utils/config.js`, `utils/settings/`, `services/analytics/`, `services/policyLimits/`
- Used by: All other layers
- Purpose: Parse command-line arguments, configure options, and invoke the appropriate launcher
- Location: `main.tsx` (4,683 lines -- the largest file; handles all Commander.js option parsing and mode dispatch)
- Contains: Commander.js program definition, all CLI flags (model, permission-mode, resume, MCP config, agent selection), session resume logic, the `launchRepl` call
- Depends on: `commands.ts`, `tools.ts`, `entrypoints/init.ts`, `replLauncher.tsx`
- Used by: `entrypoints/cli.tsx`
- Purpose: Terminal UI with React/Ink component tree
- Location: `components/`, `screens/`, `ink/`, `context/`
- Contains: `App.tsx` (root provider wrapper), `REPL.tsx` (5,005 lines -- main interactive screen), `PromptInput/`, message display components, dialogs, spinners, diff views, virtual scrolling
- Depends on: State layer, Services layer, Hooks layer
- Used by: User-facing interactive mode
- Purpose: Centralized application state for the REPL session
- Location: `state/store.ts` (generic store factory), `state/AppStateStore.ts` (AppState type + defaults), `state/AppState.tsx` (React context provider)
- Contains: `AppState` type with settings, model config, tasks, MCP connections, plugins, permission context, bridge state, speculation state
- Depends on: Nothing (leaf module)
- Used by: All UI components via `useSyncExternalStore`, tools via `ToolUseContext.getAppState/setAppState`
- Purpose: The agentic conversation loop -- sends messages to Claude API, processes responses, executes tools, manages conversation state
- Location: `query.ts` (streaming query loop, 1,729 lines), `QueryEngine.ts` (SDK-facing wrapper, 1,295 lines)
- Contains: Main `query()` generator function that yields messages/events, auto-compaction logic, token budget tracking, thinking management, tool execution orchestration, fallback model handling
- Depends on: `services/api/claude.ts`, `services/tools/toolOrchestration.ts`, `services/compact/`, `Tool.ts`
- Used by: `REPL.tsx` (interactive mode), `QueryEngine.ts` (SDK/programmatic mode)
- Purpose: Define and execute tools that Claude can invoke
- Location: `Tool.ts` (core types: `Tool`, `ToolUseContext`, `ToolPermissionContext`), `tools.ts` (tool registry), `tools/` (individual tool implementations)
- Contains: ~40 tool implementations, each in its own directory with `*.tsx` (main), `prompt.ts`, `UI.tsx`, `types.ts`, `constants.ts`
- Depends on: `utils/permissions/`, `services/`, `bootstrap/state.ts`
- Used by: Query Engine Layer via `services/tools/toolOrchestration.ts`
- Purpose: Slash commands the user types in the REPL (e.g., `/help`, `/model`, `/compact`)
- Location: `commands.ts` (registry), `commands/` directory (~86 command directories)
- Contains: Each command exports a `Command` object with name, description, handler, and optionally a React component for JSX rendering
- Depends on: Tools, Services, State
- Used by: `REPL.tsx` via `processUserInput/`
- Purpose: Business logic, API clients, and integrations
- Location: `services/`
- Contains:
- Depends on: Utils, Bootstrap
- Used by: Query Engine, Tools, Commands, UI
- Purpose: Shared utilities, helpers, and cross-cutting concerns
- Location: `utils/` (~100+ files across 30+ subdirectories)
- Contains:
- Depends on: `bootstrap/state.ts`
- Used by: Everything
## Data Flow
- `bootstrap/state.ts` holds process-wide mutable singletons (session ID, cost counters, telemetry providers)
- `state/store.ts` implements a minimal pub/sub store (`getState`, `setState`, `subscribe`)
- `state/AppStateStore.ts` defines the `AppState` type (settings, model, tasks, MCP, plugins, permissions, bridge state)
- `state/AppState.tsx` provides React context; components use `useSyncExternalStore` for granular subscriptions
- `state/onChangeAppState.ts` handles side effects on state transitions
- `state/selectors.ts` provides derived state accessors
## Key Abstractions
- Purpose: An action Claude can invoke (file read/write, bash, search, web fetch, etc.)
- Definition: `Tool.ts` exports the `Tool` type with `name`, `description`, `inputSchema`, `call()`, permission checks, UI renderers
- Builder: `buildTool()` in `Tool.ts` constructs tools from a `ToolDef` specification
- Pattern: Each tool is a directory under `tools/` containing implementation, prompt, types, UI, and constants files
- Examples: `tools/BashTool/`, `tools/FileEditTool/`, `tools/AgentTool/`, `tools/GrepTool/`
- Purpose: A slash command the user invokes (e.g., `/help`, `/compact`, `/model`)
- Definition: `commands.ts` exports `Command` type with name, aliases, description, handler function
- Pattern: Each command is a directory under `commands/` with an `index.ts` or `index.tsx`
- Examples: `commands/help/`, `commands/compact/`, `commands/model/`
- Purpose: Runtime context passed to every tool execution
- Definition: `Tool.ts` `ToolUseContext` type
- Contains: Tools list, commands, model info, abort controller, file state cache, `getAppState/setAppState`, MCP clients, agent definitions, thinking config
- Pattern: Created once per query loop iteration, updated by context modifiers from tool results
- Purpose: Complete UI state for the REPL session
- Definition: `state/AppStateStore.ts`
- Contains: Settings, model selection, tasks, MCP connections, plugins, permission context, bridge state, speculation state, expanded view mode
- Pattern: `DeepImmutable<T>` wrapper enforces immutability; updated via `setState(prev => newState)`
- Purpose: Stateful conversation manager for SDK consumers
- Definition: `QueryEngine.ts`
- Contains: Message history, file state cache, attribution state, auto-compact tracking, SDK event mapping
- Pattern: `ask()` method accepts user message, runs full agentic loop, yields SDK-compatible events
- Purpose: Background or parallel work units (bash processes, sub-agents, teammates)
- Definition: `Task.ts` defines `TaskType` (`local_bash`, `local_agent`, `remote_agent`, `in_process_teammate`, etc.)
- State: `tasks/types.ts` defines `TaskState` variants
- Implementations: `tasks/LocalShellTask/`, `tasks/LocalAgentTask/`, `tasks/InProcessTeammateTask/`, `tasks/RemoteAgentTask/`
## Entry Points
- Location: `entrypoints/cli.tsx` (fast-path checks), `main.tsx` (Commander.js full program)
- Triggers: `claude` command with no subcommand or with a prompt
- Responsibilities: Parse flags, initialize environment, load tools/commands/MCP/plugins, launch REPL or execute non-interactive query
- Location: `entrypoints/mcp.ts`
- Triggers: `claude mcp serve` subcommand
- Responsibilities: Start MCP server over stdio, expose tools as MCP tools
- Location: `entrypoints/agentSdkTypes.ts` (type definitions), `entrypoints/sdk/` (schemas)
- Triggers: Programmatic SDK usage (e.g., `@anthropic-ai/claude-code` npm package)
- Responsibilities: Provide `QueryEngine` for programmatic conversation management
- Location: `bridge/bridgeMain.ts`
- Triggers: `claude remote-control` subcommand
- Responsibilities: Serve local machine as a bridge environment, relay events to claude.ai
- Location: `entrypoints/cli.tsx` fast-path, dispatches to `daemon/workerRegistry.js`
- Triggers: Internal daemon supervisor spawns workers
- Responsibilities: Run background workers (assistant mode, cron triggers)
## Error Handling
- API errors categorized by `categorizeRetryableAPIError()` in `services/api/errors.ts`; retriable errors (rate limits, overloads) handled by `withRetry.ts` with exponential backoff
- Tool execution errors caught per-tool; error messages returned as `tool_result` with `is_error: true` so Claude can self-correct
- `FallbackTriggeredError` triggers model fallback (e.g., from extended thinking model to base model)
- `gracefulShutdown.ts` / `gracefulShutdownSync()` ensure telemetry flush and cleanup on process exit
- `utils/errors.ts` provides `errorMessage()` for safe error string extraction
- `logError()` in `utils/log.ts` captures errors to in-memory buffer and diagnostic logs
- `SentryErrorBoundary.ts` wraps React component tree for crash recovery
## Cross-Cutting Concerns
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
