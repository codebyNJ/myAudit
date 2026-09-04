# myAudit — a minimal agentic audit IDE

Point it at a codebase and it drives headless Claude Code through a small graph:
**import → understand → testgen → verify → review**. It reads the code, writes
down the flows it finds, generates a test for one of them, runs it, and reports
bugs / best-practice misses — all tracked on a kanban board with a live event
log.

It's a stripped, inverted fork of the myIntern orchestration engine: same spine
(a Postgres-free build graph driving `claude -p`), but where myIntern *builds*
apps, myAudit *interrogates* an existing one.

## What's minimal about it

- **One binary + one SQLite file.** No Docker, no Postgres, no migration tool —
  the schema is embedded and applied on open.
- **Your Claude Code auth.** The agent nodes shell out to the `claude` CLI; no
  API keys, no OpenRouter. If it's not Claude Code, it's nothing.
- **Read-only by default.** `understand` and `review` run with a read-only tool
  policy — they can never mutate the imported code. Only `testgen` writes, and
  only a new test file, in an isolated copy of your repo.

## Run it

Prereqs: **Go 1.23+**, the **`claude` CLI** (logged in), and **git**.

```bash
make run     # UI + API at http://localhost:7788, REAL Claude Code agent
make dev     # same, but the $0 stub agent (no tokens — UI/graph only)
make test    # full Go suite (temp SQLite per test; no services)
make desktop # native Tauri shell (run `make run` in another shell first)
```

Start an audit:

```bash
curl -XPOST localhost:7788/api/runs -d '{"repo_path":"/abs/path/to/repo"}'
```

Then open the UI, or read the run over the API: `GET /api/runs/{id}`,
`/board`, `/notes`.

## The pipeline

| node | model? | does |
|---|---|---|
| `import` | no ($0) | copy the target repo into an isolated workspace + git baseline |
| `understand` | claude (read-only) | summarize the code + identify flows → written to notes |
| `testgen` | claude | write one test for a key flow (new file; source untouched) |
| `verify` | no ($0) | run the tests. Pass = behavior confirmed; **fail = a finding** |
| `review` | claude (read-only) | flag bugs / best-practice misses → findings + notes |

`verify` inverts myIntern's RED gate: the code already exists, so a failing test
is a *finding*, not a step to repair.

## Layout

| Package | Responsibility |
|---|---|
| `internal/config` | env config (SQLite path, bounded concurrency) |
| `internal/store` | SQLite `runs`/`nodes`/`events`/`checkpoints`/`notes`/`file_reviews` + embedded schema |
| `internal/events` | typed, correlated event logger |
| `internal/queue` | atomic node claim + dependency gating |
| `internal/sandbox` | per-run working copy of the imported repo (git baseline, diff, run) |
| `internal/agent` | `claude -p` runner + tool policies (default / read-only) |
| `internal/worker` | one node of the audit graph (dispatch by type) |
| `internal/api` | JSON API + run loop + embedded web UI |
| `web` / `desktop` | React UI and its Tauri desktop shell |

## Configuration

All optional — see [`.env.example`](.env.example). `MYAUDIT_DB` sets the SQLite
path (default `./myaudit.db`); `CLAUDE_MODEL` picks the agent model (default
Haiku); `RUN_PACE_MS` paces the run loop.
