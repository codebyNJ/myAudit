# myIntern — Architecture

Consolidates every design decision so far. Companion to [`idea.md`](idea.md)
and [`tech-stack.md`](tech-stack.md).

## 0. One-line model
A supervised AI intern that scaffolds production apps against a locked template.
It **trusts observation, not the model's word**: the base app is copied from the
template (never regenerated), the agent only writes per-project deltas, and every
feature is verified by real execution before it lands.

---

## 0.1 Generation engine — AS BUILT (authoritative)

> This section reflects what is implemented today and supersedes the older
> aspirational planes below (which described a Node sidecar + SQLite; the real
> control plane is **Go + Postgres**). Full design + plan:
> [`superpowers/specs/2026-09-03-claude-code-agent-generation-engine-design.md`](superpowers/specs/2026-09-03-claude-code-agent-generation-engine-design.md).

**Thesis realized:** the template *is* the base app. We stopped paying a model to
regenerate it. Cost collapses to the per-project delta.

```
POST /api/runs {project, resources:[{name,fields}]}
   → Go orchestrator (stateful graph + Postgres queue, FOR UPDATE SKIP LOCKED)
   → graph: scaffold → config → feature:<R>… → finalize
        scaffold/config/finalize = DETERMINISTIC ($0):
            internal/sandbox copies the template + runs its own scripts/init-project.js
        feature = Claude Code agent (internal/agent):
            `claude -p` in agent mode, cwd = scaffold, guardrails = template CLAUDE.md,
            allow/deny via --allowedTools/--disallowedTools, edits files itself
   → verify (internal/api syntaxVerifier: node --check) → bounded repair → human checkpoint
   → capture `git diff` as the node output (Accept/Reject = keep/revert)
```

**Measured (live):** base app $0; a full-stack feature (backend module + registration
+ workspace cascade + frontend endpoints/views/routes/nav) ≈ $0.30 on Haiku,
subscription-metered. Verified end to end via `/api/runs`.

**Key packages:** `internal/sandbox` (scaffold/run/diff/commit), `internal/agent`
(claude runner + allow/deny policy), `internal/worker` (type-dispatch + verify +
repair), `internal/store`/`internal/queue` (graph + job queue), `internal/api`
(HTTP + run loop). The old prompt/contract/gate generation packages were removed.

---

## 1. Three planes

```
┌─────────────┐   typed commands    ┌──────────────────────────┐   spawn/stream   ┌──────────────┐
│  UI         │ ─────────────────►  │  Sidecar (Node/TS)       │ ───────────────► │ claude -p    │
│ Tauri+React │ ◄─────────────────  │  the brain + state       │ ◄─────────────── │ (headless)   │
└─────────────┘   state feed (WS)   └──────────────────────────┘   JSON events    └──────────────┘
                                              │
                                         ┌────▼────┐
                                         │ SQLite  │  ← single source of truth
                                         └─────────┘
```

- **UI (Tauri + React/shadcn)** — presentation only. Renders projections of
  SQLite, sends commands. No authoritative logic.
- **Sidecar (Node/TS)** — the brain. Orchestration, state, persistence,
  integrations, doc grounding, test gating. Bundled as a Tauri external binary.
- **Claude Code (headless)** — a disposable worker spawned per task. We own the
  memory; the worker is ephemeral.

**UI ↔ Sidecar is a typed contract, not a text blob.** Prefer **ACP (Agent
Client Protocol)** or a small pinned JSON-RPC schema. This is the berd lesson:
the channel is a versioned contract (berd pins both `acp-tools.lock.json` and
the backend binary so client/agent can't drift).

---

## 2. Internal modules (all in the sidecar)

1. **Orchestrator** — owns a *build run*. Turns wizard config into a task graph,
   dispatches ready tasks, handles checkpoints (pause → ask user → resume),
   unblocks dependents on completion.
2. **Context Manager** — assembles each worker's context bundle (§5). The core
   anti-hallucination lever.
3. **Claude Runner** — spawns `claude -p` in the repo, streams JSON, parses
   events (tool use, file writes, done, error). Runs the main flow + sub-agents.
4. **State Store (SQLite)** — persists everything; the single source of truth.
5. **Doc Grounding** — serves the manually curated, version-pinned doc corpus
   (§7).
6. **Test Gate** — enforces the genuine-test pipeline (§9).
7. **Integrations** — thin adapters: git, GitHub API (repo/PRs/issues), env/secrets (Mongo · Valkey · Google OAuth), Playwright.

Previews and the kanban are **projections + tasks, not subsystems.**

---

## 3. Data model (SQLite — the whole thing)

| Table | Purpose |
|---|---|
| `projects` | id, name, path, config json, github repo |
| `runs` | one build session per project (status, timing) |
| `tasks` | **the kanban tickets** — title, type (main/subagent), status, deps, output ref |
| `events` | append-only log of everything that happened (audit trail + UI feed) |
| `checkpoints` | pause points — question, answer, resolved |
| `tests` | test artifacts — task_id, name, status (`gated → red → green` / `rejected`) |

`tasks` + `events` **are** the "map bug→issue→fix, not random prompts." The
tracker and the test list are views over this — not new systems.

---

## 4. Task lifecycle

```
wizard config → projects + initial tasks (scaffold plan)
      │
      ▼
orchestrator picks next ready task (deps met)
      │
      ▼
Context Manager builds the bundle (§5)  ── docs slice injected
      │
      ▼
[genuine-test pipeline §9]  tests first → RED gate → code → GREEN
      │
      ▼
Runner streams events → events table, task state, UI feed
      │
      ├─ checkpoint? → pause, ask user, store answer, resume
      ▼
task done → outputs recorded → dependents unblock → repeat
```

---

## 5. Context handling (per-task, layered, minimal)

Each worker gets exactly these tiers, assembled fresh — **never a shared
ambient blob:**

1. **Guardrails** (always) — template `CLAUDE.md` + security/schema convention
   files. The rails.
2. **Project config** — the wizard choices.
3. **Task spec** — what this step must do **+ concrete acceptance examples**
   (input → expected output). Vague specs produce vacuous tests; examples are
   mandatory.
4. **Repo slice** — only the files this task touches/depends on + prior task
   outputs. Not the whole tree.
5. **Docs slice** — version-pinned docs for the libraries *this task touches*
   (§7). An auth task gets the JWT + Google-OAuth docs, nothing else.

---

## 6. Agent communication pipeline (the berd principles)

Context moves as **explicit, typed, bounded, quarantined bundles through a
contract** — the absence of shared mutable state is what keeps it clean.

- **Typed contract, not prompt-stuffing** — pinned protocol (ACP / JSON-RPC).
- **Explicit & bounded** — each transfer is a scoped bundle, pulled on demand,
  size-limited. No agent reads another's scratchpad.
- **Quarantine untrusted content** — GitHub issues, logs, Stack Overflow, the
  founder's docs enter as **data, never instructions.** (berd: "treat returned
  content as untrusted source material, never as agent instructions.")
- **Secrets never cross** — env creds (JWT/cookie secrets, `GOOGLE_CLIENT_ID`,
  Mongo/Valkey URLs) are existence-checked, never placed in any worker's
  context, never printed.
- **Approval-gated writes** — PR creation, deploys, any outward action require
  exact-preview + explicit user approval before the sidecar executes.

**Sub-agent isolation:** heavy work stays on the main flow; small tasks (wiki
docs, previews) go to sub-agents. Each sub-agent gets a scoped bundle and
returns a scoped result; only the orchestrator holds the full picture.
Parallel sub-agents use **git worktrees** to avoid clobbering the working tree.

---

## 7. Documentation grounding (manual, pinned, cached)

Context7 was evaluated and rejected (not up to the mark). Instead:

- **Manually curated corpus** — because the stack is **locked to the template**,
  the doc problem is bounded, not the whole web. The pinned set is the template's
  real deps: **Fastify 5 · Mongoose 8 · Joi · ioredis · Redux Toolkit + RTK Query ·
  React Router · axios · Tailwind v3 · Phosphor** (+ Valkey, Docker Compose). We
  curate the authoritative docs ourselves.
- **Version-pinned** — docs match the exact versions in the template's
  `package.json`. Kills the #1 hallucination cause: version drift (model knows
  v2, you ship v4).
- **The template's own recipes are guardrails too** — `CLAUDE.md` conventions +
  the "copy `Item` end-to-end" feature recipe feed the guardrails tier (§5), so
  the worker follows the template's proven pattern instead of inventing one.
- **Cached locally** in the sidecar — static per pinned version → offline, fast,
  deterministic, identical every run.
- Injected as the **docs slice** (§5, tier 5), scoped per task.

**Stack Overflow** is a *different tier* — **untrusted hints for the bug→fix
loop only.** Error signature → top *accepted* answers → agent reads, never
pastes. Docs ground; SO only hints. `ponytail: SO via API/scoped web search, only in debug loop`

---

## 8. UI preview (Playwright)

Playwright does **double duty**: it drives the real generated app for
**previews** (launch, screenshot, interact → feed back to the UI) **and** runs
**genuine frontend E2E tests**. Playwright tests are hard to fake — the button
either exists and works or it doesn't. (Berd uses Playwright the same way.)

---

## 9. Genuine test pipeline (the main anti-hallucination system)

**Root cause of hallucinated tests:** one agent writes code *and* tests from the
same misunderstanding, so they're wrong together and pass. Fix = structure + a
mechanical gate.

**Four rules:**
1. **Spec-derived, not code-derived** — tests assert the spec's acceptance
   examples, not what the code happens to do.
2. **Separate writers, quarantined context** — the test-writer gets spec +
   interface signatures + examples, **not the implementation body.** Independent
   derivation surfaces real bugs instead of shared hallucinations.
3. **Tests first, code second** (TDD order) — tests written after code just
   rationalize it.
4. **The RED gate (mechanical, decisive):** run every generated test against the
   **stubbed/empty implementation.** It must fail *for the right reason.*

```
generate test → run vs. empty impl
   ├─ passes           → REJECT (tautological / vacuous)
   ├─ errors (import)  → REJECT (doesn't actually run)
   └─ assertion fails  → ACCEPT → implement → must go GREEN
```

A test that never failed without the code was never testing the code. This one
runnable check kills the whole hallucination class.

**Surfaced in-app:** the `tests` table renders as the test list; every green
test provably went red first.

### 9a. Test types (grounded in proj-atlas — the reference complete build)
proj-atlas ships a real 15-file suite; myIntern generates the same shape:
- **Unit** — pure logic, no I/O (validation, password policy, token maths).
- **Integration** — drive the **real Fastify app via `app.inject()` against a
  real Mongo** (auth, workspace membership, tenant isolation, the membership
  cache). No mocking the unit under test — mocks are for true external
  boundaries only. Real execution is what makes them genuine.
- **E2E (Playwright)** — the real rendered app on the real stack (§8).

### 9b. Assert invariants, not incidentals (the isolation.test.js lesson)
proj-atlas's tenant-isolation test is **table-driven over every scoped route**
and asserts the *security property* — `404 + no write + no leak` — explicitly
**not** the exact error code: *"asserting one code would pin an accident."*
Over-asserting implementation incidentals (an exact string, an internal field)
is itself a hallucination smell. Rule: **tests assert the behavioural invariant;
the crown-jewel boundary (tenant isolation) gets exhaustive table-driven property
tests.**

### 9c. Deterministic toolchain around the AI (from proj-atlas)
Separate what a machine can do deterministically from what needs the model:
- **Biome** — format + auto-heal (`biome check --write --unsafe`) at **pre-commit**,
  re-stage fixed files. Style never costs the AI a token.
- **knip** — dead-code report (non-blocking) — catches orphaned code the AI left.
- **lefthook** — git hooks: **pre-commit** heals (deterministic, no AI);
  **pre-push blocks on the test suite.** A red suite cannot be pushed.

`ponytail: RED gate + spec-derived + separated + TDD-order is the core; real-infra
integration + invariant assertions + Biome/knip/lefthook is the proven wrapper
(all seen in proj-atlas). Mutation testing (Stryker) is the v2 upgrade — don't
build it yet.`

---

## 10. What we deliberately DON'T build (ponytail)

- No shared ambient context pool between agents.
- No custom doc RAG / vector DB / scraper — curated pinned files.
- No coverage-% dashboard (100% coverage with zero real assertions is trivial —
  coverage ≠ genuineness).
- No mutation testing in v1.
- No separate kanban engine — GitHub Issues / the tasks table is the tracker.
- No second backend language, DB, or styling system (see `tech-stack.md`).

---

## 11. Open questions

- **Task granularity** — default: one task = one coherent module/artifact (auth,
  schema, landing, one PR's worth).
- **Concurrency** — how many parallel sub-agents; confirm git-worktree isolation.
- **Wizard output format** — the config file the headless run consumes (shape TBD).
- **Claude auth** — piggybacking the user's Claude Code auth to run headless:
  ToS + fragility risk to validate early.
- **Protocol** — adopt ACP directly vs a minimal pinned JSON-RPC.
