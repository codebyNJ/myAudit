# Claude Code Agent Generation Engine — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Steps use checkbox (`- [ ]`). **After every CHECKPOINT, stop and verify the stated expected output before continuing** (check every 2–3 tasks).

**Goal:** Deterministic template scaffold ($0) + Claude Code (agent mode) for per-project feature deltas, sandboxed by Claude Code's own tool permissions, verified by real build/boot. (Revises the Pi plan — Pi removed.)

**Architecture:** Go orchestrator → `internal/sandbox` scaffolds (done) → `internal/agent` drives `claude -p` in agent mode in the scaffolded repo (guardrails = template `CLAUDE.md`, allow/deny via `--allowedTools`/`--disallowedTools`, in a container) → orchestrator runs real build/boot → captures `git diff`.

**Tech Stack:** Go, **Claude Code** (`claude` CLI, already installed + subscription-authed), Docker, `ankor-fullstack-template`.

**Spec:** `docs/superpowers/specs/2026-09-03-claude-code-agent-generation-engine-design.md`

## Global Constraints

- Template is ground truth; base app from `scripts/init-project.js`, never a model.
- Guardrails = template `CLAUDE.md` (Claude Code auto-loads it). Use `--setting-sources project` so the developer's *global* CLAUDE.md/settings don't leak in.
- No mocked model in the generation path. Real `claude` calls in integration tests, hard-capped, aborted on green.
- Every agent call bounded: turn/time cap → human checkpoint. Haiku default.
- Agent tools under an allow/deny policy inside a container.
- Template path (dev): `/Users/nijeesh/codes/ankor-fullstack-template`; configurable via `TEMPLATE_PATH`.

## Decisions (defaulted — veto to change)

1. **Verify gate** = `npm ci` + build + **smoke-boot `/health`** (real; template ships no test harness), plus project tests if present.
2. **Domain input** = `resources:[{name,fields:[{name,type}]}]` on `POST /api/runs`.
3. **Feature node** = backend + frontend for one resource (template's "copy Item end to end").
4. **Permission model** = curated `--allowedTools`/`--disallowedTools` + `--permission-mode acceptEdits`, inside a container (bypass only if the allowlist proves too fiddly).

## Pinned Claude Code facts (verified this session, v2.1.204)

- Headless agent: `claude -p "<task>" --output-format json` runs full agent mode (native Read/Write/Edit/Bash/Glob/Grep). Envelope: `{result, is_error, subtype, total_cost_usd, usage:{...}}` — already parsed in `internal/claude/claude.go`.
- **stdin must be closed** (`cmd.Stdin = nil`) — same hang class as before.
- Scope: cwd = workspace **and** `--add-dir <ws>`.
- Guardrails: cwd's `CLAUDE.md` auto-loads; `--setting-sources project` loads project (template) settings, not the developer's global ones.
- Permissions/sandbox (the allow/deny CLI): `--permission-mode <acceptEdits|bypassPermissions|…>`, `--allowedTools "Edit" "Write" "Read" "Glob" "Grep" "Bash(npm:*)" "Bash(node:*)" "Bash(git:*)" …`, `--disallowedTools "Bash(rm:*)" "Bash(sudo:*)" "Bash(curl:*)" "WebFetch" "WebSearch"`. `--dangerously-skip-permissions` bypasses all (container only).
- Model: `--model claude-haiku-4-5-20251001` (or alias); subscription-authed (no key needed).
- Repair: `--session-id <uuid>` on first call, `--resume <uuid>` to continue with the failing output.
- Cost/usage: from the JSON envelope (`total_cost_usd`, `usage`).

### Phase 0 results (verified live)

- ✅ Agent mode edits files headlessly: `claude -p … --permission-mode acceptEdits --allowedTools "Read" "Write" "Bash(cat:*)" --setting-sources project` created `b.txt="HELLO WORLD"`, `is_error:false`, cost ~$0.074 (agent-mode overhead is higher than stripped codegen, but scaffolding is $0 and only feature deltas pay it).
- ✅ Denied command refused: with `--disallowedTools "Bash(curl:*)" …`, the agent did **not** run curl (no file written) and returned `is_error:false` with a result explaining *"Your current permission settings don't allow `curl` commands."*
- ⚠️ **Denial signature:** a blocked tool does **not** set `is_error:true` — the run still "succeeds" with an explanatory `result`. So the runner detects denials by the tool not having executed (git diff empty / expected file absent), not by `is_error`. Curate the **allow** list to include everything features need (npm/node/git/mkdir/cp/mv) so the agent never stalls on a needed command.

## File Structure

- `internal/sandbox/*` — scaffold/run/diff/commit. **DONE (Phase 1).**
- `internal/agent/agent.go` — Claude Code runner (build invocation, run, parse envelope → Result). NEW.
- `internal/agent/agent_test.go` — unit (Args builder + envelope parse) + bounded real-`claude` integration. NEW.
- `internal/api/runloop.go` — delete `realSpecs`/`realGuardrails`; `NewRealDeps` builds sandbox+agent deps.
- `internal/api/api.go` — `POST /api/runs` accepts `resources`; graph reshaped.
- `internal/worker/worker.go` — dispatch (deterministic vs agent), git-diff output, verify gate, bounded repair.
- `internal/store/graph.go` — feature node carries its resource spec.
- `cmd/serve/main.go` — wire `TEMPLATE_PATH`, model, container flag.
- `Dockerfile.sandbox` — container with `claude` + node + git. NEW.
- **Remove:** `internal/piagent/*` (Pi prototype), the stripped `execRunner` contract path.

---

## Phase 0 — Prove Claude Code edits a file headlessly under a policy (spike)

### Task 0.1: One real agent-mode run with allow/deny

- [ ] **Step 1:** Throwaway git dir with `a.txt`. Run:
```bash
claude -p "Create b.txt whose contents are the uppercase of a.txt. Use your tools." \
  --output-format json --add-dir . --permission-mode acceptEdits \
  --allowedTools "Read" "Write" "Bash(cat:*)" --setting-sources project < /dev/null
```
- [ ] **Step 2:** Assert `b.txt` == `HELLO WORLD`; capture the envelope's `total_cost_usd` + `usage`. **Cut once b.txt is correct.**
- [ ] **Step 3:** Re-run a variant asking it to `curl example.com` with `--disallowedTools "Bash(curl:*)" "WebFetch"`; confirm it's refused. Record how a denial appears in the envelope.
- [ ] **Step 4:** Note findings (exact working flag set, denial signature) in this file.

**⛔ CHECKPOINT 0 — expected:** `claude` headless agent creates the file (real, subscription cost logged) and a denied command is refused. If agent mode won't run non-interactively under the allowlist, STOP and adjust the permission approach.

---

## Phase 1 — Deterministic scaffold + sandbox ($0) — ✅ DONE

`internal/sandbox` (Scaffold/Run/Diff/Commit) built and green; **CHECKPOINT 1 passed** — a scaffolded app booted `GET /api/health → 200` at $0. No further work.

---

## Phase 2 — Claude Code runner + one real feature

### Task 2.1: `agent.Run` — build invocation + parse envelope

**Files:** Create `internal/agent/agent.go`, `internal/agent/agent_test.go`.

**Interfaces — Consumes:** `sandbox.Workspace`. **Produces:**
- `type Result struct { OK bool; Summary string; CostUSD float64; Err string }`
- `type Options struct { Model, SessionID string; Allow, Deny []string; PermissionMode string }`
- `func (o Options) Args(task, wsDir string) []string`
- `func Run(ctx, ws sandbox.Workspace, task string, opt Options) (Result, error)`

- [ ] **Step 1: Failing unit tests:** (a) `Args` includes `-p`, `--output-format json`, `--add-dir <ws>`, `--permission-mode`, each `--allowedTools`/`--disallowedTools` entry, `--setting-sources project`; (b) `parseEnvelope` maps a fixture `{"result":"done","is_error":false,"total_cost_usd":0.004,"usage":{...}}` → `Result{OK:true,CostUSD:0.004}` and an `is_error:true` fixture → `OK:false` with `Err`.
- [ ] **Step 2: Run — expect FAIL.**
- [ ] **Step 3: Implement:** `exec.CommandContext(ctx, "claude", opt.Args(task, ws.Dir)...)`, `Dir=ws.Dir`, `Stdin=nil`, `Env=os.Environ()`. Parse the JSON envelope (reuse the shape from `internal/claude/claude.go`). `OK = !is_error && total_cost_usd/usage present`.
- [ ] **Step 4: Run — expect PASS.** Commit `feat(agent): claude code runner + envelope parse`.

### Task 2.2: One real feature node end-to-end (real `claude`, cut on green)

**Files:** `internal/agent/agent_integration_test.go` (`//go:build integration`).

- [ ] **Step 1:** Scaffold the real template; `agent.Run(ws, "Add a workspace-scoped feature module 'Project' with fields name:string, status:string, following the Item module pattern end to end (backend model/route/controller/validations + register in models/index.js, routes/v1/index.js, deleteWorkspace cascade; frontend endpoints/servicesApi/view/route/nav).", {Model: haiku, Allow: …, Deny: …})`. Assert `backend/src/models/project.model.js` exists and `routes/v1/index.js` references it, then `npm ci && npm run build` (or tsc) exit 0. **Abort on green.** Hard `context.WithTimeout` + turn cap.
- [ ] **Step 2: Run** `go test -tags integration ./internal/agent/ -run RealFeature -v`.
- [ ] **Step 3:** Record `Result.CostUSD`. Commit `test(agent): real feature-delta integration (bounded)`.

**⛔ CHECKPOINT 2 — expected:** one real `Project` feature lands on a real scaffold and the app still builds; cost is subscription-metered (cents). If the agent can't follow the Item pattern from `CLAUDE.md`, STOP and adjust task text / `--append-system-prompt`.

---

## Phase 3 — Allow/deny hardening + container

### Task 3.1: Policy config + denial proof

**Files:** `internal/agent/agent.go` (policy defaults), `internal/agent/agent_integration_test.go`.

- [ ] **Step 1:** Default Allow/Deny lists (per spec §4). Unit-test they render into the args.
- [ ] **Step 2:** Real bounded test: a task that tempts a denied command (network fetch) → assert refused + logged, and the fetch did not happen. Cut on observing the refusal.
- [ ] **Step 3: Commit** `feat(agent): default allow/deny tool policy + denial proof`.

### Task 3.2: Run the agent inside a container

**Files:** `Dockerfile.sandbox`; `agent.go` (`AGENT_ISOLATE=1` → `docker run`).

- [ ] **Step 1:** `Dockerfile.sandbox`: node + git + `claude`; workdir `/work`.
- [ ] **Step 2:** When `AGENT_ISOLATE=1`, run `claude` via `docker run --rm -v <ws>:/work -w /work <img>` with subscription creds forwarded and egress limited to the model endpoint.
- [ ] **Step 3:** Integration test: same feature works in the container.
- [ ] **Step 4: Commit** `feat(agent): optional docker isolation`.

**⛔ CHECKPOINT 3 — expected:** denied command provably blocked; a feature builds while `claude` runs in the container. If creds/egress break inside the container, STOP and fix before wiring the orchestrator.

---

## Phase 4 — Orchestrator integration

### Task 4.1: Reshape graph + accept resources

**Files:** `internal/api/api.go`, `internal/store/graph.go`, `internal/api/runloop.go` (delete `realSpecs`/`realGuardrails`).

- [ ] **Step 1: Failing test:** `POST /api/runs` with `{project, resources:[{name:"Project",fields:[…]}]}` creates `scaffold → config → feature:Project (carrying fields) → finalize`.
- [ ] **Step 2: Run — FAIL.**
- [ ] **Step 3: Implement:** add `Resources` to the request; build specs; store the resource spec on the feature node; delete `realSpecs`/`realGuardrails`.
- [ ] **Step 4: Run — PASS.** Commit `feat(api): resource-driven graph; drop prompt specs`.

### Task 4.2: Worker dispatch + verify + bounded repair

**Files:** `internal/worker/worker.go`.

- [ ] **Step 1: Failing test** (fake agent for dispatch logic — not the generation path): `scaffold` node → `sandbox.Scaffold`; `feature` node → agent then verify hook; verify-fail within cap re-invokes (`--resume`), past cap raises a checkpoint.
- [ ] **Step 2: Run — FAIL.**
- [ ] **Step 3: Implement:** dispatch by node type; deterministic nodes run sandbox ops; feature nodes run `agent.Run` then `verify(ws)` (npm ci + build + smoke `/health`); capture `ws.Diff()`; `ws.Commit()` on success; bounded repair → `RaiseCheckpoint`.
- [ ] **Step 4: Run — PASS.** Commit `feat(worker): agent/deterministic dispatch + verify + repair`.

### Task 4.3: Wire serve + cost logging

**Files:** `cmd/serve/main.go`, `internal/api/runloop.go`.

- [ ] **Step 1:** `NewRealDeps` builds sandbox+agent deps (`TEMPLATE_PATH`, model, `AGENT_ISOLATE`, allow/deny). Log `agent.cost` from `Result.CostUSD` via slog → Dozzle.
- [ ] **Step 2:** `go build ./... && go vet ./...`. Commit `feat(serve): wire claude-code agent deps + cost log`.

**⛔ CHECKPOINT 4 — expected:** `POST /api/runs` with one resource drives scaffold($0) → feature(agent) → verify, visible in the UI; node diff shows in the Dev tab; Accept/Reject reverts; Dozzle shows `agent.cost`. Run once for real. If the UI regresses, STOP.

---

## Phase 5 — Validation + cleanup

### Task 5.1: Bounded real-call e2e + guard proof

- [ ] **Step 1:** One real e2e: 2-resource app; scaffold boots + each feature builds; **abort on green**; hard timeout + turn cap.
- [ ] **Step 2:** Keep the allow/deny proof. `make test` (unit) green and model-free.
- [ ] **Step 3: Commit** `test: bounded e2e + guard proofs`.

### Task 5.2: Delete dead code + docs

- [ ] **Step 1:** Remove `internal/piagent`, the stripped `execRunner` contract path, and any `NewFallback`/OpenRouter wiring no longer used. `go build ./... && go vet ./... && make test`.
- [ ] **Step 2:** Update `docs/architecture.md` to the Claude Code agent engine. Commit `chore: remove pi + prompt-per-node generator; docs`.

**⛔ CHECKPOINT 5 — expected:** green unit suite (model-free), one bounded real e2e passing, guard proven, docs updated → superpowers:finishing-a-development-branch.

---

## Self-Review

- **Coverage:** scaffold-as-tool ✓ (P1 done), agent for deltas ✓ (P2), template CLAUDE.md guardrails ✓ (P0/P2), native allow/deny sandbox ✓ (P3), git-diff capture + Accept/Reject ✓ (P1/4.2), verify+repair ✓ (4.2), resources API ✓ (4.1), cost logging ✓ (4.3), real bounded tests ✓ (2.2/5.1). Pi removed ✓ (5.2).
- **Placeholder scan:** model id / denial signature pinned in Task 0.1 before use. Allow/deny lists concrete. No TBDs.
- **Type consistency:** `sandbox.Workspace`, `agent.Run`/`Result`/`Options` used consistently P2–P4.
- **Ordering:** CP0 proves agent+policy before building the runner; CP1 ($0 scaffold) already banked; guard (P3) precedes orchestrator wiring (P4).
