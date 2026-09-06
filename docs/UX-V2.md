# myAudit V2 — layout, screens, flows, productization

Driven by direct feedback + 3 annotated screenshots. Split into: (A) a layout
system to kill the single-column look, (B) module-by-module screen redesigns,
(C) the missing-flows list, (D) deeper productization. Design principle running
through all of it: **the AI dumps whatever it wants; the frontend absorbs it
gracefully.** We harden rendering in code, not by constraining the agent.

---

## A. Layout system — stop centering everything in one narrow column
Today `.pane { max-width: 860px; margin: 0 auto }` makes Summary, Report, Notes
and Settings a thin ribbon down the middle of a wide dark screen (see shots 2 &
3). Vercel's dashboards never do this — they use the **full width** with a
responsive card grid and, where useful, a content + rail split.

- **A1. Replace `.pane` with a fluid dashboard container** — full width (with a
  generous `max-width: 1400px` only on ultra-wide), real horizontal padding,
  content laid out in a responsive `grid` (`repeat(auto-fit, minmax(320px,1fr))`)
  instead of one stacked column. One CSS change lifts every screen that uses it.
- **A2. Content + rail pattern** where a screen has a primary area and metadata
  (Summary, drawer, Code) — `grid-template-columns: 1fr 320px`, collapsing to one
  column under ~900px. This is the Vercel "main + side stats" shape.
- **A3. Card primitive** (`.card`) — one consistent surface (border, radius,
  header row, hover) used by Summary tiles, Report sections, Settings groups, so
  the whole app reads as one system, multi-column by default.

## B. Screens

### B1. Board → JIRA-grade
Current board is functional (shot 1) but flat. Bring it to what people expect:
- Column **WIP count + severity chips already there** — keep; add colored column
  accents and a subtle card **priority stripe** (p0/p1 down the left edge).
- **Card face**: type icon + key (#id) + title, a compact meta row (module,
  cost, tests, time), assignee/agent avatar, severity as a solid pill. Denser,
  scannable, JIRA-like.
- **Group-by swimlanes** already exist — make them first-class (collapse, counts).
- **Empty columns** get a lighter dashed state, not a big "Empty" block.
- Keep drag-and-drop; add card **hover quick-actions** (open, copy link, jump to
  code) without opening the drawer.

### B2. Notes/Report → lightweight + AI-robust
Notes is a heavy split editor (shot 3) but the content is 99% agent-generated —
the user rarely hand-edits. Reframe:
- **Report tab = read-first.** Default to the rendered view full-width with good
  typography (the markdown the agent writes: headings, tables, code). Editing is
  a secondary toggle, not the default split.
- **Robust rendering** (the "AI dumps whatever" ask): GFM tables scroll instead
  of overflow; code blocks get the Code highlighter; long tables/wide content
  never break the page layout; unknown/odd markdown degrades cleanly.
- **Lightweight**: drop the always-open textarea + toolbar; render is the point.
  Keep export (report.md / findings.json) and a small "Edit" affordance.

### B3. Code → VS Code/Cursor basics + list-before-code
Shot request: "before code list properly" + missing editor features. Today Code
jumps straight into a file with a bare `<textarea>` for edit.
- **Landing = a proper changed-files list** (PR "Files changed" style): path,
  +/- stat, severity, review state — click a row to open. Not a blank editor.
- **Editor upgrades** (the Cursor/VS Code basics): line numbers, current-line
  highlight, in-file find (⌘F), soft-wrap toggle, language in a breadcrumb, and
  a real editable surface (not a plain textarea). *Dependency fork — see
  Decisions: CodeMirror 6 recommended over Monaco for weight.*
- Keep diff-first, the changed-file stepper, accept/reject.

### B4. Summary + Activity → one "Overview" tab
User: combine them. Activity is the event feed; Summary is the rollup. Merge:
- **Overview** = a Vercel-style dashboard: a top row of stat tiles (modules,
  findings by severity, fixes verified, cost, duration), the QA-by-module and
  fix-verification cards, UI previews — **plus** a live Activity feed in a side
  rail (A2 content+rail). One screen answers "what happened + what's happening."
- Frees a tab slot → tabs become **Board · Overview · Report · Code**.

## C. Missing flows (the list, ranked) — mostly still unbuilt (F1–F12 from
PRODUCT-FLOWS.md are all pending). The make-or-break ones:
1. **F1 Get fixes into MY repo** — export patch / branch / PR. Without this the
   whole audit is trapped in a sandbox. *Highest impact.*
2. **F3 Audit-only mode** — "find, don't touch" — the safe first run everyone
   wants before trusting autonomous fixes.
3. **F2 Cost estimate + cap** — "what will this cost / stop at $X."
4. **F5 Scope/focus** — audit only these paths/modules.
5. **F6 Undo a fix cleanly** — per-fix revert.
6. **F7 Headline scorecard** — the shareable takeaway (feeds B4 Overview).
7. **F4 Sample run**, **F8 per-ticket dialogue (why is this a bug?)**,
   **F9 re-audit/trend**, **F10 keep the tests**, **F11 CLI/CI gate**,
   **F12 notify on done**.
New (from this pass):
8. **Onboarding: pick what to review** right after import (ties F3+F5) instead of
   auto-running everything.
9. **Diff confidence** — show which fixes have passing new tests vs. not, on the
   card face (partly in Overview).

## D. Deeper productization (beyond PRODUCTIZATION.md)
- **First-run empty states** everywhere read as intentional, never "broken/fake."
- **Perceived latency**: the minutes-long QA nodes need per-module live progress
  in Overview, not just a board banner.
- **Trust signals**: every fix shows evidence (tests added, regression passed,
  preview) at a glance — not buried in a drawer.
- **Real-repo robustness**: monorepos, huge trees, non-JS stacks — the file
  surfaces must not balloon or stall (documented ceiling on the workspace walk).
- **Shareability**: Overview + Report should be exportable/linkable as the
  artifact you'd send a teammate or a client.
- **Reversibility**: nothing the agent does is scary if undo (F6) + audit-only
  (F3) exist. These two unlock adoption more than any polish.

---

## Decisions (locked)
1. **Code editor = Monaco** (full VS Code engine) — accept the bundle weight for
   the real editor experience.
2. **Sequence = Layout system (A) first** → then screens (Report, Overview merge,
   Board, Code) → flows (C) after/interleaved.
3. **Board = full JIRA clone** — match JIRA layout/interactions closely.

## Build order (V2)
V2-A layout system → V2-B2 Report → V2-B4 Overview (Summary+Activity) →
V2-B1 Board (full JIRA) → V2-B3 Code (list-first + Monaco) → V2-C flows.
