# Agent Screen Preview Fix Implementation Plan (Node + generic web apps)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Agent screen's live preview reliably reach `live` for Node and generic static-HTML web apps, fixing (1) static sites having no launcher at all and (2) the one-shot race that leaves it stuck when `npm install` finishes after the first preview attempt.

**Architecture:** `internal/preview/preview.go`'s `Command()` gains a static-site fallback launcher (in-process `http.FileServer`) alongside the existing unchanged Node launcher. `web/src/store.tsx` replaces its one-shot `startPreview` call with a retry that fires again every time a node completes, bounded naturally because completions stop once the run finishes.

**Tech Stack:** Go (backend, stdlib `net/http`/`os/exec`), React/TypeScript + Vitest (frontend).

## Global Constraints

- Scope is Node + generic static web apps only — no other-language launchers (Python/Go/etc. explicitly out of scope).
- No new external dependencies (Go stdlib only; no new npm packages).
- Every new `Command` code path must degrade to "no match" on any read/parse error — never panic or return an error from these functions.
- Existing Node behavior in `Command`/`Manager.Start` must be unchanged (same launcher chosen, same env vars, same reachability polling).
- Keep code comments minimal — only where a constraint is genuinely non-obvious. No narration comments.
- Reference design: `docs/superpowers/specs/2026-09-12-multi-language-agent-screen-design.md`.

---

### Task 1: `Command()` — static-site launcher + in-process file serving

**Files:**
- Modify: `internal/preview/preview.go`
- Test: `internal/preview/preview_test.go` (new file)

**Interfaces:**
- Consumes: `Detect(dir string) Kind` (unchanged).
- Produces: `Command(dir string, port int) (Launcher, bool)` — this REPLACES the old `Command(dir string) (name string, args []string, ok bool)`. `Launcher` is a new exported struct: `type Launcher struct { Name string; Args []string; Static bool }`. `Manager.Start` (same signature: `(runID string) (*Server, error)`) is the only other caller changed internally.

- [ ] **Step 1: Write the failing tests**

Create `internal/preview/preview_test.go`:

```go
package preview

import "testing"

func TestCommandNodeUnchanged(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"scripts":{"dev":"vite"},"dependencies":{"react":"^19"}}`)
	write(t, dir, "node_modules/.keep", "")
	l, ok := Command(dir, 3000)
	if !ok || l.Name != "npm" || len(l.Args) != 2 || l.Args[1] != "dev" || l.Static {
		t.Fatalf("Command() = %+v ok=%v, want npm run dev", l, ok)
	}
}

func TestCommandStaticFallback(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", "<!doctype html><div id=root></div>")
	l, ok := Command(dir, 4123)
	if !ok || !l.Static {
		t.Fatalf("Command() = %+v ok=%v, want static launcher", l, ok)
	}
}

func TestCommandNoneWhenDetectNone(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "README.md", "just docs\n")
	if _, ok := Command(dir, 4123); ok {
		t.Fatal("Command should refuse when Detect is none")
	}
}
```

Update the two existing tests in `internal/preview/detect_test.go` so the package compiles (old three-value signature no longer exists):

```go
func TestCommandSkipsNonUI(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"scripts":{"dev":"vite"}}`)
	write(t, dir, "node_modules/.keep", "")
	if _, ok := Command(dir, 3000); ok {
		t.Fatal("Command should refuse when Detect is none")
	}
}

func TestCommandFindsWebDev(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"scripts":{"dev":"vite"},"dependencies":{"react":"^19"}}`)
	write(t, dir, "node_modules/.keep", "")
	l, ok := Command(dir, 3000)
	if !ok || l.Name != "npm" || len(l.Args) != 2 || l.Args[1] != "dev" {
		t.Fatalf("Command() = %+v ok=%v, want npm run dev", l, ok)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go vet ./internal/preview/...`
Expected: fails to compile — `preview_test.go` calls `Command(dir, port)` (two args) and expects a `Launcher` struct, neither of which exist yet. This compile failure is the expected RED signal here, since `Command`'s public shape is what's changing.

- [ ] **Step 3: Implement**

Replace the existing `Command` function in `internal/preview/preview.go` (currently lines 39-64) with:

```go
type Launcher struct {
	Name   string
	Args   []string
	Static bool
}

func Command(dir string, port int) (Launcher, bool) {
	if Detect(dir) == KindNone {
		return Launcher{}, false
	}
	if l, ok := nodeLauncher(dir); ok {
		return l, true
	}
	if fileExists(filepath.Join(dir, "index.html")) {
		return Launcher{Static: true}, true
	}
	return Launcher{}, false
}

func nodeLauncher(dir string) (Launcher, bool) {
	pkg := filepath.Join(dir, "package.json")
	b, err := os.ReadFile(pkg)
	if err != nil {
		return Launcher{}, false
	}
	var m struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(b, &m) != nil {
		return Launcher{}, false
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); err != nil {
		return Launcher{}, false
	}
	for _, s := range []string{"dev", "start", "serve", "tauri"} {
		if _, has := m.Scripts[s]; has {
			return Launcher{Name: "npm", Args: []string{"run", s}}, true
		}
	}
	return Launcher{}, false
}
```

Update `Manager.Start` (currently lines 93-151) to use the new `Command` signature and branch on `Static`:

```go
func (m *Manager) Start(runID string) (*Server, error) {
	m.mu.Lock()
	if s, ok := m.live[runID]; ok {
		m.mu.Unlock()
		return s, nil
	}
	m.mu.Unlock()

	dir := filepath.Join(m.root, runID)
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	l, ok := Command(dir, port)
	if !ok {
		return nil, fmt.Errorf("no dev server detected for this project")
	}
	if l.Static {
		return m.startStatic(runID, dir, port)
	}

	logPath := filepath.Join(dir, ".myaudit", "preview.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	lf, _ := os.Create(logPath)

	c := exec.Command(l.Name, l.Args...)
	c.Dir = dir
	c.Env = append(os.Environ(),
		"PORT="+fmt.Sprint(port),
		"BROWSER=none",
		"NEXT_TELEMETRY_DISABLED=1",
	)
	if lf != nil {
		c.Stdout, c.Stderr = lf, lf
	}
	proc.SetGroup(c)

	if err := c.Start(); err != nil {
		if lf != nil {
			lf.Close()
		}
		return nil, err
	}

	deadline := time.Now().Add(bootTimeout)
	for time.Now().Before(deadline) {
		if reachable(port) {
			s := &Server{RunID: runID, URL: fmt.Sprintf("http://localhost:%d", port), Cmd: c, Log: logPath}
			m.mu.Lock()
			m.live[runID] = s
			m.mu.Unlock()
			writeLive(dir, s.URL)
			return s, nil
		}
		if c.ProcessState != nil && c.ProcessState.Exited() {
			return nil, fmt.Errorf("dev server exited during startup (see .myaudit/preview.log)")
		}
		time.Sleep(250 * time.Millisecond)
	}
	_ = proc.KillTree(c)
	return nil, fmt.Errorf("dev server did not answer on port %d within %s", port, bootTimeout)
}

func (m *Manager) startStatic(runID, dir string, port int) (*Server, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Addr: addr, Handler: http.FileServer(http.Dir(dir))}
	go func() { _ = srv.Serve(ln) }()

	s := &Server{RunID: runID, URL: fmt.Sprintf("http://localhost:%d", port), HTTPServer: srv}
	m.mu.Lock()
	m.live[runID] = s
	m.mu.Unlock()
	writeLive(dir, s.URL)
	return s, nil
}
```

Add `"net/http"` to the import block in `preview.go`. Extend `Server` (currently lines 22-27):

```go
type Server struct {
	RunID      string
	URL        string
	Cmd        *exec.Cmd
	HTTPServer *http.Server
	Log        string
}
```

Update `Manager.Stop` (currently lines 153-164) to close the HTTP server when there's no subprocess:

```go
func (m *Manager) Stop(runID string) {
	m.mu.Lock()
	s, ok := m.live[runID]
	delete(m.live, runID)
	m.mu.Unlock()
	if !ok {
		return
	}
	if s.HTTPServer != nil {
		_ = s.HTTPServer.Close()
	} else {
		_ = proc.KillTree(s.Cmd)
		go func() { _ = s.Cmd.Wait() }()
	}
	clearLive(filepath.Join(m.root, runID))
}
```

Finally, update the one external call site in `internal/api/live.go` (`previewStart`, around line 104):

```go
	if _, ok := preview.Command(filepath.Join("runs", id.String()), 0); !ok {
```

(replaces `if _, _, ok := preview.Command(filepath.Join("runs", id.String())); !ok {` — port `0` is fine here since only `ok` is used, not the launcher's args).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./... && go test ./internal/preview/... ./internal/api/... -v`
Expected: PASS — full build succeeds, all preview tests green (new static/none tests + fixed old tests), `internal/api` package still compiles and its existing tests pass unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/preview/preview.go internal/preview/preview_test.go internal/preview/detect_test.go internal/api/live.go
git commit -m "feat: static-site fallback launcher for Agent screen preview"
```

---

### Task 2: Fix the one-shot preview-start race with retry-on-node-completion

**Files:**
- Modify: `web/src/store.tsx`
- Test: `web/src/store.test.ts` (new file)

**Interfaces:**
- Consumes: `RunDetail.nodes: Node[]` (from `web/src/api.ts`, `Node.status: string`), `api.startPreview(id: string)`.
- Produces: an exported pure function `shouldRetryPreview(prevDoneCount: number, doneCount: number): boolean` that `store.tsx` wires into its existing `detail`-polling effect. No change to the `Store` type's public shape.

- [ ] **Step 1: Write the failing test**

Create `web/src/store.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { shouldRetryPreview } from './store'

describe('shouldRetryPreview', () => {
  it('fires on the first check for a fresh run', () => {
    expect(shouldRetryPreview(-1, 0)).toBe(true)
  })

  it('fires again when another node completes', () => {
    expect(shouldRetryPreview(1, 2)).toBe(true)
  })

  it('does not fire when the done count is unchanged', () => {
    expect(shouldRetryPreview(2, 2)).toBe(false)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npm run test -- store.test.ts`
Expected: FAIL — `shouldRetryPreview` is not exported from `./store` yet.

- [ ] **Step 3: Implement**

In `web/src/store.tsx`, add this exported pure function near the top of the file (after the existing type exports, before the `Store` type):

```ts
export function shouldRetryPreview(prevDoneCount: number, doneCount: number): boolean {
  return doneCount > prevDoneCount
}
```

Replace the one-shot preview effect (currently lines 107-112):

```ts
  const bootedPreview = useRef<string | null>(null)
  useEffect(() => {
    if (!runId || bootedPreview.current === runId) return
    bootedPreview.current = runId
    api.startPreview(runId).catch(() => {})
  }, [runId])
```

with:

```ts
  const previewDoneCounts = useRef<Record<string, number>>({})
  useEffect(() => {
    if (!runId || !detail) return
    const doneCount = detail.nodes.filter((n) => n.status === 'done').length
    const prev = previewDoneCounts.current[runId] ?? -1
    if (shouldRetryPreview(prev, doneCount)) {
      previewDoneCounts.current[runId] = doneCount
      api.startPreview(runId).catch(() => {})
    }
  }, [runId, detail])
```

This effect now re-runs on every `detail` update (already polled every 2s by the existing effect at line 143), so a preview attempt that failed before `npm install` finished gets retried the moment the next node completes. It naturally stops retrying once the run's node count stops increasing (all nodes reached a terminal state).

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npm run test -- store.test.ts`
Expected: PASS.

Then run the full frontend test suite to check for regressions: `cd web && npm run test`
Expected: PASS (all existing tests, including `util.test.ts`, still green).

- [ ] **Step 5: Commit**

```bash
git add web/src/store.tsx web/src/store.test.ts
git commit -m "fix: retry Agent screen preview start on each node completion instead of once"
```

---

## Final verification

After both tasks:

- [ ] `go build ./...` — clean.
- [ ] `go test ./...` — full backend suite green.
- [ ] `cd web && npm run test` — full frontend suite green.
- [ ] `cd web && npm run build && rm -rf ../internal/api/web/dist && cp -r dist ../internal/api/web/dist` — rebuild the embedded UI so the fix is actually servable (per `make ui-build`).
- [ ] Manual smoke test: run a Node project through the app and confirm the Agent screen reaches `live` even if it initially raced ahead of `npm install`.
