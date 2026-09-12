# Architecture

[← Documentation home](README.md)

myAudit turns a codebase into a self-driving audit-and-repair board. The engine is a dependency graph of nodes stored in SQLite; a run loop claims ready nodes and executes them by type. Nodes spawn more nodes at runtime, so the graph (and kanban board) grows as work is discovered.

## Audit graph

```
import → map ─┬→ qa(module A) ─→ bug… (dev fixes, blocked until QA)
              ├→ qa(module B) ─→ bug…
              └→ qa(module C) ─→ bug…
```

| Node | Agent? | Role |
|------|--------|------|
| **import** | No ($0) | Copy repo into `runs/<run-id>/`, git baseline |
| **map** | Read-only | Product map in notes; spawn one `qa` per module |
| **qa** | Live | Install deps, run tests, file bug tickets with reproduce steps |
| **bug** | Live | Root-cause fix, commit, independent regression |

## Scheduling: QA over dev

The queue claims ready nodes with an atomic SQLite update. Claim order is `import > map > qa > bug`, so discovery drains before fixes begin. Bug tickets depend on their module's `qa` node — `PromoteReady` only marks a node `ready` when all deps are `done`.

## Agent providers

`internal/worker` calls a provider-neutral `Agent` interface. Two CLI harnesses implement it:

| Package | CLI | Notes |
|---------|-----|-------|
| `internal/agent/claude` | `claude -p` | JSON envelope, optional Docker isolate |
| `internal/agent/opencode` | `opencode run --format json` | JSONL stream, workspace-only |

Shared types live in `internal/agent` (`Result`, `Mode`, `PolicyFor`). Resolution happens in `internal/api/agentresolve.go` (env → settings → default).

See [Agent providers](agent-providers.md) for mode mapping and settings.

## Tool policies

| Mode | Claude tools | OpenCode agent |
|------|--------------|----------------|
| **ReadOnly** | Read, Glob, Grep | `--agent plan` |
| **Live** | + Write, Edit, Bash | `--agent build --auto` |

Live mode lets QA run the product. Agent env scrubs `PORT`/`MYAUDIT_DB`. `AGENT_ISOLATE=1` jails Claude in a container.

## Data model

One SQLite file; schema embedded in `internal/store/schema.sql` (WAL, single connection).

| Table | Purpose |
|-------|---------|
| `runs` | One row per audit |
| `nodes` | Graph: type, status, deps, input/output JSON |
| `events` | Activity log + chat transcript |
| `notes` | Running markdown report |
| `checkpoints` | Human-in-the-loop blocks |
| `file_reviews` | Per-file accept/reject |
| `settings` | Project JSON (`agent_provider`, models, …) |

The entire run is inspectable via the JSON API at any instant.

## Package layout

| Package | Responsibility |
|---------|----------------|
| `internal/config` | Env config (DB path, pacing) |
| `internal/store` | SQLite persistence + embedded schema |
| `internal/events` | Typed event logger |
| `internal/queue` | Atomic claim + dependency gating |
| `internal/sandbox` | Per-run workspace (copy, diff, reclaim) |
| `internal/agent` | Shared types; `claude/` and `opencode/` runners |
| `internal/worker` | Node dispatch: import, map, qa, bug |
| `internal/api` | HTTP API, run loop, embedded web UI |
| `web` | React + Vite frontend |
| `desktop` | Tauri shell |

## Frontend

React + Vite + TypeScript, embedded via `//go:embed` (`internal/api/web/dist`). `make ui-build` syncs from `web/dist`. During development the server also serves from disk when built assets exist (see [Testing](testing.md)).

The board polls every 2s; the editor follows files changed by fixes. Desktop loads the same UI from `http://localhost:7788`.

## See also

- [HTTP API](api.md)
- [Safety](safety.md)
- [Testing](testing.md)
