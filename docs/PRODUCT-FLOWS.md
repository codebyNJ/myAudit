# myAudit — Product-flow gaps (pitch POV)

Not code defects — **flows a prospective user expects the moment the product is
pitched to them, and can't do today.** Ordered by how badly the gap undercuts the
pitch. Each: *what I'd expect → what happens today → the flow to build.*

## 🔴 Make-or-break — the pitch collapses without these

### F1. Get the fixes into MY repo (the deliverable)
- **Expect:** "It fixed 12 bugs" → I download a patch / open a PR / apply to my repo.
- **Today:** fixes live as commits inside the throwaway `runs/<id>/` copy. Export
  gives `report.md` + `findings.json` — the *words*, not the *code*. There is no
  way to get the actual changes out. An auto-fixer you can't extract fixes from
  is a demo, not a tool.
- **Flow:** per-fix and whole-run **"Download patch (.diff)"**, **"Apply to
  original repo"** (write the diff back to the source path, guarded), and
  **"Open PR"** (push branch + `gh pr create`). Start with the patch — it's the
  smallest thing that makes the output real.

### F2. "What will this cost me?" — estimate + cap
- **Expect:** before I spend, a ballpark ("~$N for M modules on Sonnet") and a
  hard **budget cap** ("stop at $10").
- **Today:** cost only shows *after*, accruing live with no ceiling. The Sonnet
  run just cost ~$9 with no forewarning. Cost is the #1 question every buyer asks.
- **Flow:** pre-run estimate on the import screen (modules × model rate); a
  `budget_usd` on the run that the loop checks before claiming the next node and
  parks the rest as "paused — budget reached."

### F3. "Audit only — don't touch anything yet"
- **Expect:** a first run that just *finds* things; I decide what to fix.
- **Today:** it always QAs **and** autonomously fixes. No "findings-only" mode.
  A cautious first-time user wants the report before it starts editing (even a
  copy) and spending fix-budget.
- **Flow:** a mode toggle at import — **Audit only** / **Audit + fix** — and a
  per-ticket "Fix this one" so fixing is opt-in until trust is built.

## 🟠 Trust & control — what I need before I point it at real code

### F4. Try it on a sample first
- **Expect:** a "Run on a sample repo" button so I see the whole flow — board,
  streaming, a real fix — before spending a cent on my own code.
- **Today:** first real experience = my repo + real cost + real risk. (`make seed`
  fakes a board; it isn't a real run.)
- **Flow:** bundle a tiny demo repo; a one-click "Try the demo" that imports it
  and runs (cheap, on Haiku) end-to-end.

### F5. Scope + focus the audit
- **Expect:** "audit only `src/auth`" or "only the files in this PR"; and "focus
  on security" (or perf / a11y / correctness).
- **Today:** it auto-splits into ≤10 modules and QAs everything with one generic
  prompt. No module selection, no focus profile.
- **Flow:** module checklist at import (from `map`'s detection); an audit-focus
  picker that steers the QA prompt; optional "changed files vs a base branch."

### F6. Undo a fix cleanly
- **Expect:** a bad fix → one-click **revert** of exactly that change.
- **Today:** Reopen re-queues the ticket but does **not** revert the prior fix
  commit (explicitly deferred). No clean per-fix undo.
- **Flow:** "Revert fix" = `git revert` the ticket's commit in the workspace +
  reopen; pairs with F1's apply so the source repo stays consistent.

## 🟡 The takeaway & ongoing use

### F7. A headline scorecard
- **Expect:** one glance: "Health B‑ · 12 issues (2 high) · top risks: …" — the
  thing I'd screenshot for my lead.
- **Today:** only the granular board + notes. No summary/verdict view.
- **Flow:** a run **Summary** tab: counts by severity, fixed vs open, a simple
  grade, and the top findings — generated from data already on the board.

### F8. Per-ticket dialogue ("why is this a bug? / it's intentional")
- **Expect:** on a finding, ask "why?" or say "won't fix, here's the reason" and
  have it understand *that ticket*.
- **Today:** chat is global over the whole repo; dismiss is a status with no
  reasoning captured. No ticket-scoped conversation.
- **Flow:** "Ask about this finding" in the drawer (chat seeded with the ticket +
  its file), and a reason field on dismiss.

### F9. Re-audit / trend over time
- **Expect:** re-run after I've made changes and see "3 new, 5 resolved since last
  run" — progress, not a cold restart.
- **Today:** every run starts fresh; no cross-run comparison or per-project
  history beyond a flat list on Home.
- **Flow:** project grouping + a run-to-run diff (new / resolved / persistent
  findings).

### F10. Keep the tests QA wrote
- **Expect:** "we added 8 tests" → keep them, add to my suite.
- **Today:** the generated tests are real and valuable but stranded in `runs/`.
- **Flow:** surface "N tests added" and include them in the patch/apply flow
  (F1) so coverage lands with the fixes.

## 🟢 Automation / integration (turns a tool into a habit)

### F11. Headless CLI + CI gate
- **Expect:** run it in CI, fail the build if P0s appear, get findings.json as an
  artifact.
- **Today:** it's a local UI you babysit; the server is the only entry point.
- **Flow:** `myaudit audit <path> --json --fail-on=high` that runs the graph
  headless and exits non-zero on threshold — the same engine, no UI.

### F12. Tell me when it's done
- **Expect:** it's long; ping me (desktop notification / webhook / Slack) when
  the audit finishes or needs me.
- **Today:** you watch the board. Run completion now flips status + emits an
  event, but nothing reaches the user who walked away.
- **Flow:** a desktop/OS notification on run.done/run.failed; optional webhook.

---

## Kanban board UX gaps (the board people actually expect)

Measured against standard kanban conventions (Trello/Jira/Linear;
[UX Patterns for Developers](https://uxpatterns.dev/patterns/data-display/kanban-board),
[Eleken drag-and-drop UX](https://www.eleken.co/blog-posts/drag-and-drop-ui)).
Our board today: 4 columns, filter bar, click a card → drawer. That's it — most
expected interactions are absent.

### K1 — Detail view (the drawer) is thin
- **Open the file from a finding.** A bug names `app/x.tsx:42` but you can't click
  it to see the code — the single most-expected click on an *audit* board. Build:
  "Open in editor" → Files tab at that file (and scroll to the line).
- **Prev / next card** without closing. Reviewing 12 findings means 12
  open-read-close cycles. Build: ↑/↓ + on-screen arrows to step through cards.
- **Copy link to this card.** No way to share/bookmark a specific finding. Build:
  card id in the URL + a copy button.
- **See the fix's diff in the drawer.** The diff exists on the Files tab but a
  bug's drawer doesn't show what its fix changed. Build: inline diff for fixed
  tickets.

### K2 — Card interactions are missing
- **Drag-and-drop between columns** — the defining kanban gesture, entirely
  absent; status only changes via buttons. Build: drag a ticket To do⇄Review⇄Done
  as a manual triage move (engine nodes stay non-draggable).
- **Quick actions on the card** (open / dismiss / fix) — today you must open the
  drawer for everything. Research: key actions shouldn't be buried. Build: a small
  action row on hover/focus.
- **Keyboard operable** — cards aren't focusable; no arrow-to-move, Enter-to-open.
  Build: focusable cards, roving focus, Enter opens.

### K3 — Board controls are shallow
- **Sort within a column** (severity / priority / newest) — none; order is fixed.
- **Group by** module or severity (swimlanes), not just status.
- **Collapse a column**; show counts by severity in the header.

## Suggested first slice
**F1 (get fixes out) → F3 (audit-only mode) → F2 (budget cap) → F4 (sample run).**
F1 makes the output real; F3+F2 make the first run safe to try; F4 earns trust
before real spend. Everything else builds on those.
