# myIntern — orchestration engine

The Go control plane for myIntern: a stateful, Postgres-backed build graph that
drives headless Claude through a gated pipeline. This repo implements the **full roadmap (Phases 0–8)** from
[`docs/superpowers/plans/2026-09-02-myintern-orchestration-spine.md`](docs/superpowers/plans/2026-09-02-myintern-orchestration-spine.md).

## Prerequisites

- **Go 1.23+**
- **Docker** (for the local Postgres)

## Test it (one command)

```bash
make test
```

That starts Postgres (`docker compose up -d --wait`), applies migrations, and
runs the full suite against real Postgres. First run pulls the `postgres:16`
image and downloads Go modules, so give it a minute.

## Local services (docker compose)

| Service | URL / port | What |
|---|---|---|
| Postgres | `localhost:5433` | platform state (RunState) |
| Valkey | `localhost:6380` | cache / sessions |
| MinIO | `localhost:9000` API, `9001` console | S3 storage (`uploads` bucket) |
| Dozzle | `http://localhost:8888` | live container logs |

```bash
make up        # bring the whole stack up
```

## See the UI

```bash
make seed      # create a demo run
make serve     # UI + API at http://localhost:7788
make desktop   # native desktop app (Tauri) — run `make serve` first
```

Open http://localhost:7788 — a Zinc-dark UI (per `docs/sample.html`) showing the
run list, the build graph nodes, and a live color-coded event log, all read from
Postgres via `GET /api/runs` and `GET /api/runs/{id}`.

Individual pieces:

```bash
make up        # just start Postgres
make migrate   # apply migrations
make down      # stop Postgres
```

To run tests yourself without make:

```bash
docker compose up -d --wait
go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations \
  postgres "postgres://myintern:dev@localhost:5433/myintern" up
TEST_DATABASE_URL="postgres://myintern:dev@localhost:5433/myintern" \
  go test -p 1 -count=1 ./...
```

> Tests run **serially** (`-p 1`): the store/queue/events/worker suites are
> integration tests sharing one Postgres, like the real build pipeline.

## What's here (the spine)

| Package | Responsibility |
|---|---|
| `internal/config` | env config (DB URL, bounded concurrency) |
| `internal/store` | Postgres `runs`/`nodes`/`events` + connection |
| `internal/events` | typed, correlated event logger (Postgres + slog) |
| `internal/queue` | `SKIP LOCKED` job claim + dependency gating |
| `internal/contract` | Claude output-contract parse + approval gate |
| `internal/claude` | Runner interface + fake + `claude -p` exec |
| `internal/gate` | RED-gate verdict (accept only genuine test failures) |
| `internal/worker` | `RunOnce` — claim → RED-gate → implement → record; retry + checkpoint interrupt |
| `internal/orchestrator` | `Tick`/`RunToCompletion` — drive the graph, pause on checkpoints, **resume** |
| `internal/capability` | idea → providers (voice → ElevenLabs) via a keyword registry |
| `internal/docs` | acquire → distill → **version-keyed cached briefs**; `BundleFor` builds the docs slice |

Phase 3 adds the doc-centric pipeline: capability detection, a `doc_briefs`
cache (distill once per `provider@version`, reuse everywhere), and a worker
**prompt-assembly seam** (`Deps.BuildPrompt`) that grounds a task on distilled
briefs instead of raw docs — the path a "build me a voice agent" request takes.

Phase 4 adds template integration: `internal/scaffold` (copy template + run its
initializer), `internal/gate` Node test-runner (classify `npm test` → RED-gate
Outcome), `internal/guardrails` (load CLAUDE.md + recipe), and `internal/prompt`
(assemble the layered bundle + output contract).

Phase 5 ships the optional **agent kit** at [`templates/agent-kit/`](templates/agent-kit/):
a provider-agnostic agent loop, conversation sessions, and an ElevenLabs voice
adapter (Vercel AI SDK), with an offline `node --test` self-check.

Phase 6 adds `internal/router` — the **auto-mode policy**: classify each task into
a model tier + autonomy level, escalating security/irreversible work to a
checkpoint and forcing one when the token budget is exhausted.

Phase 7 is the **UI** (see above). Phase 8 ships the remaining template assets:
[`templates/storage-kit/`](templates/storage-kit/) (S3/MinIO),
[`templates/deploy/`](templates/deploy/) (prod Dockerfile + Terraform), and
[`templates/swagger-kit/`](templates/swagger-kit/) (OpenAPI builder).

## Test everything

```bash
make test                                   # Go suite (16 packages) vs real Postgres
for k in agent-kit storage-kit swagger-kit; do (cd templates/$k && node --test); done
```

Phase 2 adds: `store.CreateGraph` (materialize a task graph with deps), retry
policy, a `checkpoints` table with raise/resolve, and the orchestrator loop that
runs a dependency-ordered graph to completion and resumes after a checkpoint.

## Design docs

- Idea / stack / architecture: [`docs/`](docs/)
- UI source of truth: [`docs/sample.html`](docs/sample.html)
- Full plan + roadmap: [`docs/superpowers/plans/`](docs/superpowers/plans/)
