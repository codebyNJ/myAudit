# myAudit — UX redesign plan (post-import flow, IA, explorer, smoothness)

Synthesis of a 3-lens Sonnet study (post-import/IA · explorer/worktree · per-module
audit). The lenses converged hard; high-confidence items are flagged ⭑ (found
independently by 2+). Ordered into buildable phases; nothing here needs a new
backend subsystem — it's mostly reordering defaults + plumbing fields that already
exist.

## The core reframe
myAudit is an **audit tool**, not a general IDE. The unit of work is a *finding →
its fix's diff*, not a path in a file tree. Today the UI leads with IDE metaphors
(a full repo tree on every tab, generic tab names, a raw board of pipeline
plumbing on landing). Every change below pushes findings/fixes to the front and
demotes the IDE chrome to a reference/escape-hatch.

---

## Phase U1 — Fix what's actively wrong (bugs + structural)
The cheapest, highest-trust wins. Two agents flagged #1 and #2 independently.

- **U1.1 ⭑ Screens unmount on tab switch → silent data loss.** `App.tsx` `Screen()`
  returns one component per `tab`, so switching tabs unmounts the last screen:
  unsaved Notes edits and in-progress Dev edits are silently discarded, and the
  Board's filter/sort/collapsed-column state resets every visit. The
  `.screen`/`.screen.on` show/hide CSS is already written and unused — mount the
  screens once and toggle visibility. *Root-cause fix; unblocks much below.*
- **U1.2 ⭑ QA preview shows the WRONG screenshots.** `previewsFor()` shows every
  `.myaudit/preview/*.png` in the run on every QA card. Each QA card is tagged
  `module:<name>` and its shot is saved `<name>.png` — filter by match. (Actively
  wrong today.)
- **U1.3 Poll-failure handling is inconsistent** — `loadDetail` toasts on every 2s
  failure (flood), while board pollers swallow errors (invisible staleness). Pick
  "toast once on transition into failure, silent on recovery."
- **U1.4 ⭑ Dead `TaskIcon` cases** (`understand/testgen/verify/review`) removed; add
  real icons for `map` and `qa` (the nodes that dominate the early board).

## Phase U2 — The post-import moment + IA (tabs, landing)
Right now: click "Start audit" → jump from a plain form into full IDE chrome →
land on a bare spinner → 2 cryptic "import"/"map" cards for *minutes*, in backend
jargon, with no phase/ETA framing. Fix the front door.

- **U2.1 ⭑ Rename tabs to audit vocabulary** (label-only; ids unchanged so deep
  links/URLs are safe): **Board · Summary · Report · Code · Activity**
  (was Board · Verify · Notes · Files · Activity). "Report" already holds
  report.md/findings.json; "Summary" (today's `PlaywrightScreen`) is already a
  run rollup; "Code" avoids clashing with the board's "Review" column.
- **U2.2 Land post-import on Summary, not a raw Board.** Give Summary a calm phase
  strip ("Importing ✓ → Mapping ✓ → QA 2/6 → 3 fixes open, 1 verified") computed
  from the `cards` it already fetches. Reopening a past run still lands on Board.
- **U2.3 Humanize live status.** Header run-pill maps node type →
  {import:"Importing", map:"Mapping modules", qa:"Reviewing", bug:"Fixing"}
  instead of printing "qa running". Board first-paint gets a labeled state
  ("Setting up your audit…") not a bare spinner.
- **U2.4 ⭑ Fold Verify into the board / make it non-redundant.** `PlaywrightScreen`
  is a non-interactive re-slice of the same `/board` data. Repurposed as the
  Summary landing (U2.1/U2.2) it earns its place; its rows must click through to
  `?card=<id>`. (Also drop the `Playwright*` fossil name.)

## Phase U3 — The explorer → a review queue (scope to Code tab)
The workspace is a *running app sandbox* (~22k files on disk, mostly `node_modules`
the QA loop installs), not a repo you cloned. Stop presenting it as a folder tree.

- **U3.1 ⭑ Scope Explorer to the Code tab only.** It currently renders on every tab
  (board, Settings…). Mount it only for `dev`; use the already-defined-but-unused
  `.body.locked` single-column grid elsewhere.
- **U3.2 Changed/findings-first by default.** Default the existing "Changed (N)"
  view on when there are changes; the full tree becomes the labeled secondary
  ("All files") escape-hatch.
- **U3.3 Carry severity onto changed files.** `ChangedFilesForRun` already iterates
  node outputs — also collect `severity`/`title` into `FileEntry`; color the "M"
  marker by `SEV_COLOR` (hoist from KanbanScreen to util) instead of flat green;
  worst-severity + count when a file has multiple tickets.
- **U3.4 Next/prev changed-file stepping in DevScreen** (mirror the board's existing
  ↑/↓ card stepping) so reviewing N fixes is one sitting, not N sidebar trips.
- **U3.5 Show accept state in the changed list** (dim/✓ accepted rows) so the review
  queue visibly shrinks — `FileEntry.Review` already exists, just unrendered.

## Phase U4 — Smoothness & accessibility polish
- **U4.1 ⭑ Checkpoint visibility** — badge the collapsed Chat FAB (+ a toast) when a
  checkpoint is open; a blocked run currently looks merely idle.
- **U4.2 Palette discoverability** — a clickable ⌘P/find entry point in the chrome
  (today it's keyboard-only, hinted only in the Explorer search placeholder).
- **U4.3 Keyboard/a11y for the remaining `<div onClick>`** — WorkspaceSwitcher,
  AccountMenu, column-collapse toggle (role/tabIndex/Enter/Esc), matching Tabs.
- **U4.4 Wire `loadingDetail`** into Activity (and any screen that false-shows an
  empty state on deep-link before the first poll).
- **U4.5 Add a top-level error boundary** (`main.tsx`) — screens render
  agent-produced variable-shaped data; one bad render shouldn't white-screen.

## Phase U5 — Cleanup (deletions)
- **U5.1 Delete dead `PromptBar.tsx`** (zero imports).
- **U5.2 `steer` is dead end-to-end** (no UI calls it, no worker reads it) — either
  wire it into Chat as a real "steer this run" or delete route+client+test.
- **U5.3 Event pagination** — `EventsForRun` is a hard `LIMIT 200` with no
  `before`/`offset`; a multi-module run blows past 200 in the first QA node,
  making earlier history unreachable above the DB. Add a param.

---

## Status — ✅ shipped
- **U1** no-unmount tabs, scoped QA previews, poll errors, node icons — done.
- **U2** audit tab names (Board·Summary·Report·Code·Activity), Board progress
  banner, humanized run-pill, Summary→Board click-through — done.
- **U3** explorer scoped to Code, changed-first default, PR-style diff stepper — done.
- **U4/U5** checkpoint FAB badge, error boundary, deleted PromptBar + dead steer,
  event `?events=N` — done.
- **Deferred (low value):** U3.3 severity-color on changed rows (needs a FileEntry
  field), U3.5 accept-state dimming, U4.2 palette mouse button, U4.3 a11y for the
  remaining menu `<div>`s, U4.4 loadingDetail wiring.

## Suggested build order
U1 (correctness/structural) → U2 (front door: tabs + landing) → U3 (explorer as
review queue) → U4 (polish) → U5 (cleanup). U1.1 first — it's the root cause of
several other symptoms.
