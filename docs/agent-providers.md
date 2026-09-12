# Agent providers

myAudit drives a local CLI agent harness for QA and dev-fix nodes. Two providers are supported:

| Provider | CLI | Auth | Settings key |
|----------|-----|------|--------------|
| **Claude Code** (default) | `claude` | `claude login` | `agent_provider: "claude"` |
| **OpenCode** | `opencode` | `opencode auth login` | `agent_provider: "opencode"` |

## Resolution order

1. `AGENT_PROVIDER=claude|opencode` (env override for CI/dev)
2. SQLite settings `agent_provider`
3. Default: `claude`

## Model selection

**Claude** — `model_tier` in settings (`haiku-4.5`, `sonnet-5`, `opus-4.8`), overridden by `CLAUDE_MODEL`.

**OpenCode** — `opencode_model` in settings (e.g. `anthropic/claude-haiku-4-5`), overridden by `OPENCODE_MODEL`.

## Mode mapping

| myAudit mode | Claude | OpenCode |
|--------------|--------|----------|
| `ReadOnly` (map, analysis) | read-only tool policy | `--agent plan` |
| `Live` (QA, dev fix, chat) | live tool policy | `--agent build --auto` |

## OpenCode JSON stream

`opencode run --format json` emits one JSON object per line on stdout:

| `type` | Purpose | Maps to |
|--------|---------|---------|
| `text` | Assistant text chunks | `Result.Summary` (concatenated) |
| `tool_use` | Completed tool call | `onStep` via `part.state.title` |
| `step_finish` | Turn complete (`reason: "stop"`) | `Result.OK`, `CostUSD`, `Tokens` |
| `error` | Fatal error | `Result.OK=false`, `Result.Err` |

Unlike Claude Code's envelope, OpenCode has no single `result` event — the parser treats `step_finish` with `reason: "stop"` as success, or falls back to accumulated text if the process exits cleanly.

## v1 limitations

- `AGENT_ISOLATE=1` applies to Claude only; OpenCode always runs in the workspace directory.
- No per-run provider override (project settings only).
- No OpenCode session resume (`--continue` deferred).

## Environment variables

```bash
AGENT_PROVIDER=opencode      # force provider
CLAUDE_BIN=claude            # Claude CLI path
OPENCODE_BIN=opencode        # OpenCode CLI path
CLAUDE_MODEL=...             # Claude model override
OPENCODE_MODEL=provider/model # OpenCode model override
```
