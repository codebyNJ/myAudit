# Platform UI (React) Implementation Plan — Phase 10

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans (inline) — task-by-task, commit + Playwright-verify each. Steps use `- [ ]`.

**Goal:** Replace the hand-rolled embedded UI with a real **React + Vite + Tailwind** desktop frontend built from **faithfully-copied beautifului.dev components**, served by the Go engine and loaded by the Tauri shell. Full surface: Development (with integrated chat, sample.html-style), Activity, CI/CD, Schema, Swagger, Playwright testing, Kanban, Notes, and Account & Settings.

**Architecture:** `web/` is a Vite React app. `npm run build` emits `web/dist`, which Go embeds (`//go:embed dist`) and serves at `/`; `/api/*` unchanged. In dev, Vite proxies `/api` to :7788. beautifului has **no package** (copy-paste showcase) — so each component is **scraped from the live site via Playwright** (DOM + computed CSS) and reproduced as a React+Tailwind component under `web/src/components/bui/`, matching its real tokens.

**Tech Stack:** React 18 · Vite · TailwindCSS · TypeScript · lucide-react (icons) · Go embed for serving · Playwright for scraping + verification.

**Spec:** `docs/sample.html` (chat/editor layout), beautifului.dev (components), existing `/api/*` (runs, detail, checkpoints, openapi, schema).

## Global Constraints
- **Use beautifului's real components** — scrape each from the site; do not invent. Keep its dark oklch tokens.
- **Chat is inside Development** (composer + message thread over the editor), per `sample.html` — not floating, not a separate tab.
- **Tabs:** Development · Activity · CI/CD · Schema · Swagger · Playwright · Kanban · Notes. Plus an **account menu** (top-right) → Account & Settings.
- Every screen has **loading / empty / error** states and **toasts** for actions.
- DB-first; new surfaces (Notes, Kanban, Settings) get real endpoints + tables.
- Verify each task by driving the built app with Playwright (screenshot).

## File Structure
```
web/                      # Vite React app (replaces internal/api/web/index.html)
  index.html  vite.config.ts  tailwind.config.js  tsconfig.json  package.json
  src/
    main.tsx  App.tsx  api.ts  state.ts  tokens.css
    components/bui/       # faithful beautifului components (scraped)
      SidebarNav, ToolChips, TaskRows, ApprovalCard, DiffTable, Chat, PromptBar,
      Thinking, StreamingText, CodeBlock, ContextCard, Toast, EmptyState, Spinner
    components/shell/     # Header, WorkspaceSwitcher, Tabs, AccountMenu, Layout
    screens/             # Development, Activity, CICD, Schema, Swagger, Playwright, Kanban, Notes, Settings
internal/api/embed.go     # //go:embed dist -> http.FS (replaces static.go)
```

## Phases

### Phase A — React scaffold + Go serves the build
- [ ] A1: `npm create vite@latest web -- --template react-ts`; add Tailwind (`tailwindcss postcss autoprefixer`), `tailwind.config.js` content globs, `lucide-react`. `web/src/tokens.css` = beautifului dark oklch tokens (scraped earlier) as CSS vars + Tailwind theme extend.
- [ ] A2: `vite.config.ts` — `server.proxy['/api'] -> http://localhost:7788`; `build.outDir=dist`.
- [ ] A3: `internal/api/embed.go` — embed `../../web/dist` (or copy build to `internal/api/web/dist` in Makefile), serve at `/` with SPA fallback to `index.html`. Update `cmd/serve` wiring. `make ui-build` builds the React app before `bin/serve`.
- [ ] A4: hello-world React page served by Go; Playwright verify `/` renders it.

### Phase B — Shell (tokens, header, tabs, layout, account menu)
- [ ] B1: Layout grid (header + sidebar + main), tokens applied, dark theme.
- [ ] B2: Header — WorkspaceSwitcher (runs dropdown, live), branch tag, **Tabs** (the 8), Search (⌘K visual), **AccountMenu** (avatar → Account & Settings).
- [ ] B3: `state.ts` — app store (runs, runId, detail, tab, toasts, selection); `api.ts` typed fetch with error→toast.
- [ ] B4: Toast system + EmptyState + Spinner (bui components).

### Phase C — Faithful beautifului components (scrape → React)
One task each: **Playwright-scrape the component's DOM + computed CSS from beautifului.dev**, reproduce as a React+Tailwind component, snapshot-verify visually.
- [ ] C1 SidebarNav · C2 ToolChips · C3 TaskRows · C4 ApprovalCard · C5 DiffTable
- [ ] C6 Chat (message thread) · C7 PromptBar · C8 Thinking · C9 StreamingText
- [ ] C10 CodeBlock · C11 ContextCard

### Phase D — Screens wired to the engine
- [ ] D1 **Development** — CodeBlock/DiffTable (file tree + selected file) **+ integrated Chat + PromptBar** (sample.html layout); PromptBar posts steer.
- [ ] D2 **Activity** (own tab) — event log via TaskRows/Thinking/StreamingText, live.
- [ ] D3 **CI/CD** — pipeline from nodes (TaskRows), live.
- [ ] D4 **Schema** — node visualizer from `/api/schema`.
- [ ] D5 **Swagger** — endpoint list + detail from `/api/openapi`.
- [ ] D6 **Kanban** — board of tasks/issues (needs `/api/board` or reuse nodes).
- [ ] D7 **Notes** — markdown notes (needs `/api/notes` CRUD + table).
- [ ] D8 **Playwright** — testing view: trigger + show run results/screenshots (needs `/api/tests` or artifact list).

### Phase E — New backend endpoints + tables (TDD, Go)
- [ ] E1 `notes` table + `GET/PUT /api/runs/{id}/notes`.
- [ ] E2 `POST /api/runs/{id}/steer` — append a steer event (chat).
- [ ] E3 Kanban: `GET /api/runs/{id}/board` (nodes grouped) or an `issues` table.
- [ ] E4 Playwright results: `GET /api/runs/{id}/tests` (from events/artifacts).

### Phase F — Account & Settings
- [ ] F1 `settings` table + `GET/PUT /api/settings` (model tier, budget, provider keys presence, theme).
- [ ] F2 Account & Settings screen (from AccountMenu) — profile, model/budget (router policy), provider key status (existence only, never values), theme toggle.

## Self-Review
- Chat is in Development (D1), not a tab — matches the clarified requirement.
- Tabs are exactly the 8 requested; Account & Settings via AccountMenu (F).
- Every screen: loading/empty/error + toasts (Phase B primitives reused).
- beautifului components are scraped, not invented (Phase C).
- New surfaces get real endpoints/tables (E, F) — DB-first.

## Notes / open decisions
- Embedding: build `web/dist` into the Go binary vs serve from disk in dev — A3 uses embed for the shipped binary, Vite proxy for dev.
- Playwright tab scope: start with listing the latest test/e2e run results; full in-app browser embedding is a later stretch.
- Steer/real-runner: chat PromptBar posts steer events now; wiring steer to the live claude runner is Phase 6/real-runner follow-up.
