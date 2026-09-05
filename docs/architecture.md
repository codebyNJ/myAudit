# myAudit — Architecture

myAudit turns a codebase into a self-driving audit-and-repair board. The engine
is a dependency graph of nodes stored in SQLite; a single run loop claims ready
nodes and executes them by type. Nodes spawn more nodes at runtime, so the graph
(and the kanban board that renders it) grows as work is discovered.

## The graph

```
import → map ─┬→ qa(module A) ─→ bug… (dev fixes, blocked until QA)
              ├→ qa(module B) ─→ bug…
              └→ qa(module C) ─→ bug…
```

- **import** (`$0`) — copies the target repo into `runs/<run-id>/` (skipping
  VCS + heavy build dirs, recreating symlinks) and lays a single git baseline
  commit so every later diff starts clean. The original repo is never touched.
- **map** (read-only agent) — writes a product map to `notes`, then scans the
  workspace into modules (top-level source dirs, descending one level into
  `src`/`app`/`apps`/`packages`) and **spawns one `qa` node per module**.
- **qa** (live agent) — the priority role. Installs deps, runs the suite/build,
  exercises the module, and returns findings as a JSON array. Each finding
  becomes a `bug` ticket **depending on this qa node**, so a fix can only start
  after the module's QA is done.
- **bug** (live agent) — reads the ticket, applies a minimal root-cause fix,
  commits, then the engine runs an **independent** regression (deps ensured,
  suite run). Green → `done` (auto-closed); genuine failure → `failed`; a fix
  that can't be verified or produced no diff → `in_review`. All flagged on the
  board and logged to `notes`.

## Scheduling: QA over dev

The queue claims one ready node at a time with an atomic
`UPDATE … WHERE id=(SELECT … ORDER BY … LIMIT 1) RETURNING` (SQLite's
statement-level write lock makes it safe — no `SKIP LOCKED` needed). Claim order
is `import > map > qa > bug`, tie-broken by creation time, so the board drains all
discovery before any fix begins. Bug tickets are additionally dep-gated on their
module's qa node via `PromoteReady` (a `pending` node becomes `ready` only once
every dep is `done`).

## Tool policies (the Claude Code seam)

`internal/agent` runs `claude -p --output-format json` with an allow/deny tool
policy chosen per node:

| mode | tools | used by |
|---|---|---|
| **read-only** | Read, Glob, Grep | map overview, chat questions |
| **write** | + Write/Edit/MultiEdit | legacy testgen |
| **live** | + Bash | qa, dev fix, chat commands |

Live mode is what lets QA actually run the product. The agent's environment is
scrubbed of `PORT`/`MYAUDIT_DB` so a target dev server can't collide with — or
see — the myAudit server. `AGENT_ISOLATE=1` runs the whole thing in a container.

## Data model

One SQLite file, schema embedded in `internal/store/schema.sql` and applied on
open (`CREATE TABLE IF NOT EXISTS`; WAL; single connection). Key tables:

- **runs** — one per audit.
- **nodes** — the graph. `type`, `status`, `deps` (JSON array of node ids),
  `input_snapshot` (per-node spec / ticket JSON), `output` (result JSON).
  Bug tickets are `type='bug'` nodes carrying title/file/severity/priority/detail/tags.
- **events** — the correlated activity log (also the chat transcript, as
  `chat.user` / `chat.assistant`).
- **notes** — the running markdown report.
- **checkpoints**, **file_reviews** — human-in-the-loop + per-file accept/reject.

Because state *is* the database, the entire run is inspectable at any instant via
the JSON API (`GET /api/runs/{id}`, `/board`, `/notes`).

## Frontend

React + Vite + TypeScript, embedded into the Go binary via `//go:embed`
(`internal/api/web/dist` — synced from the top-level `web/dist` Vite output by
`make`). The board polls every 2s; the editor live-refreshes the open file and
follows whichever file a fix just changed. The desktop app is a thin Tauri shell
that loads the same UI from `http://localhost:7788`.
