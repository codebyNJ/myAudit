# Claude Code Agent Generation Engine — Design

**Status:** Draft for review (revises the Pi-agent design — Pi removed)
**Date:** 2026-09-03
**Supersedes:** the prompt-per-node path (stripped `claude -p` codegen + `realSpecs`) **and** the abandoned Pi-harness approach.

**Goal:** Stop paying an LLM to regenerate a template we already own. Generate apps by (1) scaffolding the battle-tested template deterministically at **$0**, then (2) driving **Claude Code itself** — in full agent mode — only for the genuinely per-project deltas, sandboxed by Claude Code's own tool-permission system, reading the template's own `CLAUDE.md`, verified by a real build/boot.

**Architecture (one screen):** Go orchestrator (unchanged: stateful graph + Postgres queue) → deterministic scaffold step (copy template + `init-project.js`) → per-feature **Claude Code agent** (`claude -p` in agent mode) runs in a container rooted at the scaffolded repo, edits files with its native Read/Write/Edit/Bash tools under an `--allowedTools`/`--disallowedTools` policy, auto-loads the template's `CLAUDE.md` as guardrails → orchestrator runs a real build/boot verification → captures `git diff` as the node's output.

**Tech stack:** Go (orchestrator, unchanged), **Claude Code** (`claude` CLI, already installed + authenticated on the user's subscription), Docker (isolation), `ankor-fullstack-template` (Fastify + Mongoose + React). No Pi, no separate harness login, no third-party model credits.

---

## Global Constraints

- **The template is ground truth.** Files it already contains are copied, never generated. The agent may *edit* template files, but the base app is produced by `scripts/init-project.js`, not by a model.
- **Guardrails come from the template.** Claude Code auto-loads the scaffolded repo's `CLAUDE.md` (the template ships one: strict config, workspace-scoped queries, authz order, "copy `Item` to add a feature"). We do **not** re-author guardrails in Go, and we scope settings so the developer's *global* CLAUDE.md/settings don't leak in.
- **Every agent call is bounded.** No uncapped loop: per-node turn/time cap → human checkpoint. Haiku default; escalate model per node when needed.
- **No mocked model in the generation path.** Validation uses **real `claude` calls** against the real template, hard-capped and aborted the moment success is observable. Fakes are allowed only for pure-logic units (graph, queue, review, dispatch) that don't exercise the agent.
- **Sandboxed tools.** The agent runs inside a container with a `--allowedTools`/`--disallowedTools` policy (scoped Bash). Nothing it runs touches the host; egress is limited to the model endpoint.

---

## 1. Why (the problem this fixes)

`ankor-fullstack-template` is a **complete, running, production-grade app** (123 files: JWT + rotating refresh cookie auth, workspaces, multi-tenancy, RBAC authz stack, membership cache, Google auth, React+RTK frontend). The old path paid the model to regenerate weaker copies of `membershipCache.js`, `user.model.js`, `auth.controller.js` — files that already exist verbatim. It also ships:
- **A zero-token scaffolder** — `node scripts/init-project.js "Name" slug` renames the identity, seeds `backend/.env`, resets git. One command → the whole base app. No LLM. (Proven at CHECKPOINT 1: a scaffolded app boots `/health` at **$0**.)
- **Its own agent guardrails** — `CLAUDE.md` / `AGENTS.md`, which Claude Code reads natively.

So: do the deterministic 80% with the script (free); reserve the model for the per-project 20% — the specific domain modules beyond the generic `Item`.

## 2. Why Claude Code (and why not Pi)

The decision is a **single tool-calling agent — reuse the one we already run.** Claude Code is the reuse, and it beats Pi on every axis that blocked us:

- **Already authenticated on the user's Claude subscription.** This directly removes the Phase-0 blocker that killed Pi: Pi needed its own key/login, and the available OpenRouter balance couldn't afford a capable model. Claude Code has been running all session at ~$0 marginal (subscription-metered).
- **Native tool-permission system = the sandbox allow/deny we wanted, with no custom extension.** `--allowedTools "Edit Write Read Bash(npm:*) Bash(node:*) Bash(git:*) Glob Grep"`, `--disallowedTools`, `permissions.deny` in settings, `--permission-mode`. (Pi shipped no sandbox — we'd have had to build a TypeScript `bash-guard` extension. Deleted from scope.)
- **Auto-loads `CLAUDE.md`** from the scaffolded repo → template guardrails load themselves.
- **`--add-dir` + cwd** scope it to the workspace; `--dangerously-skip-permissions` inside the container is the hardened headless pattern.
- **Full agent mode** (real Bash/Edit/Write/Read) — the agent edits files itself; we read the `git diff`. This is the *opposite* of the earlier stripped "return a JSON file-contract" use of `claude -p`.
- We already parse Claude Code's `--output-format json` envelope (`internal/claude/claude.go`): `result`, `is_error`, `total_cost_usd`, `usage`. Reuse that.

## 3. Target architecture

### 3.1 Components & data flow

```
POST /api/runs (project + chosen domain resources)
        │
        ▼
Go orchestrator ── stateful graph + Postgres queue  (UNCHANGED)
        │
        ├─ node: scaffold        → DETERMINISTIC. copy template + init-project.js. 0 tokens.
        │
        ├─ node: config          → DETERMINISTIC (env flags: MEMBERSHIP_CACHE, module include/exclude).
        │
        ├─ node: feature <X>      → CLAUDE CODE AGENT. `claude -p` in agent mode, cwd = scaffolded repo,
        │   (one per domain         guardrails = template CLAUDE.md, allow/deny tool policy, in container.
        │    resource)              task = "add feature module <X> following the Item pattern; fields: …".
        │        │                  The agent edits files itself.
        │        ├─ verify: orchestrator runs a real build/boot in the sandbox. green → capture git diff.
        │        │          red   → bounded repair turn(s) → still red → checkpoint.
        │        ▼
        └─ node: finalize        → DETERMINISTIC (Dockerfile already in template).

Change capture: after each node, `git diff` in the sandbox IS the node output —
recorded to Postgres, shown in the UI; Accept/Reject = keep/revert the diff.
```

### 3.2 The agent's job is narrow

Because the base app is free, the agent only does what the template calls "adding a feature": copy the `Item` module end to end and adapt it (model → route/controller/validations → register in `models/index.js` + `routes/v1/index.js` + `deleteWorkspace` cascade → frontend endpoints/servicesApi/view/route/nav). Mostly mechanical (rename `Item`→`Project`); tokens go only to the novel fields/validations/logic. The template's `CLAUDE.md` names `Item` as the canonical example, so the agent has an in-repo reference.

### 3.3 Scaffolding is deterministic (done)

The `scaffold` node does not invoke the model. `internal/sandbox` copies the template and runs `init-project.js`. **Already built and green (Phase 1); CHECKPOINT 1 proved a scaffolded app boots `/health` at $0.**

### 3.4 Change capture (git diff, not a file contract)

The agent writes files itself, so "what changed" comes from the repo: after a node, `git diff` (vs the last node's commit) is the recorded output. Accept/Reject reuses the existing review store: reject = revert that node's diff. `git` baseline + `Diff`/`Commit` already exist in `internal/sandbox`.

### 3.5 Verification & bounded repair

- Verification runs a **real build/boot** in the sandbox (0 model tokens): `npm ci` + build/lint + **smoke-boot `/health`** (the template ships no test harness by design). Project tests run when present.
- On failure: feed the failing output back to the same `claude` session (`--resume`/`--continue`) for a capped number of repair turns (default 2). Still failing → raise a human checkpoint. Only multi-turn path; bounded.

### 3.6 Models & cost

`--model claude-haiku-…` default (cheap loop), escalate to Sonnet for nodes flagged complex. Subscription-metered (~$0 marginal). The old OpenRouter fallback becomes optional/emergency only, since subscription auth is reliable. Cost/usage logged from the `claude` JSON envelope (`total_cost_usd`, `usage`) via slog → Dozzle.

## 4. Sandbox & allow/deny (native — no extension)

Two layers:

1. **Container isolation (blast radius):** each run gets a container/volume holding only the scaffolded repo; no host mounts beyond the repo; egress limited to the model endpoint. Inside the container the headless pattern is `--dangerously-skip-permissions` (the container is the real boundary) **or** a curated allowlist (below) for defense in depth.
2. **Tool policy (intent):** Claude Code's own flags —
   - **Allow (illustrative):** `Edit Write Read Glob Grep`, `Bash(npm:*) Bash(npx:*) Bash(node:*) Bash(git:*) Bash(mkdir:*) Bash(cp:*) Bash(mv:*)`.
   - **Deny (illustrative):** `Bash(rm:*) Bash(sudo:*) Bash(curl:*) Bash(wget:*)`, `WebFetch`, `WebSearch`, and anything outside the workspace (`--add-dir` scopes writes).
   - Denials surface as tool-permission errors the agent adapts to; log them (Dozzle + DB).

Exact lists live in config so a deployment can tighten them.

## 5. What changes in the codebase

**Kept (done / good):**
- `internal/sandbox` — Scaffold/Run/Diff/Commit. **Done, green.** Harness-agnostic; unaffected by the Pi→Claude Code switch.
- Go orchestrator: graph, Postgres queue, events/slog, migrations, checkpoints.
- `POST /api/runs/{id}/review` Accept/Reject + review store (reject = revert node diff).
- React UI, board/kanban/activity, cost logging.
- `internal/claude` envelope parsing (`total_cost_usd`, `usage`) — reused by the new runner.

**New:**
- `internal/agent` — a Claude Code runner: builds the `claude -p` agent-mode invocation (allow/deny, `--add-dir`, `--output-format json/stream-json`, `--model`, cwd = workspace, `--session-id`/`--resume` for repair), runs it against a `sandbox.Workspace`, parses the envelope → `Result{OK, Summary, CostUSD, Tokens, Err}`.
- Verification hook: real build/boot in the sandbox.
- `Dockerfile.sandbox` — container with `claude` + node + git for the isolated run.

**Removed / replaced:**
- `internal/piagent` (the Pi runner just prototyped) — **deleted**. Pi is gone entirely.
- `internal/api/runloop.go` — `realSpecs` / `realGuardrails` **deleted** (guardrails = template CLAUDE.md).
- The stripped `execRunner` file-contract path in `internal/claude` — replaced by `internal/agent` (envelope parsing kept).

**Companion (product):** the config wizard's tenancy/cache/module toggles are now free template features; the generative input becomes **`resources: [{name, fields:[{name,type}]}]`** on `POST /api/runs`, driving the feature nodes.

## 6. Cost model

| Work | Old | New |
|---|---|---|
| Scaffold + base app | regenerated (worse), ~$0.36–0.43 × ~7 | **$0** (script) |
| Each custom feature | n/a | bounded agent run, Haiku, subscription-metered |
| Verification | simulated | $0 (real subprocess) |
| Repair | token-wasteful retry | capped turns → human |

$0 for everything the template covers; model spend only on per-project features, bounded per node, on the subscription.

## 7. Testing / validation

Real `claude` calls in the generation path — literally "our Claude Code" — **cut the moment success is observable**:
- **Unit (fakes allowed):** graph, queue, review-revert, dispatch logic, allow/deny arg building, sandbox lifecycle. No model.
- **Integration (real `claude`, bounded):** scaffold the real template; run one feature node (add `Project`); assert the files exist + register + the app builds/boots; **abort on green**; hard timeout + turn cap.
- **Allow/deny proof:** a task tempting a denied command (e.g. network fetch) shows it blocked + logged.
- Platform Go suite stays green (`make test`).

## 8. Risks & open questions

1. **Headless permission mode** — curated `--allowedTools` + container, vs `--dangerously-skip-permissions` in the container. *Recommendation:* curated allowlist first (gives the allow/deny the user asked for); bypass only if the allowlist proves too fiddly, since the container already bounds blast radius.
2. **Global vs project settings leakage** — ensure `--setting-sources`/cwd load the *template's* CLAUDE.md but not the developer's global one (which would pollute context/cost).
3. **Per-project verification command** — minimum gate = `npm ci` + build + smoke-boot `/health`; add project tests when present.
4. **Repair loop mechanics** — `--resume`/`--continue` a session vs a fresh call with the error appended. Pin during writing-plans.
5. **Frontend deltas** — one node does backend + frontend per resource (template's "copy Item end to end").
6. **Container has `claude` + auth** — the run container needs the `claude` binary and the subscription credentials mounted/forwarded. Packaging detail for writing-plans.

## 9. Out of scope

- Pi, multi-agent/Buzz coordination, rebuilding a Goose loop.
- The detailed config-wizard redesign (only the `resources` API contract is in scope).
- Orchestrator/queue/DB internals.

---

## Next step

On approval, revise the implementation plan to the Claude Code runner (delete the Pi phases; keep the done sandbox phase), then continue executing from Phase 2.
