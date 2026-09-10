# myAudit — Agent Behavior Audit Log

Sequential audit per item. No fixes applied — this is findings + proposals only, for review before any code change.

---

## 1. Process-launch failure — "The system cannot find the file specified"

**Files read:** `internal/agent/agent.go`, `internal/config/config.go`, `internal/sandbox/sandbox.go`.

### What the code actually does today

`agentEnv()` (`agent.go:185-197`) builds the child process environment as a filtered copy of `os.Environ()`, dropping only two keys — `PORT` and `MYAUDIT_DB`:

```go
func agentEnv() []string {
	drop := map[string]bool{"PORT": true, "MYAUDIT_DB": true}
	src := os.Environ()
	out := make([]string, 0, len(src))
	for _, kv := range src {
		k, _, _ := strings.Cut(kv, "=")
		if drop[k] {
			continue
		}
		out = append(out, kv)
	}
	return out
}
```

`command()` (`agent.go:99-135`) always invokes the bare name `"claude"` (no absolute-path override exists anywhere in config or `Options`):

```go
c = exec.CommandContext(ctx, "claude", o.Args(task, dir)...)
c.Dir = dir
proc.SetGroup(c)
c.Cancel = func() error { return proc.KillTree(c) }
```

`internal/config/config.go` only exposes `MYAUDIT_DB` — there is no config knob for a custom `claude` binary path.

### Root-cause investigation (live-tested this session, not just read)

I tested all four hypotheses from the brief directly, plus one the brief didn't list:

| Hypothesis | Verdict | Evidence |
|---|---|---|
| `agentEnv()` strips `PATH`/`PATHEXT` | **Ruled out** | Code only drops `PORT`/`MYAUDIT_DB`, confirmed by reading the source above. |
| Windows-specific | **Confirmed, but not itself the cause** | `"The system cannot find the file specified"` is the literal Win32 `ERROR_FILE_NOT_FOUND` string; this exact error class doesn't occur on POSIX. Root cause is Windows-specific, but *why* required further digging. |
| `claude` at a user-configured absolute path, not on `PATH` | **Ruled out for this case** | No such config exists, so this can't be *why* it worked sometimes — but it's a related real gap (see Fix, part B). `claude` resolves fine via `PATH` in isolation (`claude --version` and simple `-p` calls both succeed). |
| Race with `sandbox.Import()` — workspace dir doesn't exist yet | **Ruled out** | The graph enforces `import → map → qa` as a hard dependency chain (`PromoteReady` in `internal/queue/queue.go` only promotes a node once its deps are `done`), and `scanModules()` (called synchronously inside `doMap`, after import has completed) discovers module subdirectories via `os.ReadDir` on paths that must already exist for it to find them. By the time a `qa` node's `--add-dir <module-subdir>` runs, that directory has existed since `doMap` ran. |
| **(not in the brief) argument content/length** | **Confirmed as the actual root cause** | See below. |

I isolated this empirically with a minimal Go reproduction, built up in stages, run directly against the real Windows machine:

1. `exec.Command("claude", "--version")` → succeeds.
2. Full real invocation (`CommandContext`, `agentEnv()`-filtered env, `--add-dir`, `--permission-mode acceptEdits`, `--setting-sources project`, `--model claude-opus-4-8`, full `--allowedTools`/`--disallowedTools`, streaming `--output-format stream-json --verbose`, `WaitDelay`, `Cancel`) with a **trivial prompt** (`"say hi"`) → succeeds, every time, in the real workspace directory that had just failed for real.
3. Same exact invocation, but with the **real `qaTask(module, path)` prompt** (1520 characters, containing embedded double quotes, backticks, and JSON braces — see `internal/worker/qa.go:466-486`) → **reproduces the exact failure**:
   ```
   Wait err: exit status 1
   Stderr: The system cannot find the file specified.
   ```

This pinpoints the cause precisely: on Windows, `claude` resolves via `PATH`/`PATHEXT` to `claude.cmd` (a batch shim in the npm global install dir — confirmed via `Get-Command claude` and directory listing; there's no native `claude.exe`). Go's `os/exec` package has special handling for launching `.bat`/`.cmd` files on Windows (added as part of the CVE-2024-24576 command-injection fix), which re-quotes/escapes arguments before handing them to `cmd.exe /c`. The real QA/flows/fix prompts are long strings dense with `"`, `` ` ``, `{`, `}` and newlines — exactly the content most likely to break that escaping layer. When it breaks, `cmd.exe` (or something in the nested `claude.cmd` → `node` invocation chain) misinterprets part of the mangled command line as a filename, and Windows reports "file not found" — even though `claude.cmd` itself, `node`, and everything else genuinely exist and are on `PATH`.

This also explains why the `flows` node fails differently (`"flows parse: unexpected end of JSON input"` rather than the exec error): `doFlows` calls `d.Agent.Run(flowCtx, ws, flowsTask, agent.ReadOnly, nil)` with `onStep = nil`, which routes through the **non-streaming** path in `Run()` (`cmd.Output()`), not `runStreaming()`. The non-streaming path's error handling (`agent.go:244-254`) checks `len(out) > 0` before checking `err`, so if the mangled command line still causes *something* to be written to stdout before the process exits non-zero, that gets parsed as if it were a genuine (empty) envelope instead of surfacing the launch problem — a second, related bug (tracked here for completeness, primary fix is item 2's territory).

### Edge cases

- **Any prompt long/quote-dense enough to trip the escaping** — this isn't a one-off; every real `qa`/`bug`/`flows`/chat call uses a prompt of this shape, so on Windows this is not intermittent, it is the default failure mode. Confirmed: 5 separate real runs on this machine (`Bartic`, `parallax-cloud` ×2, `demo`, `zeroprep-hyderabad`) all failed identically.
- **macOS/Linux are almost certainly unaffected** — POSIX `exec` doesn't route through a shell-quoting layer the way Windows `.bat`/`.cmd` launch does; `claude` on those platforms is typically a real script with a shebang or a native binary invoked directly.
- **Shorter, simpler prompts might work by luck** — a short chat message, or a `map`/`flows` overview prompt without embedded JSON-shaped content, may not trip the escaping bug, which is consistent with the manual `map` overview prompt (`mapTask`, plain prose, no embedded quotes/braces) succeeding in earlier ad hoc testing this session while `qaTask`/`flowsTask` consistently failed.
- **This is a Go standard-library / Windows interaction, not a myAudit logic bug per se** — the fix has to work around it rather than "correct" a mistake in `agent.go`'s own logic, which is otherwise doing something reasonable (pass the task as `-p <arg>`).

### Proposed fix (not applied)

**Primary fix:** stop passing the task as a CLI argument on the code path that currently does `[]string{"-p", task}`. `claude --help` confirms `-p`/`--print` is a mode flag and `prompt` is a separate, optional positional argument — and myAudit's own code already has a comment (`agent.go:237`, `"CRITICAL: -p mode blocks forever waiting on stdin EOF"`) confirming that Claude Code CLI supports reading the prompt from stdin when no positional prompt is given. Switching to stdin-delivery for the task text sidesteps the Windows command-line construction entirely — stdin content is never subject to `cmd.exe` argument quoting/escaping, so this closes the bug regardless of prompt content, length, or platform. Concretely: pass `-p` with no value, set `cmd.Stdin` to a reader over the task text instead of `nil`, and drop `task` from the positional args in `Args()`. This is a small, scoped change confined to `agent.go`'s `command()`/`Args()`/`Run()`, with no change needed anywhere else (worker, store, prompts, UI) since the task content itself doesn't change.

**Secondary, defense-in-depth fix (part B):** add an optional `CLAUDE_BIN` (or similar) env var read in a new `internal/config` field, defaulting to `"claude"`, so a user with `claude` installed at a non-`PATH` location has a way to point myAudit at it. Not the cause of the observed bug, but a real gap surfaced during this investigation and cheap to close alongside item 1.

**Tertiary note:** the non-streaming path's `len(out) > 0` before `err` check (`agent.go:244-246`) should also be reordered or made more careful — while not the primary fix, it's what let this specific failure surface as a confusing "empty JSON" error for `flows` instead of the same clear exec error `qa`/`bug` get. Covered in item 2.

---

## 2. Truncated/invalid JSON from agent output — "unexpected end of JSON input"

**Files read:** `internal/worker/qa.go` (`parseFindings` — actually lives in `internal/worker/worker.go:219-235`, shared by `qa.go`), `internal/worker/flows.go` (`doFlows`, `parseFlows`), `internal/agent/agent.go` (`Run`, `runStreaming`, `parseEnvelope`).

### What the code actually does today

**Output capture is two different mechanisms depending on the node type:**

- `qa`/`bug` (via `worker.runAgent`, which always sets an `onStep` callback — `worker.go:139-146`) go through `runStreaming()` (`agent.go:261-305`): stdout is read **line-by-line** with a `bufio.Scanner`, each line parsed as its own small JSON object (`{"type":"assistant",...}`, `{"type":"result",...}`, etc.), and only the line where `type == "result"` is parsed into the final `Result` via `parseEnvelope`. If no such line is ever seen (`!haveFinal`), it returns an error built from stderr or the wait error — this is the `"claude stream: %s"` message from item 1.
- `map`'s best-effort overview and `flows` (both call `Agent.Run` with `onStep = nil`) go through the **buffered** path in `Run()` (`agent.go:243-255`): the entire stdout is read via `cmd.Output()` after the process exits, and **the whole thing** is handed to `parseEnvelope` as one blob — there is no line-splitting here at all, `--output-format json` is expected to produce exactly one JSON object for the whole invocation.

**`doFlows`'s handling of the result is the key gap** (`flows.go:51-69`):

```go
r, err := d.Agent.Run(flowCtx, ws, flowsTask, agent.ReadOnly, nil)
if err != nil {
	d.fail(ctx, c, "flows agent: "+err.Error())
	return true, nil
}

doc, perr := parseFlows(r.Summary)
if perr != nil {
	...
	d.fail(ctx, c, "flows parse: "+perr.Error()+" | agent said: "+snippet)
	return true, nil
}
```

It checks the **launch error** (`err`) but never checks **`r.OK`/`r.Err`** — the fields `parseEnvelope` sets when the envelope itself says `is_error: true`, or (critically) when `json.Unmarshal` of the raw bytes into the envelope struct fails, in which case `parseEnvelope` returns `Result{Err: "parse envelope: ..."}` with a **zero-value empty `Summary`** and `OK: false` (`agent.go:211-215`). `doFlows` doesn't look at `r.Err` at all — it just feeds the empty `r.Summary` straight into `parseFlows`, which then fails on `json.Unmarshal([]byte(""), &doc)` with exactly `"unexpected end of JSON input"`. This is a **generic downstream symptom hiding a more specific upstream cause** — the real problem (`r.Err`, e.g. `"parse envelope: ..."` or an `is_error` subtype) is silently dropped.

I confirmed this is exactly what happened in the observed failures: the actual root cause was item 1's Windows `.cmd`-argument-escaping bug, which — on the *buffered* path `flows` uses — produced *some* garbage stdout bytes (enough to pass `len(out) > 0` in `Run()`, so `err == nil` was returned) but not a valid envelope, so `parseEnvelope` silently failed inside `Run()` and `doFlows` never surfaced why.

**Separately, `qa`/`bug`'s `runAgent` wrapper (`worker.go:135-168`) *does* check `r.OK`** and retries up to `MaxRepairs` times, re-prompting with the failure appended, before raising a checkpoint — but `doFlows` bypasses `runAgent` entirely and calls `d.Agent.Run` directly, so `flows` nodes get **no retry, no repair-prompt, and no checkpoint** on an agent-level failure — one bad response and the node is just `failed`, permanently, with only a manual "Re-identify flows" click as recourse.

### Edge cases (evaluated against the actual code, not hypothetically)

- **Process killed by timeout mid-write** — `flowsTimeout` is 12 minutes, `liveTimeout` (`qa`/`bug`) is 20 minutes. On the streaming path, a mid-write kill would show as `!haveFinal` in `runStreaming` (item 1's error shape). On the buffered path, `cmd.Output()` would return a partial `out` plus a kill-related `err`; per `Run()`'s `len(out) > 0` check, that partial content — if non-empty — still gets sent to `parseEnvelope`, which would most likely fail with a real `"parse envelope: ..."` JSON error (truncated JSON), correctly distinguishable from the empty-string case if `doFlows` actually checked `r.Err`.
- **Model hits a token/length limit before closing the JSON object** — same shape as the above: `parseEnvelope`'s `json.Unmarshal` would fail on genuinely truncated (non-empty) JSON, producing a real, actionable Go JSON error (e.g. `"unexpected end of JSON input"` — same message text as the empty-string case, meaning **today, a real truncation and a fully-empty/garbage response are indistinguishable from the error message alone**, another argument for checking `r.Err` upstream where more context exists).
- **Multiple JSON chunks concatenated / only the last fragment parsed** — not applicable to the buffered path (`map`/`flows`) since there's exactly one `cmd.Output()` read, no chunking logic exists to get confused. On the streaming path (`qa`/`bug`), each stdout line is parsed independently and only `type == "result"` lines update `final` — if two `"result"`-typed lines somehow appeared (not expected from Claude Code's protocol, one per invocation), the *last* one silently wins with no warning; low risk but worth a defensive assertion.
- **Empty stdout (crashed before writing) vs. partial stdout** — **these are currently NOT distinguished anywhere in the code.** `Run()`'s buffered path: `if len(out) > 0 { return parseEnvelope(out), nil }` treats one byte of garbage the same as a full valid envelope — it's `parseEnvelope`'s job to reject it, and it does (via `json.Unmarshal` failing), but the *caller* (`doFlows`) doesn't read the rejection reason. `parseFindings` (`worker.go:219-235`, used by `qa`) is even more silent: `jsonArrayRe.FindString(s)` on an empty or non-matching string just returns `nil` findings with **no error signal to the caller at all** — an agent that returns prose instead of the required JSON array produces the same "0 findings" result as a genuinely clean module. This is a real, separate finding-loss risk independent of item 1.

### Proposed fix

Given the actual evidence, I'd pick **hard-fail with better diagnostics, not silent retry-with-repair, for the specific gap found here** — because the diagnosis showed the *real* problem in this exact bug isn't "the model produced imperfect JSON that needs a nudge to fix," it's "the caller discarded the one piece of information (`r.Err`) that would have told it what actually went wrong." A repair-retry loop bolted on top of that would just retry a call that's failing for an unrelated, non-model reason (item 1) and burn tokens doing it. Concretely, scoped to this item only:

1. **`doFlows` should check `r.OK`/`r.Err` before calling `parseFlows`**, and surface `r.Err` in the failure reason when `!r.OK`, instead of always going through `parseFlows` and reporting *its* (potentially misleading) error.
2. **Route `flows` through `runAgent` (or an equivalent bounded-retry wrapper) instead of calling `d.Agent.Run` directly**, so it gets the same repair-retry-then-checkpoint behavior `qa`/`bug` already have, for genuine model-side malformed-JSON cases (as opposed to infra failures, which item 1 addresses at the source).
3. **`parseFindings` should return an explicit "no findings parsed" signal distinct from "genuinely zero findings"** (e.g. return `(findings, ok bool)` or a sentinel error) so `qa()` can tell a clean module apart from a response that didn't parse, and log/retry accordingly rather than silently filing zero tickets either way.

None of this is a rewrite — items 1 and 2 together are two small, targeted changes: fix the exec layer (item 1) so this class of failure stops happening at the source, and tighten the two callers (`doFlows`, `parseFindings`'s caller) so that if a real model-side malformed-JSON response ever does occur in the future, it's diagnosable and retried rather than silently swallowed.

---

## 3. Module understanding (currently singleton/global)

**Files read:** `internal/worker/qa.go` (`doMap`, `qa`, `scanModules`, `hasCode`, `qaTask`).

### What the code actually does today

`doMap` (`qa.go:40-86`) does two independent things: a **deterministic directory scan** (`scanModules`, $0, no model call) that decides how many `qa` cards to spawn and what path each one names, and a **single, repo-wide, best-effort overview call** (`mapTask`, `qa.go:462-464`) whose output is written once to `notes` and never referenced again by any other node.

**`scanModules(root)` (`qa.go:383-425`)** is pure directory-name heuristics, not import/dependency analysis:
- Reads only the **top-level** entries of the repo root.
- If a top-level entry's name is in `containers = {"src","app","apps","packages"}`, it descends **exactly one level** into it and makes each child subdirectory (that contains recognizable code) its own module.
- Every other top-level directory with code becomes a module directly, unless its name is in `skipModuleDir` (`.git`, `node_modules`, `dist`, `docs`, `.github`, etc. — 17 entries).
- `hasCode(dir)` just walks the directory recursively and returns true on the **first** file matching a known extension — a folder with a single stray `.go`/`.ts` file becomes a full module card exactly like a folder with thousands of files.
- **Hard cap of 10 modules** (`const cap = 10`, `qa.go:420-423`): `mods = mods[:cap]` silently truncates — modules 11+ get **no QA card at all**, and nothing in the UI, notes, or logs indicates truncation happened. A monorepo scanner that finds 14 top-level service directories audits 10 of them and never mentions the other 4 exist.

**The actual bug for "singleton understanding" is in how the *agent's tool access and context* work, not just the scan.** `d.qa(ctx, c, ws)` (`qa.go:88` onward) receives `ws`, which is the **same `sandbox.Workspace` for the entire run** — constructed once in `worker.RunOnce` (`ws := sandbox.Workspace{Dir: filepath.Join(d.WorkspaceRoot, c.RunID.String())}`, i.e. the repo root) and passed unchanged into every node handler regardless of type. `qa()` never builds a module-scoped workspace; it calls `ensureInstalled(ctx, ws)` and `d.runAgent(ctx, c, ws, qaTask(sp.Module, sp.Path), agent.Live)` using that same root `ws` every time. Inside `agent.Options.command()`, `dir := ws.Dir` becomes **both** `cmd.Dir` (the Bash working directory) **and** the `--add-dir` value (the tool-access allowlist) — always the repo root.

So, concretely: **every QA card gets identical, full-repo tool access** (Read/Glob/Grep/Bash can reach the entire imported codebase, not just its assigned module) — the only per-module differentiation is that `sp.Module`/`sp.Path` get interpolated into the *prompt text* (`"You are the QA engineer for the module %q (path `%s`)..."`). There is no hard boundary; a "backend" QA card can freely read/exercise "frontend" files if the model chooses to, and nothing prevents or flags it.

**Also confirmed: `qaTask(module, path string) string` takes no `notes` parameter at all** — compare to `fixTask(b store.Bug, notes string)`, which explicitly does. So the one piece of shared understanding that *does* get produced once (`mapTask`'s repo overview, written to `notes`) is **never handed to any `qa` node** — each of the (up to 10) QA calls starts completely cold and has to independently re-discover product structure and context from scratch via its own Read/Glob calls against the whole repo, every time. This is the literal "singleton, not module-scoped" behavior the item title describes: there's exactly one piece of shared understanding generated (the map overview), and it's shared with *nobody* — each QA call redundantly rebuilds its own.

### Edge cases (concrete, not generic)

- **A monorepo with 50+ top-level service directories** (e.g. a microservices repo with `services/auth/`, `services/billing/`, `services/notifications/`, ... 50 of them, none named `src`/`app`/`apps`/`packages` so no one-level descent applies): `scanModules` returns the first 10 in `os.ReadDir` order (alphabetical on Windows/most filesystems) and silently drops the rest. If the 50 services are alphabetically named `svc-01` through `svc-50`, only `svc-01`–`svc-10` (or similar) ever get audited, forever, with no signal that 40 services were never looked at.
- **Modules with circular imports** — has literally zero effect on today's algorithm, because `scanModules` does no import-graph analysis at all; it's a directory-shape heuristic. Not a bug per se, but worth noting the brief's framing ("modules with circular imports") assumes an import-graph-aware scoping model that doesn't exist yet — see the proposed fix.
- **A "module" that's really just a loose folder with no clear boundary**: a `scripts/` or `tools/` directory containing one `.sh`-adjacent `.py` helper script becomes a full QA card, competing for the same 10-module budget as a genuine `services/billing/` directory with real business logic — `hasCode`'s "stop at first match" means there's no size/substance weighting at all.
- **A Django/Rails-style two-levels-deep layout** (e.g. `myproject/apps/billing/`, `myproject/apps/auth/` where `apps` itself sits one level under a non-container-named `myproject/` top-level dir): `scanModules` only descends into a `containers`-named directory when it's a **top-level** entry — `myproject/` itself isn't `src`/`app`/`apps`/`packages`, so its `apps/` child is never inspected for one-more-level descent, and the whole `myproject/` tree collapses into a single flat module.

### Proposed fix (scoping strategy, no UI changes required)

A minimal, UI-transparent scoping strategy: give each `qa` node **its own `sandbox.Workspace` scoped to `filepath.Join(runWs.Dir, sp.Path)`** instead of reusing the run-root `ws` — this makes `cmd.Dir` and `--add-dir` for that specific `claude` invocation point at just the module's own subtree, which is "a module's own files" (the first half of the brief's suggested "own files plus one hop of its import graph"). The "one hop of its import graph" half is a larger feature (needs actual import parsing per language) and is out of scope for a minimal fix — flagging it as a natural follow-up once the file-scoping half is in, not bundling both into one change.

Alongside that: (a) pass the `notes` (map's overview) into `qaTask`'s prompt the same way `fixTask` already does, so QA cards stop redundantly re-deriving product-level context the map step already paid for; (b) when `scanModules` truncates at the 10-module cap, write a note (in the `notes` doc, one line) naming the modules that were skipped, so it's at least visible rather than silent.

---

## 4. Run-loop parallelism (currently one node per tick)

**Files read:** `internal/api/runloop.go` (`TickAll`, `StartRunLoop`), `internal/queue/queue.go` (`Claim`, `PromoteReady`), `internal/store/store.go` (connection pool setup).

### What the code actually does today

Confirmed directly from `TickAll` (`runloop.go:100-132`): it calls `deps.Queue.PromoteReady(ctx)` once, then `worker.RunOnce(ctx, deps)` **exactly once**, then `deps.Store.FinalizeDrainedRuns(ctx)`, and returns `1` or `0` (nodes processed this tick). There is no loop, no goroutine fan-out, no batch claim — `RunOnce` itself (`worker.go`) calls `d.Queue.Claim(ctx)` a single time and, if a node was claimed, dispatches and **fully processes it synchronously** before `RunOnce` returns.

Critically, "one node per tick" undersells how serialized this actually is: for a live node (`qa`/`bug`), `RunOnce` blocks on `d.Agent.Run(...)` — a synchronous call all the way down through `runAgent` → `agent.Run` → `cmd.Wait()` — which can legitimately run for **up to `liveTimeout` = 20 minutes**. `StartRunLoop` (`runloop.go:137-156`) only calls `TickAll` again, and thus only attempts the *next* `Claim()`, after the *current* `TickAll` call has fully returned. So the real cadence isn't "claim a node every ~2 seconds" — it's "claim a node, block for however long that node's entire live agent call takes (seconds to 20 minutes), then wait `every` before claiming the next one." The `every` pacing param (2s real / 1s stub, `time.Second` when idle) only actually matters between fast/instant nodes; it's irrelevant once a live call is in flight.

### Edge cases

- **Two ready `qa` nodes with no dependency between them** (the exact case in the brief — e.g. `qa · backend` and `qa · frontend`, both depending only on `map`, with no edge between each other): confirmed forced fully serial today. `Claim()`'s `ORDER BY ... created_at LIMIT 1` picks exactly one; the other sits at `status='ready'` untouched until the current node's entire live call (install deps, run tests, exercise the app, up to 20 minutes) finishes, gets `Finish()`'d, and the loop comes back around. This matches what was directly observed in this session's live debugging: `zeroprep-hyderabad`'s two QA cards (`app/api`, `lib`) logged `node.start` at `17:22:57` and `17:23:46` respectively — back-to-back, not overlapping, exactly as the code predicts.
- **What's the actual bottleneck if concurrency were added?** Evaluated all three named candidates against the real code:
  - **SQLite locking** — `internal/store/store.go:36` sets `db.SetMaxOpenConns(1)`: the whole application shares **one** database connection. This wouldn't *break* concurrent node processing (Go's `database/sql` pool just queues requests on that one connection), but it does mean every `Claim`/`Finish`/`PromoteReady`/`events.Log` call from N concurrently-running nodes would still serialize through that single connection — a real, if partial, throughput ceiling on anything beyond the *agent call itself* running in parallel.
  - **Claude CLI process limits** — there is **no concurrent-process cap anywhere in the current code**. (Note: an older README/doc pass referenced a `MAX_CONCURRENT_CLAUDE` env var; grepping the actual current source confirms it is not read or enforced anywhere today — that documentation claim is stale, not a real constraint.) So this isn't currently a bottleneck because nothing currently *allows* concurrent processes to exist in the first place, not because something's actively capping them.
  - **Cost control** — `store.PauseOverBudget` (checked once per `TickAll`, i.e. once per node today) is the only spend guard, and it only *reacts* to spend already recorded from completed nodes — it doesn't reserve/predict spend for in-flight work. With today's strict serialization, this naturally limits exposure to "at most one node's cost before the next budget check." **If concurrency were added naively, this becomes a real gap**: N nodes could all be claimed and start real, billed `claude` calls before any of them finish and get their cost recorded, so `PauseOverBudget` would only catch the overage *after* N calls' worth of spend already happened, not before.

### Proposed fix (bounded concurrency, not unlimited fan-out)

Given the SQLite single-connection constraint is a soft (queuing, not blocking) limit and the real risk is uncontrolled cost exposure, not correctness: change `Claim` (or add a new method) to claim **up to N ready nodes in one call** (N from a small config knob, e.g. `MaxConcurrentClaude`, default 2–3 to actually revive the old documented-but-unimplemented intent), and have `RunOnce`/`TickAll` dispatch each claimed node's processing in its own goroutine, `sync.WaitGroup`'d before the tick returns. Keep `PromoteReady`'s dependency ordering as-is (it already correctly handles "no dep between them" nodes being simultaneously `ready`) — the claim-order `CASE type WHEN 'import' THEN 0 ...` still holds, just claiming N rows instead of 1 in the same atomic-per-row pattern. Pair this with tightening `PauseOverBudget` to check *before* dispatching each claimed node in a batch (not just once per tick), so a burst of concurrent claims can't blow past the budget cap before the first one finishes and gets recorded. This is additive to `queue.go`/`runloop.go`/`worker.go`'s dispatch loop — no change needed to `PromoteReady`'s SQL, the node types, or the UI.

---

## 5. Finding categorization

**Files read:** `internal/store/tickets.go` (`Bug` struct, `CreateBug`), `internal/store/schema.sql`, `internal/worker/qa.go` (`qaTask`'s requested JSON shape, re-confirmed from item 1/3's reads).

### What the code actually does today

The `Bug` struct (`tickets.go:14-23`) is the entire findings schema — there is no separate SQL table for tickets; a bug is just a `nodes` row with `type='bug'` and this struct JSON-encoded into `input_snapshot`:

```go
type Bug struct {
	Title    string   `json:"title"`
	Name     string   `json:"name"`
	File     string   `json:"file"`
	Severity string   `json:"severity"` // high | medium | low
	Priority string   `json:"priority"` // P0 | P1 | P2
	Detail   string   `json:"detail"`
	Tags     []string `json:"tags"`
	Kind     string   `json:"type"` // "bug" | "feature" | "chore" (card type)
}
```

Confirmed: **no `category` or `confidence` field exists anywhere** — not in this struct, not in `schema.sql` (which has no dedicated findings table at all, `nodes.input_snapshot` is a free-form JSON blob). `Kind` looks category-shaped at first glance but isn't used that way: `qaTask`'s requested output schema (`qa.go:482-483`) is exactly `{"title","file","severity","detail"}` — the model is **never asked for a `type`/`kind` at all** — so `CreateBug`'s `if b.Kind == "" { b.Kind = "bug" }` (`tickets.go:34-36`) means every single finding defaults to `Kind: "bug"` unconditionally; the `"feature"`/`"chore"` values in the type comment are dead — nothing in the QA path ever produces them. `Tags` is the only free-form field, populated by `qa()` with exactly three entries: `"from:qa"`, `"module:<name>"`, and the severity string again — not a category taxonomy, just provenance + the module.

Yet `qaTask`'s own prompt asks the model to find **four different kinds of things in one pass**: "real bugs, correctness issues, best-practice violations, AND important test cases that are missing" (`qa.go:478-479`) — none of which is captured as structured data on the resulting ticket; which of the four a given finding actually is only exists as prose buried inside free-text `Detail`.

**Dedup is exact-match only.** `CreateBug`'s dedup query (`tickets.go:44-52`) matches on case-insensitive, trimmed `title` **and** `file` being identical:

```sql
SELECT id FROM nodes WHERE run_id=? AND type='bug'
  AND lower(trim(...title...))=lower(trim(?))
  AND lower(trim(...file...))=lower(trim(?))
```

Anything short of an exact title+file match — different wording, or the same root cause surfacing at a different file location — is **not** deduplicated; a new ticket is filed every time.

### Edge cases

- **A finding that's simultaneously a security issue and a missing-test gap** (e.g. QA finds an unauthenticated endpoint *and* notes there's no test covering auth at all for it): today this is filed as **one** ticket with `severity:"high"` and both aspects only distinguishable by a human reading the `Detail` prose — there's no way to query "show me the security findings" separately from "show me the test-coverage gaps" on the board or via the API, because that distinction was never asked for or stored.
- **Near-duplicate findings filed across different modules for the same root cause** (the brief's exact scenario, concretely: a shared validation helper in `lib/` has a bug, and QA cards for both `backend` and `frontend` independently notice symptoms of it — `backend`'s finding might have `file:"backend/handlers/user.go:42"` and `title:"user creation accepts empty email"`, while `frontend`'s might have `file:"frontend/src/api/user.ts:18"` and `title:"signup silently succeeds with blank email"` — same root cause, completely different title text and file, so the exact-match dedup **does not catch this at all**; two separate tickets, two separate dev-fix attempts, potentially two different (possibly conflicting) patches to the same underlying bug.

### Proposed fix (minimal schema addition, no UI work required)

Add two optional fields to `Bug` (`tickets.go`) and thread them through `CreateBug` exactly like `Severity`/`Priority` already are — no new SQL table needed since everything already lives in the JSON blob:

```go
Category   string `json:"category,omitempty"`   // "bug" | "correctness" | "best-practice" | "test-gap" | "security"
Confidence string `json:"confidence,omitempty"`  // "high" | "medium" | "low" — how sure the model is
```

Update `qaTask`'s requested JSON shape (`qa.go:483`) to ask for `"category"` and `"confidence"` per finding alongside the existing four fields — a one-line prompt change, no code-path restructuring. `qa()` (`qa.go:106-` onward, where `store.Bug{...}` is built from each parsed `finding`) just needs the two new fields copied across, same as `Severity`/`Priority` are today. This directly enables filtering/reporting by category later (board, export) without any of that being required as part of *this* change — the brief only asked for the data model + prompt change, not new UI, which this respects.

For the duplicate-root-cause edge case: exact title+file matching is fundamentally the wrong tool for catching cross-module duplicates (they'll rarely share a file). A minimal, still-scoped improvement: extend `CreateBug`'s dedup check to also look for an existing *unresolved* ticket in the same run whose `Detail` shares a high enough substring/keyword overlap with the incoming one — but this is a judgment call on threshold/approach worth a design decision rather than a blind implementation; flagging it as identified but **not proposing a specific mechanism to apply**, since a naive fuzzy-match could just as easily suppress two genuinely distinct bugs that happen to read similarly.

---

## 6. Multi-provider support (Codex, OpenRouter, etc.)

**Files read:** `internal/worker/worker.go` (`Agent` interface), `internal/agent/agent.go` (`Result`, `Mode`, `PolicyFor` — re-confirmed from item 1's read), `internal/api/runloop.go` (`realAgent`, `stubAgent` — re-confirmed from item 4's read).

### What the code actually does today

The seam `Worker` depends on is exactly this, verbatim from `worker.go:29-31`:

```go
type Agent interface {
	Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode, onStep func(string)) (agent.Result, error)
}
```

I traced every call site in `worker`/`qa.go`/`flows.go` that touches this boundary: they only ever pass a plain `task string` and one of `agent.Mode`'s two values (`ReadOnly`/`Live` — myAudit's own enum, `agent.go:66-69`), and only ever read back `agent.Result`'s five fields (`OK`, `Summary`, `CostUSD`, `Tokens`, `Err` — all provider-neutral types: bool, string, float64, int, string). **`Worker` never imports or references anything Claude-Code-specific** — no tool names, no CLI flags, no envelope JSON shape. The provider-specific mechanics (`PolicyFor(mode)`'s `--allowedTools`/`--disallowedTools` Claude tool names, `exec.Command("claude", ...)`, `--output-format`, envelope parsing) all live inside `internal/agent` and are only invoked from `realAgent.Run` in `runloop.go` — one level removed from `Worker` entirely. So: **yes, the boundary is genuinely generic enough** to plug in a second `Agent` implementation without touching `internal/worker` at all, *provided* the new provider can express "read-only" vs. "live, file-editing + shell" as two coherent modes, and can surface its own final response as a plain string ending in the JSON shape `parseFindings`/`parseFlows` already expect (both are pure downstream parsers over `Result.Summary`, provider-agnostic by construction).

### Edge cases

- **Different tool-call formats (function calling vs. Claude's `tool_use`)** — isolated entirely inside whatever new type implements `Agent`; `onStep func(string)` is already a minimal, provider-agnostic "human-readable step" callback (`describeTool` in `agent.go` is Claude-specific *today*, but it's private to `internal/agent`, not part of the interface contract) — a new provider's implementation owns translating its own tool-call wire format into that same simple string, with zero interface change.
- **Different auth models (API key vs. CLI login session)** — also fully isolated; `realAgent` today reads nothing about auth at all (Claude Code CLI handles its own login state out-of-process). A new implementation for an API-key-based provider would read its key from env/config inside its own constructor, same pattern as `realAgent{store, isolate, image}` today.
- **Different JSON/structured-output conventions per provider** — as long as the new implementation's `Run()` ultimately hands back the model's final text as `Result.Summary`, the existing `parseFindings`/`parseFlows` work unchanged, since they only ever operate on that string, not on any provider's native envelope shape.

**One important asymmetry the brief's edge cases don't surface, but the code does:** Claude Code CLI is not just an API client — it's a **complete, self-contained agentic coding tool** (it owns its own Read/Write/Bash/Glob/Grep tool execution, sandboxing, and permission model; myAudit just drives it via CLI flags and reads its final JSON envelope). A provider like **OpenRouter is not this** — it is a raw chat-completions proxy with no built-in file/shell tool-execution loop at all. Adding OpenRouter as an `Agent` wouldn't be "write a new thin CLI-shim like `realAgent`" — it would require myAudit to **implement its own entire agentic tool loop from scratch**: define Read/Write/Bash/Glob/Grep as function-calling tool schemas, send them with every request, parse `tool_calls` from the response, actually execute those tools locally against the sandboxed workspace, feed results back as follow-up messages, and loop until a final non-tool-call response — a materially larger project than a spike, and one with real new security surface (myAudit, not the provider's CLI, would now own sandboxing/confinement of arbitrary Bash execution).

### Proposed spike (one provider, not full multi-provider support)

Given that asymmetry, **Codex CLI is the more tractable spike target** if it ships an analogous non-interactive/print mode with its own native file+shell tool access (mirroring Claude Code's `-p --output-format json`) — that shape can be implemented as a near drop-in second `internal/agent`-style package (e.g. `internal/agent/codex`, or a `provider` field threaded through `Options`) exposing the same `Run(ctx, ws, task, mode, onStep) (Result, error)` signature, with its own `command()`/`Args()`/envelope-parsing tailored to Codex's actual current CLI flags and output shape (I have not verified Codex CLI's exact current flag names/JSON envelope against the live code, so that mapping would need to be confirmed against Codex's own docs before implementation, the same way this audit confirmed Claude Code's actual flags by reading `agent.go` rather than assuming them). `runloop.go` would then need a small `resolveAgent()`-style branch (mirroring `resolveModel`'s existing pattern) to pick `realAgent` vs. a new `codexAgent` based on a config value — a small, additive change, not a rewrite. **OpenRouter is explicitly out of scope for this spike** given the tool-loop gap above; it's a distinct, larger feature deserving its own design pass, not something to fold into "add a second provider."

---

## 7. Making it lighter

**Files read:** `internal/worker/qa.go` (`mapTask`, `qaTask`, `fixTask` prompts — re-confirmed from items 1/3/5), `internal/worker/flows.go` (`flowsTask`), `internal/agent/agent.go` (`Options`, `Args` — re-confirmed from item 1), `internal/worker/worker.go` (`runAgent`'s retry logic).

### What actually gets sent to Claude as context per node today

None of the prompts (`mapTask`, `qaTask`, `flowsTask`, `fixTask`) embed file contents, a repo tree, or any pre-gathered context directly — they're all short-to-medium instruction strings (the longest, `qaTask`, is ~1500 characters). The actual code-reading happens **agentically**: each node's `claude -p` invocation gets `Read`/`Glob`/`Grep` (and `Bash` for `Live` mode) tool access via `--allowedTools`, and the model itself decides what to read, turn by turn, inside that one session — myAudit never stuffs file contents into the prompt itself. So per-node cost is driven by however much the model chooses to Glob/Read/Grep during its own turns, not by anything myAudit constructs up front.

**The actual lightness gap is cross-node, not within a single node.** I checked whether any session/context reuse exists across the (up to) 1 map-overview + 1 flows + up to 10 qa + N bug calls a single run makes: `agent.Options` has `SessionID`/`Resume` fields explicitly intended for this (`agent.go:85-86`, comment: *"set for repair continuity"*, and `Args()` correctly emits `--resume <id>` or `--session-id <id>` when set — confirmed working, `agent_test.go:65-69` even tests it) — **but grepping every call site that constructs `agent.Options` in the actual run path (`realAgent.Run` in `runloop.go`) shows `SessionID` is never set.** Every single node — including the `qa` node's retry-with-repair attempts inside `runAgent` (`worker.go:148-162`, which appends the prior failure text and re-calls `d.Agent.Run` with a brand-new call, not `--resume`) — is a **completely fresh, sessionless `claude -p` invocation**. Confirmed via item 3's finding too: `qaTask` doesn't even receive `notes` (the map overview), so there's no shared context of any kind, structural or session-level, between any two of these calls, even when they're auditing the same repo minutes apart.

Concretely, for one real run against a repo with, say, 5 modules: that's 1 (map overview) + 1 (flows) + 5 (qa) + however many bug fixes = 7+ **entirely independent** Claude sessions, every one of which may re-Glob the repo root, re-read the same root `README.md`/`package.json`/config files, and re-derive the same "what does this codebase do" context the map step already paid for and wrote to `notes` — just never handed it to anyone.

### Edge cases

- **Large binary or generated files being read unnecessarily** — `sandbox.excludeDirs`/`skipModuleDir` (confirmed in items 1/3's reads) filter out well-known heavy directories (`node_modules`, `.venv`, `dist`, etc.) at the *copy* and *module-scan* level, but that's a directory-name denylist, not a file-content/size guard — nothing stops the live agent's own `Glob`/`Read` tool calls (inside its Bash-enabled session) from reading a large generated file, lockfile, or binary asset that happens to live outside those excluded directories (e.g. a large `.sql` seed file, a committed `dist/bundle.js` that wasn't caught because it's not literally named `dist/`, or a large fixture file under `test/fixtures/`). This is bounded only by the model's own judgment, not by myAudit.
- **The same repo structure being rediscovered independently by QA and fix nodes for the same module** — confirmed directly above: a `bug` node fixing a ticket filed by a specific `qa` module card gets `fixTask(bug, notes)` (which *does* get `notes` — unlike `qaTask`), but even so, it's a fresh session with no access to the *specific* `qa` session's file reads/context that originally found the bug — it re-discovers the relevant code from scratch via its own Read/Glob calls, informed only by the ticket's `Detail` text and the shared `notes` log, not by anything the original QA session actually saw.

### Proposed fix (caching/reuse, ties into item 3)

Two independent, additive levers, both already half-supported by existing-but-unused fields:

1. **Wire up the already-present `SessionID`/`Resume` mechanism** for the one case where it's most clearly safe and valuable: a `bug` node fixing a ticket, `--resume`'d from *that ticket's originating `qa` node's session* (would require persisting the `qa` session's id on the `Bug`/ticket — a small store schema addition, same shape as item 5's proposed fields) — the fix session starts with the QA session's already-built-up context of that specific module, instead of re-discovering it from zero.
2. **Pass `notes` into `qaTask` the same way `fixTask` already receives it** (this is the same concrete fix proposed in item 3, restated here because it's the most direct "stop redundant rediscovery" lever available with the least risk — a one-line prompt change, no new session-continuity plumbing needed).

Both are additive to `qa.go`/`agent.go`'s existing option-passing pattern — no restructuring of the module scan, the prompts' core instructions, or the parsing logic that already handles `Result.Summary`.

---

## 8. Best-practice / common-issue sources

**Files read:** `internal/worker/qa.go` (`qaTask`, full text re-confirmed from items 1/3/5/7), repo-wide search for any checked-in rules/reference/best-practices file.

### What the code actually does today

`qaTask`'s prompt asks the model to find "real bugs, correctness issues, **best-practice violations**, AND important test cases that are missing" (`qa.go:478-479`) — that's the entirety of the guidance given. I searched the whole myAudit repository for anything resembling a checked-in rules file, best-practices reference, or per-language lint/rule config belonging to *myAudit itself* (as opposed to a target repo's own tooling) — there is none. The only match for rule/lint-shaped filenames anywhere in the tree is `eslint.config.mjs` **inside a target repo's copied workspace** (`runs/<id>/eslint.config.mjs`) — i.e. the *audited project's own* lint config, not anything myAudit ships or references. **Confirmed: QA's "best practice" judgment comes purely from the model's own training knowledge — there is no versioned, checked-in source of truth it's instructed to consult at all.**

### Edge cases

- **A language-specific best practice misapplied to the wrong language in the same repo** — since there's no reference doc at all (let alone a per-language one), there's nothing in myAudit's own code that could route a Python-specific rule at a `.py` file versus a JS/TS-specific rule at a `.ts` file — that routing, to the extent it happens at all, depends entirely on the model correctly inferring language boundaries from the file paths it happens to read within one module's undifferentiated (per item 3) full-repo tool access. A concrete failure shape: a monorepo module mixing a Python backend and a TypeScript frontend under one QA card (per item 3's finding that `scanModules` module boundaries are directory-shape heuristics, not language-aware) could plausibly get a `detail` citing a Python idiom (e.g. "use a context manager") against a `.ts` file, or vice versa, with nothing in myAudit's own prompt or data model to catch or prevent it.

### Proposed fix (minimal, versionable source)

A checked-in, per-language/per-category reference doc that `qaTask` explicitly cites and is told to prefer over implicit model knowledge when the two conflict — concretely: a small `internal/worker/rules/` directory (or a single `rules.md` alongside `qa.go`) with short, versioned sections (e.g. `## Go`, `## TypeScript/React`, `## Security` — whatever set is actually useful, a decision for you rather than something to guess here), loaded once and either (a) embedded directly into `qaTask`'s prompt text when the relevant language is present in the module being audited, or (b) handed to the model as an additional `--add-dir`-visible reference file it can `Read` itself if instructed to check it first. Either mechanism is a small, additive change to `qaTask`'s construction — no change to the parsing/ticket-filing path, since the output schema (`{"title","file","severity","detail"}`, plus item 5's proposed `category`/`confidence`) doesn't need to change to support this; it only changes what informs the model's judgment, not what it's asked to report back.

---

## Summary

### Safe to apply immediately, low risk

- **Item 1** (process-launch failure) — the actual, empirically-confirmed root cause on this machine: real QA/flows/fix prompts (long, quote/backtick/JSON-dense strings) break when passed as a raw CLI argument to the Windows `.cmd` shim `claude` resolves to, due to Go's `os/exec` `.bat`/`.cmd` argument-escaping layer. Fix is scoped to `internal/agent/agent.go`'s `command()`/`Args()`/`Run()`: deliver the task via stdin instead of as a `-p <task>` argument (myAudit's own code already documents that Claude Code CLI supports this — the `cmd.Stdin = nil` line's comment says so). No change needed anywhere else. This is the confirmed cause of every real-mode run failing on this machine to date.
- **Item 2** (invalid/truncated JSON) — mostly a downstream *symptom* of item 1 in the cases actually observed this session, but two small, independently-safe hardening changes stand on their own regardless of item 1's fix: make `doFlows` check `r.OK`/`r.Err` before parsing (currently silently discarded), and route `flows` through the same bounded-retry wrapper (`runAgent`) `qa`/`bug` already use instead of calling `Agent.Run` directly. Both are narrow, isolated to `flows.go`.

### Need a design decision from you before any code change

- **Item 3** (module scoping) — real fix means giving each `qa` node its own sub-workspace (`ws.Dir` = the module's own path, not the run root) — a behavior change to what the agent can see/touch per card, worth confirming you want before implementing, plus a decision on whether the 10-module cap should just be raised, made configurable, or paired with the proposed "skipped modules" note.
- **Item 4** (concurrency) — bounded-concurrency claiming is a real architecture change to the run loop + a new config knob; also needs a decision on the default concurrency level and how strictly the budget check should gate a concurrent batch.
- **Item 5** (categorization) — the `category`/`confidence` field *names and values* are a judgment call (I picked plausible ones, not final); the cross-module dedup improvement was explicitly flagged as needing a design decision on matching strategy, not a specific mechanism to implement blind.
- **Item 6** (multi-provider) — needs you to confirm Codex CLI is the right first target (and its actual current CLI surface verified against its own docs, which I have not done), and explicit confirmation that OpenRouter is being deferred as a separate, larger effort rather than folded in.
- **Item 7** (lighter) — wiring up `SessionID`/`Resume` for bug-fix continuity requires a small schema addition (persisting a `qa` node's session id onto its tickets) — a data-model change worth your sign-off before implementing, even though it's small.
- **Item 8** (best-practice sourcing) — genuinely open-ended: the actual rule content, categorization, and delivery mechanism (embed-in-prompt vs. `Read`-able reference file) are all decisions for you; I've only confirmed the current state (no such source exists) and sketched the shape of a fix.

No fixes have been applied. No commits made.

