# myAudit — Productization Backlog

Deduped findings from a four-lens deep audit of our own codebase (onboarding,
core workflow, UX, code health). Prioritized by **user impact**, not cost.
Tags in brackets = which lenses flagged it (cross-confirmed items rank higher).

## Progress

- **Batch 1 — trust & correctness (done):** #3, #4, #7, #10, #19, #20, #21, #22, #27.
- **Batch 2 — autonomous-fix trust loop (done):** #8, #9 (reopen; auto-revert
  deferred), #11, #12, #13.
- **Batch 3 — control & real-repo safety (done):** #2, #6, #14 (PR deferred),
  #17, #18 (edit-race serialization deferred).
- **Next — Batch 4 (live feedback & onboarding):** #1 (streaming, the last P0),
  #15 (claude preflight), #16 (import UX), #24, #25, #26.

Legend: **P0** breaks/blocks core value · **P1** major friction, missing
table-stakes, trust, or real-repo failure · **P2** polish / cleanup / robustness.
Size = rough effort (S/M/L).

---

## P0 — breaks the core value

1. **Dead air during live QA/fix runs — no streaming, no progress.** [ux]
   Agent nodes are one blocking `claude -p` call (`agent.go:136,222`); between
   `node.start` and completion, *zero* events fire, so a running card sits static
   for minutes on the board — looks hung. The product's whole promise is "LIVE,
   minutes-long QA."
   Fix: `--output-format stream-json`, forward tool/assistant deltas as events,
   render a live "current step" + spinner on the running card. **L** (interim: heartbeat events + spinner, **M**).

2. **No way to stop / cancel / pause a running audit.** [workflow]
   One global loop on `context.Background()` (`main.go:52`, `runloop.go:88`); no
   cancel route. A live node runs the target's code + `npm install` on the host
   for up to 20 min; the only recourse is killing the binary (which wedges the
   node — see #6).
   Fix: `POST /api/runs/{id}/cancel` + per-run cancel flag + per-node
   `CancelFunc`; Stop button on the header pill. **M**

---

## P1 — trust, table-stakes, real-repo failures

### Trust: the UI reads as unfinished / fake

3. **"myIntern" leftovers + fabricated Settings.** [onboarding][ux][code-health]
   Splash literally says **"myIntern"** (`Splash.tsx:40`). Settings hardcodes
   fake account + integrations incl. **"Claude CLI: connected"** even when it
   isn't (`SettingsScreen.tsx:17,27`) — lies exactly when a user is debugging #10.
   Stale copy ("steer the intern", "generate in Config", "No projects — use
   Config") across `DevScreen.tsx:124`, `PromptBar.tsx:54`, `WorkspaceSwitcher.tsx:23`,
   `NotesScreen.tsx:93`, `ConfigScreen.tsx:38`, `PlaywrightScreen.tsx:11`.
   Dead "Sign out" (`AccountMenu.tsx:15`). Doc comment says "over Postgres"
   (`main.go:1`); git author is "myIntern" (`sandbox.go:102`).
   Fix: rename splash, copy pass to audit vocabulary, make Settings reflect real
   state or hide it. **S–M**

4. **Model selector is a no-op — audits silently run on Haiku.** [onboarding][workflow]
   Settings saves `model_tier` (UI default opus-4.8) but **nothing reads it**;
   model comes only from `CLAUDE_MODEL` env at boot (`runloop.go:46`). User picks
   Opus, sees "saved", still gets Haiku.
   Fix: accept `model` per run + wire Settings → `agent.Options.Model`. **M**

5. **Dead controls erode trust.** [workflow][onboarding]
   `POST /steer` only logs, no worker consumes it (`api.go:282`). Checkpoint
   Confirm/Skip requeues but the answer is never fed to the retry
   (`checkpoints.go`, `worker.go:213`). `MaxConcurrentClaude` knob does nothing
   (single loop). The `fix` chat command is undiscoverable and fires on *any*
   message starting with "fix" (`chat.go:94`).
   Fix: implement or remove each; tighten `isFixIntent` to an explicit `/fix`;
   surface commands in the chat composer. **S–M**

### Trust: acting on the autonomous fix

6. **No recovery for a node stuck `running` (crash/restart wedges the run).** [workflow][code-health]
   `Claim` only takes `ready`; `claimed_at` is written but never read; `Requeue`
   exists but is called nowhere. A crash/kill mid-node strands it `running`
   forever — and a stuck `qa` blocks all its module's fixes (dep-gated).
   Fix: on startup/tick, requeue `running AND claimed_at < now-lease` (bounded by
   attempts); real signal handling in `main`. **S–M**

7. **Run status never flips off "running" — no completion signal.** [workflow][code-health]
   `runs.status` defaults `running` and **nothing ever updates it**; the detail
   handler doesn't even load the run row (`api.go:81`). Every past audit shows
   "running" on Home; no done toast / notification.
   Fix: persist terminal status when the graph drains; fire `run.done` + toast /
   Tauri notification; load the real run row. **S–M**

8. **No diff of an autonomous fix; "Accept Diff" shows the full file.** [workflow][ux]
   The sandbox computes a real unified diff (`sandbox.go:67`) but **no endpoint
   exposes it**; the UI shows post-fix file content (`DevScreen.tsx:120`). You
   can't see what a fix changed without leaving the tool.
   Fix: `GET /api/runs/{id}/diff?path=` + a diff view; relabel button. **S–M**

9. **Auto-close has no approval gate and Done can't be reopened.** [workflow]
   Green regression → `done` with no human step; `ReopenableBugs`/`canRefix`
   exclude `done` (`tickets.go:71`, `KanbanScreen.tsx:78`). A wrong fix that
   passed a thin suite is permanently un-actionable in-product.
   Fix: optional "review before close" mode (park green fixes in Review w/ diff +
   Approve/Reject); allow reopening `done` (revert commit on reopen). **M**

10. **Swallowed status write → possible false "fixed" (green).** [code-health]
    In `bug`, `complete()` sets `done` first, then `_ = SetNodeStatus(failed/in_review)`
    fire-and-forget (`qa.go:150,170,174`). If that write fails, a failed fix shows
    green — breaking the "never a false fixed" promise.
    Fix: make `Complete` take the final status (one checked write). **S**

### Trust: the findings themselves

11. **Duplicate findings become separate cards (rel=noreferrer ×3).** [ux][workflow]
    `CreateBug` is a plain INSERT, no dedup (`tickets.go:30`). Same issue across
    modules → N identical cards; fixed/triaged multiple times.
    Fix: dedup on normalized (title|file) at file time, or cluster with a count. **M**

12. **No board filter / sort / search.** [workflow]
    4 columns by status, ordered by created_at; no filter by severity/priority/
    module/tag. A real audit = ~80 cards dumped in one list; can't ask "show P0s".
    Data (`severity/priority/tags`) is already on the card.
    Fix: client-side filter/sort bar over `cards`. **S–M**

13. **No real triage — can't dismiss / won't-fix / change severity.** [workflow]
    Only free-text tags + re-run; no severity/priority setter, no dismiss, no
    manual move. LLM findings include false positives that never go away.
    Fix: `PATCH /nodes/{nid}` for severity/priority/status incl. `dismissed`
    (filtered out); drawer controls. **M**

14. **No export / report (MD / JSON / PR).** [workflow]
    No export endpoint; report is a mutable notes blob. Results can't leave the
    tool into a tracker / PR / CI, despite `fix` producing commits.
    Fix: `GET /report.md` + `/findings.json`; "Create PR" (push run branch +
    `gh pr create`). **M**

### Onboarding cliffs

15. **No preflight for the `claude` CLI (missing / not logged in).** [onboarding]
    Nothing checks `claude` exists/authed; import ✓ then `map` dies, reason buried
    in a red card drawer, header looks idle → app appears silently dead. This is
    the most common fresh-machine state.
    Fix: `claude --version` + auth probe on boot / `/api/health`; banner with
    remediation; persist node-fail reason onto the card summary. **S–M**

16. **Browser `make run` has no folder picker + weak path validation.** [onboarding][ux]
    "Browse…" is desktop-only (`ConfigScreen.tsx:50`); the headline path forces a
    hand-typed absolute path; relative paths resolve against the *server* cwd; bad
    input → generic "not a directory" with no "must be absolute" hint.
    Fix: require/verify absolute path w/ actionable errors; recent-paths /
    drag-a-folder; inline validation. **M**

### Live-path robustness (real, messy repos)

17. **Orphaned dev-server processes on timeout → cascading port collisions.** [code-health]
    `CommandContext` SIGKILLs only the `claude` child; the dev server it spawned
    is orphaned and keeps its port. Module B's QA then can't bind → false
    "not runnable"; orphans accumulate. Under `AGENT_ISOLATE`, containers leak too.
    Fix: `SysProcAttr{Setpgid:true}` + kill the process group; `docker run --name`
    + `docker rm -f` in defer; `cmd.WaitDelay`. **M**

18. **Chat runs a Bash agent un-timeout'd, un-isolated, concurrent with the loop.** [code-health]
    `chatReply` calls `agent.Run` with `LiveAllow` directly (`chat.go:117`): no
    timeout, ignores `AGENT_ISOLATE` (escapes the jail the README promises), and
    runs inside the HTTP handler → two `claude` Bash procs + racing edits/commits
    on the same workspace/git tree during an active run.
    Fix: route chat through `worker.Deps.Agent` (inherits model+isolate+timeout);
    serialize against the loop or run as a node. **M**

19. **`map` step's agent call has no timeout.** [code-health]
    `doMap` uses the loop ctx with no deadline (`qa.go:39`); a hung read-only map
    call wedges the whole (global, single-threaded) loop. On the critical path of
    every run.
    Fix: wrap in `context.WithTimeout`. **S**

20. **Events query returns the *oldest* 200 → live log freezes mid-run.** [workflow][code-health]
    `ORDER BY ts LIMIT 200` ascending (`read.go:225`). Past 200 events, Activity,
    per-card log, Verify, and **chat replies** (chat re-reads events) all stop
    updating — the "talk to your code" feature silently dies mid-run.
    Fix: `ORDER BY ts DESC LIMIT N` then reverse (keep newest) / paginate; render
    chat reply optimistically. **S**

### Debt / correctness

21. **Dead half of the worker kept green by tests.** [code-health]
    `understand/testgen/verify/review` handlers + prompts + the `Write` mode +
    `Default*` policies are unreachable in the live graph but still compiled and
    "covered" by 3 tests → false confidence; every refactor must keep them alive.
    Fix: delete handlers/cases/prompts/`Write` mode + rewrite the tests; keep only
    shared helpers. **M**

22. **Core "green → auto-close" branch is untested.** [code-health]
    `TestQALedFlow` only hits the not-runnable path; `classifyTests` `testPass`/
    `testFail` (the "fixed vs broken" decision) and `scanModules` (the fan-out
    heuristic) have no coverage.
    Fix: table tests for `classifyTests` (codes 0/1/127/not-runnable) + `scanModules`
    over fixture trees. **S–M**

23. **Embed-dist footgun — stale/blank UI ships.** [code-health]
    `//go:embed internal/api/web/dist` is a hand-copied, gitignored-source artifact
    synced only by `make ui-build`; `run/dev/test` don't depend on it, so a stale
    `index.html` (content-hashed bundles) → blank UI. Already happened once.
    Fix: make `run/dev/build` depend on `ui-build`, or CI `git diff --exit-code`
    guard; better, build UI in CI and stop committing the artifact. **M**

24. **"Verify" tab is misleading — usually empty, says "All clear".** [ux][onboarding]
    Filters events for `verify|checkpoint|node.fail`; most runs emit none → a
    falsely reassuring "All clear" under leftover "features generated" copy.
    Fix: repurpose to per-module QA pass/fail + regression + previews, or remove;
    at minimum change empty state to "No verification events yet". **M**

25. **Accessibility: core controls are mouse-only `<div onClick>`.** [ux]
    Tabs, cards, drawer close, explorer rows, menus are non-focusable divs — no
    role/tabIndex/keys; drawer has no Esc/focus-trap; error toasts auto-dismiss in
    3.6s with no `aria-live`/dismiss. Keyboard/AT users can't run the core flow.
    Fix: real `<button>`s (or role+tabIndex+onKeyDown); Esc + focus-trap; sticky
    dismissible error toasts w/ `aria-live`; visible `:focus-visible`. **M**

26. **No deep-linking / state persistence — reload drops you to Home.** [ux]
    `runId`/`tab` are React state only; a reload (or Tauri reload) loses the open
    run/tab; can't share a link to a finding.
    Fix: reflect runId/tab (+card) in URL hash/router; restore on load. **M**

---

## P2 — polish, cleanup, robustness

27. **Card age timer never freezes ("1197m" on Done cards).** [ux]
    NOT a timezone bug (backend emits correct UTC `Z` — verified). `since()` is an
    unbounded age counter shown with a Clock icon even on terminal cards → reads
    "stuck for hours". Fix: freeze at terminal (show created→done duration) / hide
    on done; treat zone-less strings as UTC. **S**

28. **Failed `qa`/`map` strands dependents in `pending` forever.** [code-health]
    `PromoteReady` needs all deps `done`; a `failed` dep never promotes its
    children, with no board signal. Fix: promote-on-terminal or cascade to
    `blocked`/`failed` + surface it. **M**

29. **Four divergent skip-dir lists; tree walk balloons after QA.** [code-health]
    `views.skipTreeDir` (walked every 2s) is the leanest (5 dirs) and misses
    `.venv`/`target`/`.gradle` that LIVE QA *regenerates* → tens of thousands of
    entries returned every poll on Python/Rust/Java repos. Fix: one shared exclude
    set; consider lazy tree from the DB `changed` set. **S–M**

30. **install failures swallowed / mislabeled; install at repo root.** [code-health]
    `ensureInstalled` discards npm/pip error output; a genuinely broken install
    later reads as "tests not runnable → in_review" instead of surfacing. Runs at
    repo root, wrong for monorepo modules. Fix: capture+log output; distinguish
    "install failed" from "no runner"; install in module path. **M**

31. **`make dev` files zero tickets but still runs `npm install`; `make seed` hidden.** [onboarding]
    The cautious first try yields a barren board + an unexpected install (contra
    "$0, UI only"), while the command that populates a rich board free (`make seed`)
    is undocumented. Fix: document `seed`; note stub files no tickets; skip
    `ensureInstalled` under the stub. **S**

32. **`make desktop` needs Rust/cargo-tauri, not in prereqs; stale server note.** [onboarding]
    Fix: add Rust + `cargo install tauri-cli`; reconcile auto-start wording. **S**

33. **Empty repo / missing `git` → cryptic import failure.** [onboarding]
    `git commit` "nothing to commit" surfaced verbatim. Fix: detect no-source →
    "No source files found to audit"; preflight `git`. **S**

34. **Port 7788 busy — fatal for `make run`, silently wrong for desktop.** [onboarding]
    Hardcoded in 4 places; desktop guard opens a window at a *stranger's* :7788.
    Fix: clear bind-failure message / auto-pick a free port. **S**

35. **No top-level "failed" signal; failed folds into Review only.** [onboarding]
    Header pill knows only running/queued; after a root failure the app looks idle.
    Fix: failed indicator on the pill + error toast when import/map fails. **S**

36. **Board not responsive; long titles don't truncate; dead stylesheets.** [ux]
    `.board` fixed `repeat(4,1fr)`, columns don't scroll independently; `.kt` no
    clamp; `App.css`/`index.css`/`tokens.css` are never imported so **all `@media`
    rules are dead** (app ships zero responsive CSS) + a second unused token system.
    Fix: min-col-width + per-column scroll; clamp titles; delete dead CSS / one
    token system. **S–M**

37. **Explorer goes all-green at scale; no changed-only view/count.** [ux]
    A broad fix tints most of the tree green → "changed" loses meaning; no
    "N changed" or changed-only filter. Fix: a "Changed (N)" section/toggle. **M**

38. **Chat draft lost on send error; blocking, no streaming/cancel.** [ux]
    `send()` clears the draft before `await`; on error the text is gone. Fix:
    restore on error (clear on success); stream / show elapsed + cancel. **S–M**

39. **Config sprawl.** [code-health]
    `config` package omits the live knobs (read ad-hoc via `os.Getenv` in 4 files)
    and includes a dead one. Fix: centralize env reads / delete dead knob. **S–M**

40. **`prefers-reduced-motion` unhandled; unskippable 3.4s splash.** [ux]
    Fix: gate animations on reduced-motion; click/Esc to skip splash. **S**

41. **Base-fork leftovers.** [code-health]
    Stale doc comments (`worker.go:1`, `sandbox.go:1`); unused `Resource`/`Field`
    type (`graph.go:13`). Fix: rewrite comments; delete if unreferenced. **S**

42. **Investigate the stray "aux" run** on Home (unexpected; origin unknown). [self]
    — resolved: it was an old run stuck on the perpetual-"running" bug (fixed #7);
    it now finalizes to a real end state.

43. **Regression precision** — the dev-fix gate runs the repo-wide `npm test`, so
    one ticket's fix can be marked `failed` due to an unrelated failing test (or a
    thin/absent test setup). Scope the regression to the changed module, or treat
    "no real test for this change" distinctly from "this change broke a test."
    Surfaced while verifying streaming on a contrived tiny repo. [self] — P2

---

## Recommended fix order

- **Batch 1 — trust & correctness quick wins (mostly S):** 3 (rename/de-fake),
  4 (wire model), 20 (events newest), 10 (atomic status), 19 (map timeout),
  7 (run status + done toast), 27 (freeze timer), 21 (delete dead worker),
  22 (classifyTests tests). High trust-per-effort, low risk.
- **Batch 2 — the autonomous-fix trust loop:** 8 (diff endpoint+view), 9
  (approve/reopen), 11 (dedup), 12 (filter/sort), 13 (triage).
- **Batch 3 — control & real-repo safety:** 2 (cancel), 6 (stuck recovery),
  17 (process groups), 18 (chat isolate/timeout), 14 (export/PR).
- **Batch 4 — live feedback & onboarding:** 1 (streaming — P0 but larger),
  15 (claude preflight), 16 (import UX), 24 (Verify tab), 25 (a11y), 26 (routing).
- **Batch 5 — P2 polish/cleanup:** the rest.
