<div align="center">

<h1>CLAWGO</h1>
<h3><em>Your Favorite Terminal Coding Agent, now in Go</em></h3>

  <p>
    <a href="https://github.com/khaledmoayad/clawgo"><img src="https://img.shields.io/badge/Built_with-Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Built with Go"></a>
    <a href="https://github.com/khaledmoayad/clawgo"><img src="https://img.shields.io/badge/Tracking-None-2E8B57?style=for-the-badge" alt="No Tracking"></a>
    <a href="https://github.com/khaledmoayad/clawgo"><img src="https://img.shields.io/badge/Single_Binary-CGO__ENABLED=0-FF6F00?style=for-the-badge" alt="Single Binary"></a>
  </p>

</div>

---

> [!NOTE]
> **ClawGo is a complete Go rewrite of Claude Code** — Anthropic's CLI for Claude. It's a drop-in replacement that replicates the core feature set as a single compiled binary: faster startup, lower memory, no runtime dependencies.
>
> The project is functional with 55 packages, 738 tests, and all core features working. Multi-provider support (Bedrock, Vertex, Foundry), MCP server/client, OAuth, and the full tool system are implemented. Bug reports and contributions welcome — [open an issue](https://github.com/khaledmoayad/clawgo/issues/new) or [reach out](https://github.com/khaledmoayad).

---

# IMPORTANT NOTICE

This repository does not hold a copy of the proprietary Claude Code TypeScript source code.
This is a clean-room Go reimplementation of Claude Code's behavior.

The process was explicitly two-phase:

**Specification** — An AI agent analyzed the publicly available source (leaked via npm sourcemap) and produced exhaustive behavioral specifications: architecture, data flows, tool contracts, system designs. No source code was carried forward.

**Implementation** — A separate AI agent implemented from the spec alone, producing idiomatic Go that reproduces the behavior, not the expression.

This mirrors the legal precedent established by Phoenix Technologies v. IBM (1984) — clean-room engineering of the BIOS — and the principle from Baker v. Selden (1879) that copyright protects expression, not ideas or behavior.

---

## Why Go?

| | TypeScript (Original) | Go (ClawGo) |
|---|---|---|
| **Startup** | ~800ms (Bun runtime init) | ~50ms (compiled binary) |
| **Memory** | ~150MB (Node/Bun heap) | ~30MB (Go runtime) |
| **Distribution** | npm install + runtime | Single static binary |
| **Dependencies** | node_modules + Bun | Zero (CGO_ENABLED=0) |
| **Cross-compile** | Platform-specific builds | `GOOS=linux GOARCH=amd64 go build` |

One binary. No runtime. No `node_modules`. Just `./clawgo`.

---

## Quick Start

### Build from Source

```bash
git clone https://github.com/khaledmoayad/clawgo.git
cd clawgo
make build
```

### Run

```bash
# Set your API key
export ANTHROPIC_API_KEY=sk-ant-...

# Interactive REPL
./clawgo

# Single query
./clawgo "explain this codebase"

# Resume last session
./clawgo --resume
```

### All CLI Flags

```
--model, -m          Model to use (default: claude-sonnet-4-20250514)
--permission-mode    Permission mode: default, plan, auto, bypass
--resume, -r         Resume previous conversation
--session-id         Specific session ID to resume
--verbose, -v        Verbose output
--max-turns          Maximum conversation turns
--system-prompt      Custom system prompt
--output-format      Output format: text, json, stream-json
--allowed-tools      Comma-separated list of allowed tools
--disallowed-tools   Comma-separated list of denied tools
--mcp-config         Path to MCP server configuration file
```

---

## What's Inside

ClawGo is not a wrapper or a thin client. It's a full reimplementation of Claude Code's architecture in idiomatic Go.

### Architecture

```
cmd/clawgo/          Entry point
internal/
  api/               Anthropic API client (streaming, retry, multi-provider)
  app/               Application wiring (REPL, non-interactive, shutdown)
  auth/              OAuth PKCE + secure credential storage
  bridge/            Remote control mode (WebSocket relay)
  classify/          Bash command security classification (AST-based)
  claudemd/          CLAUDE.md project instructions loader
  cli/               Cobra CLI framework with all flags
  commands/          46 slash commands (/help, /model, /compact, /clear, ...)
  compact/           Context compaction (auto, micro, reactive)
  config/            Settings hierarchy (user < project < enterprise < remote)
  cost/              Token/cost tracking with model pricing
  daemon/            Background workers and cron scheduling
  enterprise/        Policy limits, settings sync, team memory
  errors/            Error type hierarchy
  featureflags/      Runtime feature flags (GrowthBook-compatible)
  git/               Git operations and gitignore
  hooks/             Tool event hooks (shell commands on pre/post tool use)
  ide/               IDE detection (VS Code, JetBrains)
  lspclient/         LSP client (JSON-RPC over stdio)
  mcp/               MCP server (stdio) + client + enterprise SSO
  memory/            Session memory extraction and persistence
  permissions/       Multi-mode permission system + file glob checks
  platform/          Platform detection (macOS, Linux, Windows)
  plugins/           Plugin system (git-based installation, manifests)
  query/             Agentic conversation loop with tool orchestration
  remote/            Remote session manager (WebSocket)
  sandbox/           Sandboxed execution (bubblewrap + Docker)
  sdk/               Agent SDK / QueryEngine for programmatic usage
  securestorage/     Keyring (macOS/Linux/Windows) + plaintext fallback
  server/            Direct-connect WebSocket server for IDE extensions
  session/           JSONL session persistence, resume, history
  skills/            Skill system (markdown instructions, change detection)
  swarm/             Multi-agent coordination (leader-worker pattern)
  telemetry/         OpenTelemetry traces, metrics, spans
  teleport/          Teleport API (cross-environment sessions)
  tools/             42 built-in tools (see below)
  tui/               Bubble Tea terminal UI
  uds/               Unix Domain Socket IPC
```

### 42 Built-in Tools

| Category | Tools |
|----------|-------|
| **File I/O** | Read, Write, Edit, Glob, Grep |
| **Execution** | Bash, PowerShell, Sleep |
| **Web** | WebFetch, WebSearch |
| **Agents** | Agent (sub-agent spawning), SendMessage, TeamCreate, TeamDelete |
| **Tasks** | TaskCreate, TaskGet, TaskUpdate, TaskList, TaskStop, TaskOutput |
| **IDE** | LSP, NotebookEdit |
| **MCP** | ListMcpResources, ReadMcpResource |
| **Navigation** | ToolSearch, EnterWorktree, ExitWorktree |
| **Mode** | EnterPlanMode, ExitPlanMode, Brief, Config, Skill |
| **Scheduling** | CronCreate, CronDelete, CronList |
| **Other** | AskUser, TodoWrite, SyntheticOutput |

### 46 Slash Commands

```
/help  /model  /compact  /clear  /cost  /exit  /version  /status
/permissions  /resume  /context  /debug  /diff  /branch  /env
/memory  /config  /export  /login  /logout  /vim  /theme  /color
/copy  /doctor  /effort  /fast  /feedback  /files  /hooks  /ide
/keybindings  /mcp  /plan  /plugin  /review  /rewind  /session
/skills  /stats  /tag  /tasks  /usage  /upgrade  /agents  /add-dir
```

### Multi-Provider Support

```bash
# Direct Anthropic API (default)
export ANTHROPIC_API_KEY=sk-ant-...

# AWS Bedrock
export CLAUDE_CODE_USE_BEDROCK=1

# GCP Vertex AI
export CLAUDE_CODE_USE_VERTEX=1
export ANTHROPIC_VERTEX_PROJECT_ID=my-project
export CLOUD_ML_REGION=us-central1

# Azure Foundry
export CLAUDE_CODE_USE_FOUNDRY=1
export ANTHROPIC_FOUNDRY_RESOURCE=my-resource
```

### MCP (Model Context Protocol)

```bash
# Serve ClawGo's tools over MCP (stdio)
clawgo mcp serve

# Connect to external MCP servers
clawgo --mcp-config ./mcp-servers.json
```

### Programmatic Usage (Agent SDK)

```go
import "github.com/khaledmoayad/clawgo/internal/sdk"

engine, _ := sdk.NewQueryEngine(sdk.Config{
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    Model:  "claude-sonnet-4-20250514",
})

events := engine.Ask(ctx, "Explain this codebase")
for event := range events {
    switch event.Type {
    case sdk.EventTextDelta:
        fmt.Print(event.Text)
    case sdk.EventToolUse:
        fmt.Printf("[Using %s]\n", event.ToolName)
    }
}
```

---

## Config Compatibility

ClawGo reads and writes the same `~/.claude/` directory structure as the TypeScript version:

```
~/.claude/
  .config.json          Global config
  .credentials.json     API keys
  settings.json         User settings
  projects/<hash>/      Session storage (JSONL)
  memory/               Cross-session memories
  plugins/              Installed plugins
```

Your existing Claude Code configuration works out of the box.

---

## TUI Features

Built with the [Charm](https://charm.sh) ecosystem:

- **Bubble Tea** — Interactive REPL with streaming display
- **Glamour** — Markdown rendering with syntax highlighting
- **Lip Gloss** — Styled terminal output with color degradation
- **Chroma** — Code syntax highlighting
- Vim keybinding mode (`/vim` to toggle)
- Custom keybindings (config-driven)
- File diff views with color-coded unified diffs
- Virtual scrolling for large content
- Permission prompt dialogs (y/n/a)
- Spinner indicators during API calls

---

## Building

```bash
# Build
make build

# Build static binary (default)
CGO_ENABLED=0 go build -o clawgo ./cmd/clawgo/

# Run tests
make test

# Run tests with race detector
go test -v -race ./...

# Cross-compile
GOOS=darwin GOARCH=arm64 go build -o clawgo-darwin-arm64 ./cmd/clawgo/
GOOS=linux GOARCH=amd64 go build -o clawgo-linux-amd64 ./cmd/clawgo/
GOOS=windows GOARCH=amd64 go build -o clawgo.exe ./cmd/clawgo/
```

---

## Test Suite

```
55 packages | 738 tests | 0 failures
```

Every package has tests. Core tools have integration tests with real filesystem and subprocess operations.

---

## How It Was Built

ClawGo was built autonomously by Claude (Opus 4.6) using the [GSD workflow system](https://github.com/anthropics/claude-code). The entire project — from research through 7 phases of implementation — was executed via parallel subagent orchestration:

- **7 phases**, **37 plans**, **~80 tasks**
- Research agents analyzed the TypeScript source for behavioral specs
- Planner agents created detailed execution plans with acceptance criteria
- Executor agents implemented code in parallel worktrees
- Verifier agents checked goal achievement against requirements
- Plan checker agents validated plans before execution

The full planning artifacts (ROADMAP, CONTEXT, RESEARCH, PLAN, SUMMARY, VERIFICATION files) are preserved in the development repository.

---

## License

This project is a clean-room reimplementation. See [IMPORTANT NOTICE](#important-notice) above.

---

<div align="center">
  <sub>Built with Go and Claude by <a href="https://github.com/khaledmoayad">Khaled Moayad</a></sub>
</div>
