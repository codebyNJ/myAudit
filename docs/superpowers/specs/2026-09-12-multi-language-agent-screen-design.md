# Agent Screen Preview Fix (Node + generic web apps) — Design

**Branch:** `fix/agent-screen`

## Context

The Agent screen's live preview only works for Node projects, and even then only sometimes: `preview.Command` (`internal/preview/preview.go:39`) hardcodes `npm run <script>` as the only launch strategy — a plain static HTML/CSS/JS site with no `package.json` has no launcher at all, even though `Detect()` (`internal/preview/detect.go:35`) already correctly classifies it as `KindWeb` via the `index.html` check.

Confirmed live against a real run (`kiwi`, a Python project): `Detect` correctly returned `KindNone` since there's no web UI, and the frontend's `idle+kind=none` state (`AgentScreen.tsx:52`) correctly showed "No UI in this project" rather than spinning forever — that part of the system already works as designed.

**Scope decision:** support is limited to Node and generic (framework-agnostic) web apps — no Python/Go/other-language launchers. The two real gaps in that scope:
1. **Static sites have no launcher** — `Command()` only knows `npm run <script>`; a plain static HTML site (`Detect() == KindWeb` via `index.html`, no `package.json`) currently has nothing that can serve it.
2. **One-shot race** — `store.tsx:107-112` calls `startPreview` exactly once per run. If `node_modules` isn't installed yet at that moment, it never retries, so a Node project that would work fine 30 seconds later stays stuck.

Goal: make the Agent screen reliably reach `live` for any Node or static web project, robust to slow `npm install` timing.

**Deferred** (not in this branch, not currently planned): non-JS language support (Python/Go/etc.), agent-inferred run commands, user-supplied command overrides, and a process-log-stream view for UI-less CLI programs.

## Design

### 1. `Command()` — add a static-site launcher

File: `internal/preview/preview.go`

`Command(dir)` currently returns `ok=false` whenever there's no `package.json`/`node_modules`, even when `Detect(dir) == KindWeb` because of a plain `index.html`. Add a fallback: when the Node check doesn't match but `index.html` exists at `dir` root, return a "static" launcher instead of failing. `Manager.Start` (`preview.go:93`) gets a small branch: for a static launcher, skip the `exec.Command`/reachability-poll path and instead start an in-process `http.Server` (`http.FileServer(http.Dir(dir))`) on the assigned port, tracked in the same `live` map so `Stop`/`StopAll` keep working unchanged.

Node's existing detection/launch logic (`package.json` + `node_modules` + `dev`/`start`/`serve`/`tauri` script → `npm run <script>`) is unchanged.

### 2. Fix the one-shot preview race

File: `web/src/store.tsx`

Replace the `bootedPreview` ref (fires `startPreview` once on first seeing a `runId`) with a retry keyed to node completion:

- Track the count of nodes with `status === 'done'` from `detail.nodes` (already polled every 2s by the existing `loadDetail` effect).
- Whenever that count increases for the current run (a node just finished — e.g. `import`, or a QA node that ran `npm install`), call `api.startPreview(runId)` again.
- This is naturally bounded: a run's node-done-count can only increase up to its total node count, so retries stop on their own once the run finishes — no separate "terminal state" tracking needed.

No backend changes needed for idempotency — `previewStart` (`internal/api/live.go:94`) already short-circuits to a cheap "already live" or "unsupported" response without spawning duplicate processes when called repeatedly.

## Testing

- `internal/preview/preview_test.go` (new file): a static-fallback test asserting `Command()` returns a static launcher when only `index.html` is present (no `package.json`), plus the existing Node-launcher test cases carried over unchanged.
- `web/src/store.test.ts` (new file): a pure function `shouldRetryPreview(prevDoneCount, doneCount)` unit-tested directly (matches the existing `util.test.ts` convention — plain function tests, no component rendering, since the frontend test setup has no jsdom/testing-library).

## Error handling

- The static-launcher check must degrade to "no match" on any read/parse error, matching existing `Detect`/`Command` behavior.
- The static file-serving path reuses the same port-allocation guard (`freePort()`) as the subprocess launcher so `Manager.Start`'s contract (`(*Server, error)`) is unchanged for callers.
