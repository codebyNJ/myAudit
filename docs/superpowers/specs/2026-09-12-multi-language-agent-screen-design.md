# Multi-language Agent Screen Preview — Design

**Branch:** `fix/agent-screen`

## Context

The Agent screen's live preview only works for Node projects, and even then only sometimes: `preview.Command` (`internal/preview/preview.go:39`) hardcodes `npm run <script>` as the only launch strategy, and `preview.Detect` (`internal/preview/detect.go:35`) only recognizes JS-ecosystem signals (React/Vue/Tauri/index.html/Vite-Next-Nuxt configs) when deciding whether a project even has a UI worth previewing.

Confirmed live against a real run (`kiwi`, a Python project): `Detect` correctly returned `KindNone` since there's no JS UI, and the frontend's `idle+kind=none` state (`AgentScreen.tsx:52`) correctly showed "No UI in this project" rather than spinning forever — that part of the system already works as designed.

The real gaps:
1. **Language coverage** — Python/Go web projects have real browsable UIs that `Detect` can't see, so they're wrongly classified as "no UI."
2. **Launch strategy** — even once detected, there's no launcher for anything but `npm`.
3. **One-shot race** — `store.tsx:107-112` calls `startPreview` exactly once per run. If dependencies (`node_modules`, a venv, etc.) aren't installed yet at that moment, it never retries, so a project that would work perfectly fine 30 seconds later is stuck.

Goal: make the Agent screen work for Node, Python, Go, and static-HTML projects, and make the retry timing robust to slow dependency installs.

**Deferred to a follow-up** (not in this branch): agent-inferred run commands and user-supplied command overrides for projects the tier-1 heuristics still can't launch, and a process-log-stream view for UI-less CLI programs. Those need a new `runs.preview_cmd` column and touch the agent-prompt layer.

## Design

### 1. `Detect()` — recognize non-JS web signals

File: `internal/preview/detect.go`

Add, alongside the existing JS checks in `Detect(dir)`:
- **Python**: `KindWeb` if `manage.py` exists (Django), or a `requirements.txt`/`pyproject.toml` mentions `flask`, `fastapi`, or `django`, or a `templates/` directory exists (Flask/Django convention).
- **Go**: `KindWeb` if a `main.go` exists (root or one level down, e.g. `cmd/*/main.go`) and any `.go` file imports `net/http`, `github.com/gin-gonic/gin`, `github.com/labstack/echo`, or `github.com/gofiber/fiber`.

These are additional `return KindWeb` branches — no change to existing JS/desktop logic or `KindNone` fallback.

### 2. `Command()` — ordered multi-language launchers

File: `internal/preview/preview.go`

Replace the single npm-only `Command(dir)` with an ordered list of launcher checks, returning on the first match:

1. **Node** (existing logic, unchanged): `package.json` + `node_modules` present + one of `dev`/`start`/`serve`/`tauri` scripts → `npm run <script>`.
2. **Python — Django**: `manage.py` present → `python manage.py runserver 0.0.0.0:{port}`.
3. **Python — Flask**: an `app.py` or `wsgi.py` importing `flask` → `flask run --host 0.0.0.0 --port {port}` (with `FLASK_APP` set to the detected file in `c.Env`).
4. **Python — FastAPI**: a file importing `fastapi` with a module-level `app = FastAPI(...)` → `uvicorn {module}:app --host 0.0.0.0 --port {port}` (module name derived from the file's path relative to `dir`).
5. **Go**: `main.go` at `dir` root → `go run .`; else a single `cmd/*/main.go` → `go run ./cmd/<name>` (matches the `Detect` check in section 1).
6. **Static**: no launcher matched, but `Detect(dir) == KindWeb` via the plain `index.html` check → no subprocess at all; `Manager.Start` serves `dir` directly with `http.FileServer` on the assigned port.

`Manager.Start` (`preview.go:93`) needs a small branch: if the matched launcher is "static," skip the `exec.Command`/reachability-poll path and instead start an in-process `http.Server` on the assigned port, tracked in the same `live` map so `Stop`/`StopAll` keep working unchanged.

Port injection stays as-is (`PORT` env var, `freePort()`) for Node/static; Python/Go launchers get the port via their own CLI flag as shown above, plus the same `PORT` env var for frameworks that read it (Flask/FastAPI both do when told to via the flag above, so no extra env-based branching needed).

### 3. Fix the one-shot preview race

File: `web/src/store.tsx`

Replace the `bootedPreview` ref (fires `startPreview` once on first seeing a `runId`) with a retry keyed to node completion:

- Track the count of nodes with `status === 'done'` from `detail.nodes` (already polled by `loadDetail`).
- Whenever that count increases (a node just finished — e.g. `import`, or a QA node that ran `npm install`/`pip install`), call `api.startPreview(runId)` again.
- Stop retrying once the live-view poll (`AgentScreen.tsx`'s existing `api.live` interval) reports a terminal state: `live`, `frames`, or `idle` with `kind === 'none'`. A `stoppedPreviewRetry` ref (keyed by `runId`) records this so completed/no-UI runs don't keep re-POSTing.

No backend changes needed for idempotency — `previewStart` (`internal/api/live.go:94`) already short-circuits to a cheap "already live" or "unsupported" response without spawning duplicate processes when called repeatedly.

## Testing

- `internal/preview/detect_test.go`: table-driven cases for each new Python/Go signal (Django `manage.py`, Flask `app.py`, FastAPI file, Go `main.go` + `net/http`), plus a negative case (Go file with no web import → `KindNone`).
- `internal/preview/preview_test.go` (or new file): one test per new launcher in `Command()`, using temp-dir fixtures, asserting the exact `(name, args)` returned; a static-fallback test asserting `Manager.Start` serves the directory without spawning a subprocess (assert `Server.Cmd == nil` for the static case, or equivalent).
- `web/src/store.test.tsx` (check existing test setup under `web/` — `vitest.config.ts` exists): test that `startPreview` is called again when the done-node count increases, and NOT called again once `api.live` returns a terminal state.

## Error handling

- All new `Detect`/`Command` checks must degrade to "no match" on any read/parse error — never throw, matching existing `Detect` behavior (`fileExists`/`readPackageJSON` already swallow errors).
- Static file-serving path reuses the same `bootTimeout`/port-allocation guards as subprocess launchers so `Manager.Start`'s contract (`(*Server, error)`) is unchanged for callers.
