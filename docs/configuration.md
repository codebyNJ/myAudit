# Configuration

[← Documentation home](README.md)

myAudit reads config from environment variables and SQLite settings (via the UI or API). No API keys are stored — auth comes from your local CLI.

## Environment variables

Copy [`.env.example`](../.env.example) to `.env` and uncomment what you need.

| Variable | Default | Purpose |
|----------|---------|---------|
| `MYAUDIT_DB` | `./myaudit.db` | SQLite database path |
| `PORT` | `7788` | HTTP listen port |
| `REAL_CLAUDE` | unset | Set `1` for real agent (`make run` sets this) |
| `AGENT_PROVIDER` | `claude` | Force provider: `claude` or `opencode` |
| `CLAUDE_MODEL` | Haiku tier | Overrides in-app Claude model |
| `CLAUDE_BIN` | `claude` | Path to Claude Code CLI |
| `OPENCODE_MODEL` | — | Overrides in-app OpenCode model (`provider/model`) |
| `OPENCODE_BIN` | `opencode` | Path to OpenCode CLI |
| `MAX_CONCURRENT_CLAUDE` | `1` | Max parallel agent processes |
| `RUN_PACE_MS` | — | Milliseconds between run-loop ticks |
| `AGENT_ISOLATE` | unset | `1` = run Claude agents in a container |
| `AGENT_IMAGE` | `myaudit-sandbox` | Docker image when isolating |

Resolution order for provider: `AGENT_PROVIDER` env → SQLite `agent_provider` → `claude`.

## In-app settings (`PUT /api/settings`)

| Key | Values | Purpose |
|-----|--------|---------|
| `agent_provider` | `claude`, `opencode` | Active CLI harness |
| `model_tier` | `haiku-4.5`, `sonnet-5`, `opus-4.8` | Claude model (when provider is Claude) |
| `opencode_model` | e.g. `anthropic/claude-haiku-4-5` | OpenCode model string |

Env overrides always win over settings for provider and model.

## Model recommendations

| Tier | Best for | Relative cost |
|------|----------|---------------|
| Haiku 4.5 | Fast scans, small repos | Lowest |
| Sonnet 5 | Balanced QA + fixes (recommended) | Medium |
| Opus 4.8 | Deepest analysis | Highest |

A full small-repo audit on Haiku is typically ~$1–3. Set `CLAUDE_MODEL=claude-sonnet-5` or pick Sonnet in Settings for senior-grade depth.

## See also

- [Agent providers](agent-providers.md)
- [Safety](safety.md) — `AGENT_ISOLATE` and live-mode risks
- [HTTP API — Settings](api.md#settings)
