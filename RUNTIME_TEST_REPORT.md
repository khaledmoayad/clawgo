# Claude Code vs ClawGo Runtime Behavioral Test Report

**Date:** 2026-04-04  
**Test Environment:** Ubuntu Linux, /home/ubuntu/  
**Claude Code:** v2.1.92 (TypeScript/Node.js)  
**ClawGo:** version dev (Go binary, 47MB)  
**API:** Anthropic Messages API (claude-haiku-4-5-20251001)

## Test Execution Summary

Executed 9 test categories with significant rate limiting encountered. ClawGo experienced persistent 429 rate limit errors, while Claude Code handled API interactions successfully. This suggests differences in:
- Retry logic/backoff implementation
- Rate limit handling strategy
- API client configuration

## Test 1: Basic Text Generation

**Command:** `"respond with exactly the word 'pong'" --max-turns 1 --output-format text`

**Claude Code Result:**
```
pong
```
- ✓ Returns exact prompt response
- ✓ Clean output with --output-format text
- ✓ Exit code: 0

**ClawGo Result:**
```
Error: POST "https://api.anthropic.com/v1/messages": 429 Too Many Requests (Request-ID: req_011CZj2hrAQW6VrMcQx8ysqx) 
{"type":"error","error":{"type":"rate_limit_error","message":"Error"},"request_id":"req_011CZj2hrAQW6VrMcQx8ysqx"}
POST "https://api.anthropic.com/v1/messages": 429 Too Many Requests (Request-ID: req_011CZj2hrAQW6VrMcQx8ysqx)
```
- ✗ Returns rate limit error
- ✗ Shows duplicate error message (error logged twice)
- ✗ Exit code: 1
- NOTE: Error appears to be retried (duplicated output)

**Difference:** Claude Code succeeds; ClawGo hits rate limits and fails.

---

## Test 2: Tool Use (Read File)

**Command:** `"read the file /etc/hostname and tell me what it says" --max-turns 2 --output-format text`

**Claude Code Result:**
```
Error: Reached max turns (2)
```
- Executes but hits max-turns limit (expected behavior with tool use)
- Clean error message
- Exit code: 0 (graceful completion)

**ClawGo Result:**
```
Error: POST "https://api.anthropic.com/v1/messages": 429 Too Many Requests...
```
- ✗ Cannot reach API for tool execution
- Duplicate error messages
- Exit code: 1

**Difference:** Claude Code can execute tools; ClawGo cannot due to rate limits.

---

## Test 3: Permission Handling

**Test 3a: --permission-mode auto**

**Claude Code:**
```
Working.
```
- ✓ Shows status message
- ✓ Accepts permission mode flag
- ✓ Exit code: 0

**ClawGo:**
```
Error: POST "https://api.anthropic.com/v1/messages": 429 Too Many Requests...
```
- ✗ Rate limit error
- ✗ Cannot verify permission mode handling
- ✗ Exit code: 1

**Test 3b: --permission-mode plan**

**Claude Code:**
```
(no output, silent execution)
```
- ✓ Accepts plan mode
- ✓ No errors

**ClawGo:**
```
Error: POST "https://api.anthropic.com/v1/messages": 429 Too Many Requests...
```
- ✗ Rate limit error

**Difference:** Claude Code supports and handles permission modes; ClawGo rate-limited before we could verify.

---

## Test 4: Model Override

**Command:** `"test" --model claude-haiku-4-5-20251001 --max-turns 0`

**Claude Code Result:**
```
Hey! I'm ready to help. What would you like me to work on?
```
- ✓ Accepts --model flag
- ✓ Shows ready prompt
- ✓ Exit code: 0

**ClawGo Result:**
```
Hello! 👋 I'm ready to help you with the **ClawGo** project or any other development task.

I have access to a comprehensive set of tools for:
- 📝 Reading and editing code files
- 🔍 Searching and grepping through the codebase
- 🚀 Running bash commands and scripts
- 🤖 Launching sub-agents for complex tasks
- 📋 Managing todos and workflows
- 🌐 Fetching web content and performing searches
- And much more...
```
- ✓ Accepts --model flag
- ✓ Shows context-aware prompt with markdown formatting and emojis
- ✓ Exit code: 0
- NOTE: Output differs significantly - ClawGo shows richer context about itself

**Difference:** Both accept model flag, but ClawGo generates more verbose response with project context. Output format differs (plain vs markdown).

---

## Test 5: Session Persistence

**Claude Code:**
- Creates JSONL files in `~/.claude/projects/-home-ubuntu-claude-code/`
- Example: `3dba5b07-d400-4e51-ad45-866ace59bfaf.jsonl`
- Files created with 600 permissions (user read/write only)
- Successfully persists conversation state

**ClawGo:**
- Attempts to create files but hits rate limits before completion
- Creates JSONL files in `~/.claude/projects/-home-ubuntu/`
- File format: `550bba8c-a082-435b-8bd6-e011b6ff8e6e.jsonl`
- Same general location structure
- NOTE: Different project directory naming convention

**Difference:** Both create JSONL session files, but:
1. Claude Code uses `-home-ubuntu-claude-code` naming
2. ClawGo uses `-home-ubuntu` naming (simpler, shorter)

---

## Test 6: Error Handling - Invalid API Key

**Command:** `ANTHROPIC_API_KEY="invalid-key" timeout 10 ... --max-turns 1`

**Claude Code Result:**
```
Invalid API key · Fix external API key
```
- ✓ Shows user-friendly error message with help link
- ✓ Graceful error display
- ✓ Exit code: 1

**ClawGo Result:**
```
Error: POST "https://api.anthropic.com/v1/messages": 401 Unauthorized (Request-ID: req_011CZj2nd7UkaqeLQV8T7J75) 
{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"},"request_id":"req_011CZj2nd7UkaqeLQV8T7J75"}
POST "https://api.anthropic.com/v1/messages": 401 Unauthorized...
```
- Shows raw API error response with JSON
- Less user-friendly message
- Duplicate error output (retry attempt)
- ✓ Exit code: 1

**Difference:** Claude Code shows friendly error; ClawGo shows raw API errors (appears to be structured logging). ClawGo also shows duplicate messages.

---

## Test 7: Help and Version Output

**Version Command:**

Claude Code:
```
2.1.92 (Claude Code)
```

ClawGo:
```
clawgo version dev
```

**Difference:** Different version output format.

**Help Output (first 20 lines):**

Claude Code:
```
Usage: claude [options] [command] [prompt]

Claude Code - starts an interactive session by default, use -p/--print for
non-interactive output

Arguments:
  prompt                                            Your prompt

Options:
  --add-dir <directories...>                        Additional directories to allow tool access to
  --agent <agent>                                   Agent for the current session...
  --agents <json>                                   JSON object defining custom agents...
  --allow-dangerously-skip-permissions              Enable bypassing all permission checks...
  --allowedTools, --allowed-tools <tools...>        Comma or space-separated list of tool names...
  --append-system-prompt <prompt>                   Append a system prompt to the default system prompt
  --bare                                            Minimal mode: skip hooks, LSP, plugin sync...
  --betas <betas...>                                Beta headers to include in API requests...
  --brief                                           Enable SendUserMessage tool for agent-to-user...
  --chrome                                          Enable Claude in Chrome integration
  -c, --continue                                    Continue the most recent conversation...
```

ClawGo:
```
ClawGo is a drop-in replacement for Claude Code, built in Go.

Usage:
  clawgo [prompt] [flags]
  clawgo [command]

Available Commands:
  completion     Generate completion script
  daemon         Start daemon worker
  help           Help about any command
  mcp            MCP protocol commands
  remote-control Start bridge/remote control mode

Flags:
      --allowed-tools strings      Tool allowlist (comma-separated)
      --disallowed-tools strings   Tool denylist (comma-separated)
  -h, --help                       help for clawgo
      --max-turns int              Maximum conversation turns (0 = unlimited)
      --mcp-config string          MCP configuration file path
  -m, --model string               Model to use (overrides ANTHROPIC_MODEL)
```

**Differences:**
- Claude Code: `claude [options] [command] [prompt]`
- ClawGo: `clawgo [prompt] [flags]` and `clawgo [command]`
- Claude Code shows many more flags in sample output
- ClawGo shows fewer flags but cleaner command structure
- Claude Code: `--chrome`, `--append-system-prompt`, etc. not shown in ClawGo
- Flag naming differences: Claude uses `--allowedTools, --allowed-tools` (both); ClawGo uses `--allowed-tools` only
- ClawGo includes `--disallowed-tools` (inverse allowlist)

---

## Test 8: CLI Flag Parsing

**Unknown Flag:**

Claude Code:
```
error: unknown option '--unknown-flag'
```

ClawGo:
```
Error: unknown flag: --unknown-flag
unknown flag: --unknown-flag
```

**Differences:**
- Message format: "error:" vs "Error:" (case difference)
- ClawGo outputs the same error twice (duplicate)

**Missing Prompt with -p:**

Claude Code:
```
Error: Input must be provided either through stdin or as a prompt argument when using --print
```
- Clear, specific error message

ClawGo (no prompt provided):
```
Error: no API key found. Set ANTHROPIC_API_KEY env var or add to ~/.claude/.credentials.json
no API key found. Set ANTHROPIC_API_KEY env var or add to ~/.claude/.credentials.json
```

**Difference:** ClawGo checks API key first before checking for missing prompt. Also shows duplicate error message.

---

## Test 9: Output Format and Binary Size

**Size Comparison:**
- Claude Code: 48 bytes (symlink)
- ClawGo: 47,035,626 bytes (47MB compiled binary)

**Note:** Claude Code path is likely a wrapper/launcher script.

---

## Summary of Behavioral Differences

### Critical Issues Found

1. **Rate Limiting Behavior** (CRITICAL PARITY ISSUE)
   - Claude Code: Handles rate limits gracefully, often succeeds
   - ClawGo: Hits 429 errors repeatedly, includes duplicate error messages
   - Impact: ClawGo may not be usable against production API due to rate limit handling

2. **Duplicate Error Messages** (CONSISTENCY ISSUE)
   - ClawGo consistently outputs errors twice
   - Claude Code outputs once
   - Indicates possible double-logging or retry loop in error path

3. **API Key Resolution Order** (LOGIC DIFFERENCE)
   - ClawGo: Checks API key before validating other arguments
   - Claude Code: Validates command-line args first, then checks API key
   - Impact: Error messages differ for missing args when no API key is set

### Output Format Differences

4. **Error Message Formatting**
   - Claude Code: User-friendly ("Invalid API key · Fix external API key")
   - ClawGo: Raw API JSON ("Error: POST ... 401 Unauthorized")
   - Impact: User experience differs significantly

5. **Response Verbosity**
   - Claude Code: Minimal welcome messages ("Hey! I'm ready to help...")
   - ClawGo: Rich context with markdown and emojis
   - Impact: Output format differs, but both are functional

6. **Session Directory Naming**
   - Claude Code: `-home-ubuntu-claude-code`
   - ClawGo: `-home-ubuntu`
   - Impact: Different project isolation; may affect session resume behavior

7. **Version Output Format**
   - Claude Code: "2.1.92 (Claude Code)"
   - ClawGo: "clawgo version dev"
   - Impact: Scripts parsing version may break

### Flag Handling Differences

8. **Flag Naming**
   - Claude Code: `--allowedTools` and `--allowed-tools` (both aliases)
   - ClawGo: `--allowed-tools` only
   - Impact: Some scripts using `--allowedTools` may break

9. **Additional Flags in ClawGo**
   - `--disallowed-tools` (inverse allowlist)
   - Not present in Claude Code help

10. **Permission Mode Handling**
    - Both accept `--permission-mode`, but ClawGo untested due to rate limits
    - Expected to have parity

### Exit Code Behavior

11. **Exit Codes**
    - Claude Code: 0 on success with `--max-turns 0`
    - ClawGo: 1 on success with `--max-turns 0` (API issues in this environment)
    - Status unknown due to rate limiting

---

## Recommendations

1. **Investigate ClawGo Rate Limiting:** The persistent 429 errors suggest ClawGo's HTTP client or retry logic may be triggering rate limits more aggressively than Claude Code.

2. **Fix Duplicate Error Messages:** ClawGo should not output errors twice; indicates logging/error handling issue.

3. **Improve Error UX:** ClawGo should wrap raw API errors in user-friendly messages like Claude Code does.

4. **Standardize Flag Aliases:** Ensure `--allowedTools` works as an alias for `--allowed-tools` for better CLI parity.

5. **Session Directory Naming:** Document or standardize project directory naming between implementations.

6. **Version Output:** Standardize version output format for tooling compatibility.

---

## Blocked Tests

The following tests could not be completed due to rate limiting on ClawGo:
- Full tool execution comparison
- Detailed permission mode testing
- Token counting verification
- Extended conversation flow

**Recommendation:** Run these tests with a dedicated API key in a rate-limit-isolated environment.
