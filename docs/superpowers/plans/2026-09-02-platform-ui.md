# Platform UI Implementation Plan (Phase 9)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Turn the static platform UI (`docs/sample.html`, embedded at `internal/api/web/index.html`) into a **live, interactive** app wired to the Go engine — create runs, watch them progress, resolve checkpoints, and steer — with the designed Zinc-dark IDE tabs (Dev / Config / CI-CD / Schema / Swagger) reading real data.

**Architecture:** The Go `serve` process gains (a) **write endpoints** to create runs and resolve checkpoints, (b) a **background run loop** that drives ready runs so the UI shows live progression, and (c) **richer read endpoints** (checkpoints + latest node output). The embedded UI's inline JS is replaced with real `fetch` wiring per tab. Runs are driven by a **stub runner** that emits realistic events (the real `claude -p` runner swaps in behind the same interface later).

**Tech Stack:** Go (net/http, httptest) · vanilla JS in the embedded page · Postgres · Playwright for UI verification.

**Spec:** `docs/sample.html` (UI source of truth), `docs/architecture.md`, and the existing engine (`internal/{store,queue,orchestrator,worker,api}`).

## Global Constraints

- **DB-first:** all state via the store; no `.md`/file writes for runtime data.
- **Reuse the engine:** create runs via `store.CreateGraph`, drive via `orchestrator`, resolve via `store.ResolveCheckpoint`. Do not duplicate that logic in handlers.
- **Write endpoints validate input** and return JSON errors; irreversible/steering actions map to existing gated paths.
- **Frontend stays in the embedded page** (`internal/api/web/index.html`) so `go:embed` ships it; no external assets (self-contained, CSP-safe).
- Go tasks are TDD (`httptest` + real Postgres). Frontend tasks are verified by **driving the page with Playwright** and asserting via the API/DOM (documented per task) — not unit tests.
- Run the DB suite with `-p 1` (shared Postgres).

## File Structure

- `internal/api/api.go` — add write routes + extend detail route (MODIFY)
- `internal/api/runloop.go` — background orchestrator loop + stub runner (CREATE)
- `internal/api/runloop_test.go` — loop advances a run (CREATE)
- `internal/api/write_test.go` — create/resolve endpoint tests (CREATE)
- `internal/store/read.go` — add `OpenCheckpointsForRun`, `LatestNodeOutput` (MODIFY)
- `cmd/serve/main.go` — start the run loop alongside the server (MODIFY)
- `internal/api/web/index.html` — replace inline JS with live wiring per tab (MODIFY)

---

## Phase 9A — Backend: write API + run loop

### Task 9.1: POST /api/runs — create a run from wizard config

**Files:**
- Modify: `internal/api/api.go`
- Test: `internal/api/write_test.go`

**Interfaces:**
- Produces: `POST /api/runs` accepting `{"project":"...","landing":bool,"google_auth":bool}` → `201 {"id":"<uuid>"}`. Internally calls `store.CreateGraph` with a fixed task set (scaffold → schema → auth → [landing]).

- [ ] **Step 1: Write the failing test**

```go
// internal/api/write_test.go
package api
import ("bytes";"context";"encoding/json";"net/http";"net/http/httptest";"testing")
func TestCreateRunEndpoint(t *testing.T){
  s:=newStore(t); defer s.Close()
  srv:=httptest.NewServer(NewMux(s,nil)); defer srv.Close()
  body:=bytes.NewBufferString(`{"project":"acme-saas","landing":true,"google_auth":true}`)
  resp,err:=http.Post(srv.URL+"/api/runs","application/json",body)
  if err!=nil{t.Fatal(err)}
  if resp.StatusCode!=201{t.Fatalf("want 201, got %d",resp.StatusCode)}
  var out struct{ ID string `json:"id"` }
  json.NewDecoder(resp.Body).Decode(&out)
  if out.ID==""{t.Fatal("expected run id")}
  // graph materialized with >=3 nodes
  runs,_:=s.ListRuns(context.Background(),10)
  if len(runs)!=1||runs[0].Project!="acme-saas"{t.Fatalf("run not created: %+v",runs)}
}
```

- [ ] **Step 2: Run test — expect FAIL** (`404`/no handler)

Run: `TEST_DATABASE_URL=... go test ./internal/api/ -run CreateRun`

- [ ] **Step 3: Implement the handler**

```go
// add inside NewMux in internal/api/api.go, before the static handler
mux.HandleFunc("POST /api/runs", func(w http.ResponseWriter, r *http.Request){
  var cfg struct{ Project string `json:"project"`; Landing bool `json:"landing"`; GoogleAuth bool `json:"google_auth"` }
  if err:=json.NewDecoder(r.Body).Decode(&cfg); err!=nil || cfg.Project==""{
    http.Error(w,"project required",400); return
  }
  specs:=[]store.TaskSpec{
    {Key:"scaffold",Type:"scaffold"},
    {Key:"schema",Type:"implement",DepKeys:[]string{"scaffold"}},
    {Key:"auth",Type:"implement",DepKeys:[]string{"schema"}},
  }
  if cfg.Landing {
    specs=append(specs,store.TaskSpec{Key:"landing",Type:"implement",DepKeys:[]string{"auth"}})
  }
  run,_,err:=s.CreateGraph(r.Context(),cfg.Project,specs)
  if err!=nil{ http.Error(w,err.Error(),500); return }
  w.Header().Set("content-type","application/json"); w.WriteHeader(201)
  json.NewEncoder(w).Encode(map[string]string{"id":run.String()})
})
```

- [ ] **Step 4: Run test — expect PASS**
- [ ] **Step 5: Commit** `feat: POST /api/runs (create run from wizard config)`

### Task 9.2: POST /api/checkpoints/{id}/resolve

**Files:** Modify `internal/api/api.go`; Test `internal/api/write_test.go`

**Interfaces:** `POST /api/checkpoints/{id}/resolve` body `{"answer":"..."}` → `200`; calls `store.ResolveCheckpoint`.

- [ ] **Step 1: Failing test**

```go
func TestResolveCheckpointEndpoint(t *testing.T){
  ctx:=context.Background(); s:=newStore(t); defer s.Close()
  run,_:=s.CreateRun(ctx,"acme"); nid,_:=s.AddNode(ctx,run,"implement",nil)
  s.Pool().Exec(ctx,`UPDATE nodes SET status='running' WHERE id=$1`,nid)
  cp,_:=s.RaiseCheckpoint(ctx,run,nid,"Google-only?",[]string{"yes","no"})
  srv:=httptest.NewServer(NewMux(s,nil)); defer srv.Close()
  resp,_:=http.Post(srv.URL+"/api/checkpoints/"+cp.String()+"/resolve","application/json",
    bytes.NewBufferString(`{"answer":"yes"}`))
  if resp.StatusCode!=200{t.Fatalf("want 200, got %d",resp.StatusCode)}
  n,_:=s.GetNode(ctx,nid); if n.Status!="ready"{t.Fatalf("node should be requeued, got %s",n.Status)}
}
```

- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement**

```go
mux.HandleFunc("POST /api/checkpoints/{id}/resolve", func(w http.ResponseWriter, r *http.Request){
  id,err:=uuid.Parse(r.PathValue("id")); if err!=nil{ http.Error(w,"bad id",400); return }
  var b struct{ Answer string `json:"answer"` }
  json.NewDecoder(r.Body).Decode(&b)
  if err:=s.ResolveCheckpoint(r.Context(),id,b.Answer); err!=nil{ http.Error(w,err.Error(),500); return }
  w.WriteHeader(200)
})
```

- [ ] **Step 4: Run — PASS**  · **Step 5: Commit** `feat: POST /api/checkpoints/{id}/resolve`

### Task 9.3: Extend read — open checkpoints + latest node output

**Files:** Modify `internal/store/read.go`, `internal/api/api.go`; Test `internal/api/api_test.go`

**Interfaces:**
- Produces: `(*Store).OpenCheckpointsForRun(ctx, run) ([]Checkpoint, error)`; `RunDetail` gains `Checkpoints []store.Checkpoint`.

- [ ] **Step 1: Failing test** — seed a run + raised checkpoint; `GET /api/runs/{id}` returns it under `checkpoints`.

```go
func TestRunDetailIncludesCheckpoints(t *testing.T){
  ctx:=context.Background(); s:=newStore(t); defer s.Close()
  run,_:=s.CreateRun(ctx,"acme"); nid,_:=s.AddNode(ctx,run,"implement",nil)
  s.Pool().Exec(ctx,`UPDATE nodes SET status='running' WHERE id=$1`,nid)
  s.RaiseCheckpoint(ctx,run,nid,"Google-only?",nil)
  srv:=httptest.NewServer(NewMux(s,nil)); defer srv.Close()
  var d RunDetail
  resp,_:=http.Get(srv.URL+"/api/runs/"+run.String()); json.NewDecoder(resp.Body).Decode(&d)
  if len(d.Checkpoints)!=1||d.Checkpoints[0].Question!="Google-only?"{t.Fatalf("checkpoints: %+v",d.Checkpoints)}
}
```

- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement** `OpenCheckpointsForRun` (SELECT open checkpoints WHERE run_id) and add to `RunDetail` in the detail handler.

```go
// internal/store/read.go
func (s *Store) OpenCheckpointsForRun(ctx context.Context, run uuid.UUID) ([]Checkpoint, error){
  rows,err:=s.pool.Query(ctx,`SELECT id,run_id,node_id,question,resolved,COALESCE(answer,'')
    FROM checkpoints WHERE run_id=$1 AND NOT resolved ORDER BY created_at`,run)
  if err!=nil{return nil,err}; defer rows.Close()
  var out []Checkpoint
  for rows.Next(){ var c Checkpoint
    if err:=rows.Scan(&c.ID,&c.RunID,&c.NodeID,&c.Question,&c.Resolved,&c.Answer);err!=nil{return nil,err}
    out=append(out,c) }
  return out,rows.Err()
}
```
Add `Checkpoints []store.Checkpoint `json:"checkpoints"`` to `RunDetail` and populate it in the `GET /api/runs/{id}` handler.

- [ ] **Step 4: Run — PASS** · **Step 5: Commit** `feat: run detail includes open checkpoints`

### Task 9.4: Background run loop + stub runner

**Files:** Create `internal/api/runloop.go`, `internal/api/runloop_test.go`; Modify `cmd/serve/main.go`

**Interfaces:**
- Produces: `api.StubRunner()` (a `claude.Runner` returning `{status:ok, tests:[{expected_red_reason:"assertion"}]}`), `api.StubGate()` (a `gate.TestRunner` returning `AssertionFail`), and `api.TickAll(ctx, deps) (int, error)` — promote-ready then process one node across all ready runs; returns nodes processed. `StartRunLoop(ctx, deps, every)` runs `TickAll` on a ticker.

- [ ] **Step 1: Failing test** — create a graph, call `TickAll` in a loop, assert nodes reach `done`.

```go
// internal/api/runloop_test.go
func TestTickAllAdvancesRun(t *testing.T){
  ctx:=context.Background(); s:=newStore(t); defer s.Close()
  run,_,_:=s.CreateGraph(ctx,"acme",[]store.TaskSpec{{Key:"a",Type:"implement"}})
  deps:=worker.Deps{Store:s,Queue:queue.New(s.Pool()),Log:events.New(s.Pool()),
    Claude:StubRunner(),Gate:StubGate()}
  for i:=0;i<5;i++{ if _,err:=TickAll(ctx,deps);err!=nil{t.Fatal(err)} }
  var done int; s.Pool().QueryRow(ctx,`SELECT count(*) FROM nodes WHERE run_id=$1 AND status='done'`,run).Scan(&done)
  if done!=1{t.Fatalf("node should be done, got %d",done)}
}
```

- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement** `runloop.go` (StubRunner/StubGate using `claude.NewFake`/`gate.NewFakeRunner`; `TickAll` = `Queue.PromoteReady` then `worker.RunOnce`; `StartRunLoop` = `for range time.Tick(every) { TickAll(...) }` in a goroutine honoring `ctx`).
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Wire into serve** — in `cmd/serve/main.go`, build `worker.Deps` with StubRunner/StubGate and `go api.StartRunLoop(ctx, deps, time.Second)` before `ListenAndServe`.
- [ ] **Step 6: Commit** `feat: background run loop + stub runner (runs progress live)`

---

## Phase 9B — Frontend: wire the platform UI to the engine

Each task edits `internal/api/web/index.html`, rebuilds `bin/serve`, and is **verified by driving the page with Playwright** (navigate localhost:7788, act, screenshot, assert via `/api/*`). Replace the file's placeholder inline JS incrementally.

### Task 9.5: Config tab → create a run

- [ ] **Step 1: Implement** — the Config tab's "Scaffold" button reads the form and `POST`s to `/api/runs`, then switches to the CI/CD tab with the new run id in a module-level `state.runId`.

```js
async function scaffold() {
  const body = { project: document.querySelector('#svc-name').value || 'acme-saas', landing: true, google_auth: true };
  const r = await fetch('/api/runs', { method:'POST', headers:{'content-type':'application/json'}, body: JSON.stringify(body) });
  const { id } = await r.json();
  state.runId = id; switchTab('workflows'); startPolling();
}
```
(Give the service-name input `id="svc-name"` and bind `scaffold` to the Scaffold button.)

- [ ] **Step 2: Verify** — Playwright: click Config → fill name → click Scaffold → assert `GET /api/runs` now returns a run with that project; screenshot.
- [ ] **Step 3: Commit** `feat(ui): Config tab creates a real run`

### Task 9.6: CI/CD tab → live pipeline rows

- [ ] **Step 1: Implement** — `startPolling()` fetches `/api/runs/{state.runId}` every 1.5s and renders `nodes` as the Actions Pipeline rows, using the existing `.t-row` markup, with the status icon driven by `node.status` (done=green check, running/ready=blue spinner dot, failed=red, blocked=amber, pending=grey).

```js
function renderPipeline(d){
  document.querySelector('#pipeline').innerHTML = d.nodes.map(n => `
    <div class="t-row"><span class="s-${n.status}">●</span>
      <div class="t-title">${n.type}</div><div class="t-commit">${n.status}</div></div>`).join('');
}
```

- [ ] **Step 2: Verify** — Playwright: after 9.5, watch the CI/CD tab; poll until a node shows `done`; screenshot showing mixed statuses.
- [ ] **Step 3: Commit** `feat(ui): CI/CD tab renders live run nodes`

### Task 9.7: Dev tab → live diff, event log, checkpoint + steer

- [ ] **Step 1: Implement** —
  - Render the run's **event log** (from `d.events`) in the Dev tab's chat area, color-coded by kind (reuse the runs.html scheme).
  - When `d.checkpoints` is non-empty, render an **approval card** (using `sample.html`'s `.approval-card` styling) with Confirm/Skip that `POST`s `/api/checkpoints/{id}/resolve` (`{"answer":"..."}`), then re-polls.
  - The composer "send" posts a steer message: for now `POST /api/runs/{id}/steer` **or** (if that endpoint is out of scope) append locally and log — pick one and note it.

```js
function renderCheckpoints(d){
  const cp = (d.checkpoints||[])[0]; const el = document.querySelector('#checkpoint');
  if (!cp) { el.innerHTML=''; return; }
  el.innerHTML = `<div class="approval-card"><b>${cp.question}</b>
    <button data-a="yes">Confirm</button><button data-a="skip">Skip</button></div>`;
  el.querySelectorAll('button').forEach(b => b.onclick = async () => {
    await fetch('/api/checkpoints/'+cp.id+'/resolve',{method:'POST',headers:{'content-type':'application/json'},
      body:JSON.stringify({answer:b.dataset.a})}); loadDetail();
  });
}
```

- [ ] **Step 2: Verify** — Playwright: drive a run to a checkpoint (seed one whose auth node raises it via a checkpoint-stub runner), see the card, click Confirm, assert the node returns to `ready`/progresses; screenshot.
- [ ] **Step 3: Commit** `feat(ui): Dev tab shows events + resolves checkpoints`

### Task 9.8: Schema + Swagger tabs → real sources

- [ ] **Step 1: Implement** —
  - **Swagger tab:** add `GET /api/openapi` in Go returning a spec built from the known template routes (mirror `templates/swagger-kit`), and render the endpoint list + method chips from it.
  - **Schema tab:** add `GET /api/schema` returning the template's entities (users/workspaces/workspace_members/items with fields + relations, as a static-but-real descriptor) and render the existing schema-node visualizer from it.

- [ ] **Step 2: Verify** — Playwright: open each tab, assert the rendered endpoints/entities match the API JSON; screenshot both.
- [ ] **Step 3: Commit** `feat(ui): Swagger + Schema tabs read live endpoints`

---

## Self-Review

- **Spec coverage:** every `sample.html` tab is wired — Config (9.5), CI/CD (9.6), Dev (9.7), Schema+Swagger (9.8); backend create/resolve/read + run loop (9.1–9.4). Header workspace switcher and search remain visual (out of scope; note in commit).
- **Placeholders:** Go tasks carry real code + tests; frontend tasks carry real JS snippets + explicit Playwright verification (no unit-test theater for the DOM).
- **Type consistency:** `RunDetail` extended once (9.3) and consumed by 9.6/9.7; `store.Checkpoint`, `store.Node`, `store.EventRow` reused as-is; run id flows as `state.runId`.
- **Dependency:** 9.6–9.7 depend on 9.1/9.3/9.4; 9.8 is independent. `/api/runs/{id}/steer` is optional — if included, add it as a sibling of 9.2 (append a `steer` event); otherwise mark the composer as local-only.

## Open decisions (resolve before/inside their task)
- **Steer semantics** (9.7): does "send" enqueue a new task, or just record intent? Simplest v1: record a `steer` event; real steering waits for the live Claude runner.
- **Real runner:** this plan uses the stub runner so the UI is live now; swapping in `claude.NewExec` + the Node `gate.NodeRunner` is a separate follow-up (the seams already exist).
