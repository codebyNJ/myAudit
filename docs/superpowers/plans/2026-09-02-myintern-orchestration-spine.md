# myIntern — Implementation Plan (Phase 0–1: Orchestration Spine)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up myIntern's stateful, Postgres-backed Go orchestration engine — the spine that plans a build graph, drives headless Claude, RED-gates its output, and records everything as typed events — proven end-to-end with fakes.

**Architecture:** A Go control plane where a build run is a **stateful graph** persisted in **Postgres**. A **SKIP LOCKED** job queue feeds a **bounded worker pool**; each node reconstructs its context bundle from `RunState`, calls Claude via headless mode, and gates the structured output. All operational data (state, logs, audits, metrics) lives in Postgres and is read by query — never `.md`. The orchestrator never writes app code; Claude does.

**Tech Stack:** Go 1.23+ · Postgres 16 (via Docker) · `jackc/pgx/v5` · `pressly/goose` (migrations) · `log/slog` (stdlib logging) · stdlib `testing` against real Postgres.

**Spec:** This plan's Architecture Reference (below) + [`../../architecture.md`](../../architecture.md), [`../../tech-stack.md`](../../tech-stack.md). Decisions from the design conversation are captured in the Architecture Reference so the plan is self-contained.

## Global Constraints

- **DB-first:** all runtime records/logs/audits/metrics → Postgres, queryable. `.md` only for major human docs. No operational data in files.
- **Orchestrator never writes app code.** It plans, grounds, budgets context, drives Claude, gates output.
- **We own the memory**, not Claude. Each node rebuilds context from `RunState`; never rely on Claude session continuity across invocations.
- **Context by retrieval, not accumulation.** Per-task bounded bundle; prior work enters as summaries.
- **Bounded concurrency.** A weighted semaphore caps concurrent headless Claude processes; excess stays queued.
- **Genuine tests only.** Every generated test passes the RED gate (fails on empty impl for the right reason) before it counts.
- Module path: `myintern`. All paths below are relative to the platform repo root (`ankor-platform/`).

---

## Architecture Reference (the spec, condensed)

**RunState (persisted in Postgres):**
- `runs` — one build session (project, status, budget).
- `nodes` — graph tasks: type, status, deps, `input_snapshot` (JSONB), `output` (JSONB), attempts.
- `events` — append-only typed log (the logger + audit trail), correlated by `run_id`/`node_id`/`span_id`.
- (later phases add `summaries`, `checkpoints`, `doc_briefs`, `findings`.)

**Node lifecycle:** `pending → ready(deps met) → claimed(SKIP LOCKED) → running(snapshot input) → {done | failed(retry+backoff) | needs_checkpoint} → txn state update → route next`.

**Node types (Phase 1 implements `implement`; others roadmapped):** `plan · doc_acquire · distill · test_write · implement · test_run · audit · checkpoint`.

**Output contract (Claude → orchestrator, parseable):**
```json
{ "summary": "...", "status": "ok|blocked|needs_checkpoint",
  "files": [{"path":"...","action":"create|edit|delete","content":"..."}],
  "tests": [{"path":"...","content":"...","expected_red_reason":"assertion|import|other"}],
  "commands": ["..."], "new_dependencies": [{"name":"...","version":"...","reason":"..."}],
  "checkpoint": {"needed":false,"question":"","options":[]},
  "security_notes": ["..."] }
```

**Event kinds (frozen vocabulary):** `run.start run.end node.start node.end node.retry node.fail claude.request claude.response doc.fetch doc.distill test.write gate.red test.run audit.finding checkpoint.raise checkpoint.resolve error`.

---

## Phase Roadmap

- **Phase 0 — Project scaffold** (detailed below): Go module, Docker Postgres, config, migrations, test harness.
- **Phase 1 — Orchestration spine** (detailed below): store · logger · queue · dep-gating · contract parse · Claude runner · RED gate · worker + implement-node (end-to-end with fakes).
- **Phase 2 — Planner & graph routing** *(own plan later):* idea → task graph; conditional edges (RED loop, checkpoint interrupt); resume/replay from snapshots.
- **Phase 3 — Doc-centric pipeline** *(own plan later):* capability detection → doc acquire → distill to briefs (Postgres/MinIO) → grounded bundles; lazy version-keyed cache.
- **Phase 4 — Template integration** *(own plan later):* drive `init-project.js`; "copy Item" recipe as guardrail; real Node `test_run` + RED gate against real Mongo.
- **Phase 5 — Agent kit module** *(own plan later):* Vercel AI SDK kit + conversation sessions (Valkey+Mongo) + streaming; ElevenLabs adapter as the reference extension.
- **Phase 6 — Auto-mode router** *(own plan later):* task classification → (model tier, autonomy, checkpoint); budgets; never-auto for irreversible.
- **Phase 7 — UI surfaces (beautifului only)** *(own plan later):* Dev/Config/Swagger/CI-CD/Schema per `sample.html`, the logger/query views, and the **development build-flow graph UI** (live node/edge graph over `RunState` — detailed here later).
- **Phase 8 — Storage & deploy** *(own plan later):* MinIO file-storage module, Terraform + Docker deploy, Swagger generation.

---

## Phase 0 — Project scaffold

### Task 0.1: Go module, Docker Postgres, config loader

**Files:**
- Create: `go.mod`, `docker-compose.yml`, `internal/config/config.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Produces: `config.Load() (config.Config, error)` where `Config{ DatabaseURL string; MaxConcurrentClaude int }`.

- [ ] **Step 1: Init module and compose**

```bash
cd ankor-platform && go mod init myintern
```

Create `docker-compose.yml`:
```yaml
services:
  postgres:
    image: postgres:16
    environment: { POSTGRES_USER: myintern, POSTGRES_PASSWORD: dev, POSTGRES_DB: myintern }
    ports: ["5433:5432"]
    healthcheck: { test: ["CMD-SHELL","pg_isready -U myintern"], interval: 2s, timeout: 3s, retries: 20 }
```

- [ ] **Step 2: Write the failing test**

```go
// internal/config/config_test.go
package config
import ("os";"testing")
func TestLoadReadsEnv(t *testing.T){
  os.Setenv("DATABASE_URL","postgres://x"); os.Setenv("MAX_CONCURRENT_CLAUDE","3")
  c,err:=Load(); if err!=nil{t.Fatal(err)}
  if c.DatabaseURL!="postgres://x"{t.Fatalf("url=%q",c.DatabaseURL)}
  if c.MaxConcurrentClaude!=3{t.Fatalf("n=%d",c.MaxConcurrentClaude)}
}
func TestLoadFailsWithoutURL(t *testing.T){
  os.Unsetenv("DATABASE_URL"); if _,err:=Load();err==nil{t.Fatal("want error")}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL — `undefined: Load`.

- [ ] **Step 4: Write minimal implementation**

```go
// internal/config/config.go
package config
import ("errors";"os";"strconv")
type Config struct{ DatabaseURL string; MaxConcurrentClaude int }
func Load() (Config, error) {
  url := os.Getenv("DATABASE_URL")
  if url == "" { return Config{}, errors.New("DATABASE_URL required") }
  n, _ := strconv.Atoi(os.Getenv("MAX_CONCURRENT_CLAUDE")); if n == 0 { n = 2 }
  return Config{DatabaseURL: url, MaxConcurrentClaude: n}, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/config/` → Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod docker-compose.yml internal/config && git commit -m "chore: scaffold module, Postgres, config loader"
```

### Task 0.2: Migrations + store connection

**Files:**
- Create: `migrations/00001_init.sql`, `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Produces: `store.Open(ctx, url) (*store.Store, error)`; `(*Store).Pool() *pgxpool.Pool`; `(*Store).Ping(ctx) error`.

- [ ] **Step 1: Add deps**

```bash
go get github.com/jackc/pgx/v5/pgxpool github.com/pressly/goose/v3
```

- [ ] **Step 2: Write migration**

```sql
-- migrations/00001_init.sql
-- +goose Up
CREATE TABLE runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'running',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE nodes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID NOT NULL REFERENCES runs(id),
  type TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'pending',
  deps UUID[] NOT NULL DEFAULT '{}',
  input_snapshot JSONB, output JSONB, attempts INT NOT NULL DEFAULT 0,
  claimed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE INDEX idx_nodes_run_status ON nodes(run_id, status);
CREATE TABLE events (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID NOT NULL, node_id UUID, span_id TEXT,
  ts TIMESTAMPTZ NOT NULL DEFAULT now(),
  level TEXT NOT NULL, kind TEXT NOT NULL, msg TEXT, attrs JSONB);
CREATE INDEX idx_events_run ON events(run_id, ts);
-- +goose Down
DROP TABLE events; DROP TABLE nodes; DROP TABLE runs;
```

- [ ] **Step 3: Write the failing test**

```go
// internal/store/store_test.go
package store
import ("context";"os";"testing")
func testURL(t *testing.T) string {
  u := os.Getenv("TEST_DATABASE_URL")
  if u == "" { t.Skip("set TEST_DATABASE_URL (docker compose up -d)") }
  return u
}
func TestOpenAndPing(t *testing.T){
  s,err:=Open(context.Background(),testURL(t)); if err!=nil{t.Fatal(err)}
  defer s.Close()
  if err:=s.Ping(context.Background());err!=nil{t.Fatal(err)}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/store/` → Expected: FAIL — `undefined: Open`.

- [ ] **Step 5: Write minimal implementation**

```go
// internal/store/store.go
package store
import ("context";"github.com/jackc/pgx/v5/pgxpool")
type Store struct{ pool *pgxpool.Pool }
func Open(ctx context.Context, url string) (*Store, error) {
  p, err := pgxpool.New(ctx, url); if err != nil { return nil, err }
  return &Store{pool: p}, nil
}
func (s *Store) Pool() *pgxpool.Pool { return s.pool }
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) Close() { s.pool.Close() }
```

- [ ] **Step 6: Run migration + test**

```bash
docker compose up -d
export TEST_DATABASE_URL="postgres://myintern:dev@localhost:5433/myintern"
goose -dir migrations postgres "$TEST_DATABASE_URL" up
go test ./internal/store/
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add migrations internal/store && git commit -m "feat: migrations + store connection"
```

---

## Phase 1 — Orchestration spine

### Task 1: RunStore — create run, add node, fetch node

**Files:**
- Create: `internal/store/runs.go`
- Test: `internal/store/runs_test.go`

**Interfaces:**
- Produces: `(*Store).CreateRun(ctx, project string) (uuid.UUID, error)`; `(*Store).AddNode(ctx, runID uuid.UUID, typ string, deps []uuid.UUID) (uuid.UUID, error)`; `(*Store).GetNode(ctx, id uuid.UUID) (Node, error)` where `Node{ID, RunID uuid.UUID; Type, Status string; Deps []uuid.UUID}`.

- [ ] **Step 1: Write the failing test**

```go
// internal/store/runs_test.go
package store
import ("context";"testing")
func TestCreateRunAndNode(t *testing.T){
  ctx:=context.Background(); s,_:=Open(ctx,testURL(t)); defer s.Close()
  run,err:=s.CreateRun(ctx,"acme-saas"); if err!=nil{t.Fatal(err)}
  n,err:=s.AddNode(ctx,run,"implement",nil); if err!=nil{t.Fatal(err)}
  got,err:=s.GetNode(ctx,n); if err!=nil{t.Fatal(err)}
  if got.Type!="implement"||got.Status!="pending"{t.Fatalf("%+v",got)}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run CreateRun` → Expected: FAIL — `undefined: CreateRun`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/store/runs.go
package store
import ("context";"github.com/google/uuid")
type Node struct{ ID, RunID uuid.UUID; Type, Status string; Deps []uuid.UUID }
func (s *Store) CreateRun(ctx context.Context, project string) (uuid.UUID, error){
  var id uuid.UUID
  err:=s.pool.QueryRow(ctx,`INSERT INTO runs(project) VALUES($1) RETURNING id`,project).Scan(&id)
  return id,err
}
func (s *Store) AddNode(ctx context.Context, run uuid.UUID, typ string, deps []uuid.UUID) (uuid.UUID, error){
  if deps==nil{deps=[]uuid.UUID{}}
  var id uuid.UUID
  err:=s.pool.QueryRow(ctx,`INSERT INTO nodes(run_id,type,deps) VALUES($1,$2,$3) RETURNING id`,run,typ,deps).Scan(&id)
  return id,err
}
func (s *Store) GetNode(ctx context.Context, id uuid.UUID) (Node, error){
  var n Node
  err:=s.pool.QueryRow(ctx,`SELECT id,run_id,type,status,deps FROM nodes WHERE id=$1`,id).
    Scan(&n.ID,&n.RunID,&n.Type,&n.Status,&n.Deps)
  return n,err
}
```
Run `go get github.com/google/uuid` if needed.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/store/ -run CreateRun` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/store && git commit -m "feat: run/node store"
```

### Task 2: Event logger (typed events → Postgres + slog)

**Files:**
- Create: `internal/events/events.go`
- Test: `internal/events/events_test.go`

**Interfaces:**
- Consumes: `*pgxpool.Pool`.
- Produces: `events.New(pool) *Logger`; `(*Logger).Log(ctx, Event)` where `Event{RunID uuid.UUID; NodeID *uuid.UUID; SpanID, Level, Kind, Msg string; Attrs map[string]any}`.

- [ ] **Step 1: Write the failing test**

```go
// internal/events/events_test.go
package events
import ("context";"testing";"github.com/google/uuid";"github.com/jackc/pgx/v5/pgxpool";"os")
func pool(t *testing.T)*pgxpool.Pool{ u:=os.Getenv("TEST_DATABASE_URL"); if u==""{t.Skip("no db")}; p,_:=pgxpool.New(context.Background(),u); return p }
func TestLogWritesRow(t *testing.T){
  ctx:=context.Background(); p:=pool(t); defer p.Close()
  run:=uuid.New(); p.Exec(ctx,`INSERT INTO runs(id,project) VALUES($1,'t')`,run)
  l:=New(p); l.Log(ctx,Event{RunID:run,Kind:"node.start",Level:"info",Msg:"hi"})
  var n int; p.QueryRow(ctx,`SELECT count(*) FROM events WHERE run_id=$1 AND kind='node.start'`,run).Scan(&n)
  if n!=1{t.Fatalf("rows=%d",n)}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/events/` → Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/events/events.go
package events
import ("context";"encoding/json";"log/slog";"github.com/google/uuid";"github.com/jackc/pgx/v5/pgxpool")
type Event struct{ RunID uuid.UUID; NodeID *uuid.UUID; SpanID,Level,Kind,Msg string; Attrs map[string]any }
type Logger struct{ pool *pgxpool.Pool }
func New(p *pgxpool.Pool) *Logger { return &Logger{pool:p} }
func (l *Logger) Log(ctx context.Context, e Event){
  if e.Level==""{e.Level="info"}
  attrs,_:=json.Marshal(e.Attrs)
  _,err:=l.pool.Exec(ctx,`INSERT INTO events(run_id,node_id,span_id,level,kind,msg,attrs)
    VALUES($1,$2,$3,$4,$5,$6,$7)`,e.RunID,e.NodeID,nullStr(e.SpanID),e.Level,e.Kind,e.Msg,attrs)
  if err!=nil{ slog.Error("event insert failed","err",err) }
  slog.Info(e.Kind,"run",e.RunID,"msg",e.Msg)
}
func nullStr(s string) any { if s==""{return nil}; return s }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/events/` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/events && git commit -m "feat: typed event logger (Postgres + slog)"
```

> Note: batched async insert is a Phase-2 optimization; `ponytail: synchronous insert now, buffered flusher when volume warrants`.

### Task 3: Job queue — enqueue, claim (SKIP LOCKED), complete

**Files:**
- Create: `internal/queue/queue.go`
- Test: `internal/queue/queue_test.go`

**Interfaces:**
- Produces: `queue.New(pool) *Queue`; `(*Queue).Claim(ctx) (*ClaimedNode, error)` (returns nil if none); `(*Queue).Complete(ctx, id uuid.UUID, output []byte) error`; `(*Queue).Fail(ctx, id uuid.UUID) error`. `ClaimedNode{ID, RunID uuid.UUID; Type string}`.

- [ ] **Step 1: Write the failing test**

```go
// internal/queue/queue_test.go
package queue
import ("context";"testing";"github.com/google/uuid";"github.com/jackc/pgx/v5/pgxpool";"os")
func pool(t *testing.T)*pgxpool.Pool{ u:=os.Getenv("TEST_DATABASE_URL"); if u==""{t.Skip("no db")}; p,_:=pgxpool.New(context.Background(),u); return p }
func TestClaimReturnsReadyNodeOnce(t *testing.T){
  ctx:=context.Background(); p:=pool(t); defer p.Close()
  run:=uuid.New(); p.Exec(ctx,`INSERT INTO runs(id,project) VALUES($1,'t')`,run)
  var nid uuid.UUID
  p.QueryRow(ctx,`INSERT INTO nodes(run_id,type,status) VALUES($1,'implement','ready') RETURNING id`,run).Scan(&nid)
  q:=New(p)
  c1,_:=q.Claim(ctx); if c1==nil||c1.ID!=nid{t.Fatal("first claim should get node")}
  c2,_:=q.Claim(ctx); if c2!=nil{t.Fatal("second claim should be empty (already running)")}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/queue/` → Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/queue/queue.go
package queue
import ("context";"github.com/google/uuid";"github.com/jackc/pgx/v5/pgxpool")
type ClaimedNode struct{ ID, RunID uuid.UUID; Type string }
type Queue struct{ pool *pgxpool.Pool }
func New(p *pgxpool.Pool) *Queue { return &Queue{pool:p} }
func (q *Queue) Claim(ctx context.Context) (*ClaimedNode, error){
  row:=q.pool.QueryRow(ctx,`
    UPDATE nodes SET status='running', claimed_at=now(), attempts=attempts+1
    WHERE id = (SELECT id FROM nodes WHERE status='ready'
                ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1)
    RETURNING id, run_id, type`)
  var c ClaimedNode; err:=row.Scan(&c.ID,&c.RunID,&c.Type)
  if err!=nil { if err.Error()=="no rows in result set"{return nil,nil}; return nil,err }
  return &c,nil
}
func (q *Queue) Complete(ctx context.Context, id uuid.UUID, output []byte) error{
  _,err:=q.pool.Exec(ctx,`UPDATE nodes SET status='done', output=$2 WHERE id=$1`,id,output); return err
}
func (q *Queue) Fail(ctx context.Context, id uuid.UUID) error{
  _,err:=q.pool.Exec(ctx,`UPDATE nodes SET status='failed' WHERE id=$1`,id); return err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/queue/` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/queue && git commit -m "feat: SKIP LOCKED job queue"
```

### Task 4: Dependency gating — promote pending→ready when deps done

**Files:**
- Modify: `internal/queue/queue.go`
- Test: `internal/queue/gating_test.go`

**Interfaces:**
- Produces: `(*Queue).PromoteReady(ctx) (int, error)` — sets `status='ready'` for `pending` nodes whose every dep is `done`; returns count promoted.

- [ ] **Step 1: Write the failing test**

```go
// internal/queue/gating_test.go
package queue
import ("context";"testing";"github.com/google/uuid")
func TestPromoteReadyRespectsDeps(t *testing.T){
  ctx:=context.Background(); p:=pool(t); defer p.Close()
  run:=uuid.New(); p.Exec(ctx,`INSERT INTO runs(id,project) VALUES($1,'t')`,run)
  var a,b uuid.UUID
  p.QueryRow(ctx,`INSERT INTO nodes(run_id,type,status) VALUES($1,'a','done') RETURNING id`,run).Scan(&a)
  p.QueryRow(ctx,`INSERT INTO nodes(run_id,type,status,deps) VALUES($1,'b','pending',$2) RETURNING id`,run,[]uuid.UUID{a}).Scan(&b)
  q:=New(p); n,err:=q.PromoteReady(ctx); if err!=nil{t.Fatal(err)}
  if n<1{t.Fatal("b should be promoted")}
  var st string; p.QueryRow(ctx,`SELECT status FROM nodes WHERE id=$1`,b).Scan(&st)
  if st!="ready"{t.Fatalf("status=%s",st)}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/queue/ -run Promote` → Expected: FAIL — `undefined: PromoteReady`.

- [ ] **Step 3: Write minimal implementation**

```go
// append to internal/queue/queue.go
func (q *Queue) PromoteReady(ctx context.Context) (int, error){
  tag,err:=q.pool.Exec(ctx,`
    UPDATE nodes n SET status='ready'
    WHERE n.status='pending'
      AND NOT EXISTS (
        SELECT 1 FROM unnest(n.deps) d
        JOIN nodes dn ON dn.id=d WHERE dn.status<>'done')`)
  return int(tag.RowsAffected()),err
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/queue/ -run Promote` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/queue && git commit -m "feat: dependency gating (pending -> ready)"
```

### Task 5: Output contract — types + parse + reject undeclared deps

**Files:**
- Create: `internal/contract/contract.go`
- Test: `internal/contract/contract_test.go`

**Interfaces:**
- Produces: `contract.Parse(raw []byte) (Result, error)`; `Result{Summary,Status string; Files []File; Tests []Test; NewDeps []Dep; Checkpoint Checkpoint; SecurityNotes []string}`; `(Result).RequiresApproval() bool` (true if `len(NewDeps)>0` or `Checkpoint.Needed`).

- [ ] **Step 1: Write the failing test**

```go
// internal/contract/contract_test.go
package contract
import "testing"
func TestParseValid(t *testing.T){
  raw:=[]byte(`{"summary":"x","status":"ok","files":[{"path":"a.js","action":"create","content":"y"}],"tests":[],"new_dependencies":[]}`)
  r,err:=Parse(raw); if err!=nil{t.Fatal(err)}
  if r.Status!="ok"||len(r.Files)!=1{t.Fatalf("%+v",r)}
  if r.RequiresApproval(){t.Fatal("no deps -> no approval")}
}
func TestNewDepsRequireApproval(t *testing.T){
  raw:=[]byte(`{"status":"ok","new_dependencies":[{"name":"left-pad","version":"1","reason":"x"}]}`)
  r,_:=Parse(raw); if !r.RequiresApproval(){t.Fatal("new dep must require approval")}
}
func TestParseRejectsBadJSON(t *testing.T){ if _,err:=Parse([]byte("{")); err==nil{t.Fatal("want error")} }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/contract/` → Expected: FAIL — `undefined: Parse`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/contract/contract.go
package contract
import "encoding/json"
type File struct{ Path,Action,Content string }
type Test struct{ Path,Content,ExpectedRedReason string `json:"expected_red_reason"` }
type Dep struct{ Name,Version,Reason string }
type Checkpoint struct{ Needed bool; Question string; Options []string }
type Result struct{
  Summary,Status string
  Files []File; Tests []Test
  NewDeps []Dep `json:"new_dependencies"`
  Checkpoint Checkpoint; SecurityNotes []string `json:"security_notes"`
}
func Parse(raw []byte) (Result, error){ var r Result; err:=json.Unmarshal(raw,&r); return r,err }
func (r Result) RequiresApproval() bool { return len(r.NewDeps)>0 || r.Checkpoint.Needed }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/contract/` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/contract && git commit -m "feat: output contract parse + approval gate"
```

### Task 6: Claude runner — interface, fake, real spawn

**Files:**
- Create: `internal/claude/claude.go`, `internal/claude/fake.go`
- Test: `internal/claude/claude_test.go`

**Interfaces:**
- Produces: `type Runner interface { Run(ctx, prompt string) (contract.Result, error) }`; `claude.NewFake(result contract.Result) Runner`; `claude.NewExec(bin string) Runner` (spawns `bin -p <prompt>`, parses stdout JSON via `contract.Parse`).

- [ ] **Step 1: Write the failing test**

```go
// internal/claude/claude_test.go
package claude
import ("context";"testing";"myintern/internal/contract")
func TestFakeReturnsResult(t *testing.T){
  want:=contract.Result{Status:"ok",Summary:"done"}
  r:=NewFake(want)
  got,err:=r.Run(context.Background(),"any prompt"); if err!=nil{t.Fatal(err)}
  if got.Summary!="done"{t.Fatalf("%+v",got)}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/claude/` → Expected: FAIL — `undefined: NewFake`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/claude/claude.go
package claude
import ("context";"os/exec";"myintern/internal/contract")
type Runner interface{ Run(ctx context.Context, prompt string) (contract.Result, error) }
type execRunner struct{ bin string }
func NewExec(bin string) Runner { return &execRunner{bin:bin} }
func (e *execRunner) Run(ctx context.Context, prompt string) (contract.Result, error){
  out,err:=exec.CommandContext(ctx,e.bin,"-p",prompt).Output()
  if err!=nil{ return contract.Result{},err }
  return contract.Parse(out)
}
```
```go
// internal/claude/fake.go
package claude
import ("context";"myintern/internal/contract")
type fakeRunner struct{ res contract.Result }
func NewFake(res contract.Result) Runner { return &fakeRunner{res:res} }
func (f *fakeRunner) Run(_ context.Context, _ string) (contract.Result, error){ return f.res, nil }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/claude/` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/claude && git commit -m "feat: claude runner interface + fake + exec"
```

### Task 7: RED gate — classify a generated test against empty impl

**Files:**
- Create: `internal/gate/gate.go`, `internal/gate/fake.go`
- Test: `internal/gate/gate_test.go`

**Interfaces:**
- Consumes: `type TestRunner interface { RunAgainstEmpty(ctx, test contract.Test) (Outcome, error) }` with `Outcome` one of `OutcomePass, OutcomeImportError, OutcomeAssertionFail`.
- Produces: `gate.Verdict(o Outcome) (accept bool, reason string)` — accept only `OutcomeAssertionFail`; `gate.NewFakeRunner(o Outcome) TestRunner`.

- [ ] **Step 1: Write the failing test**

```go
// internal/gate/gate_test.go
package gate
import "testing"
func TestVerdict(t *testing.T){
  if a,_:=Verdict(OutcomePass); a { t.Fatal("pass-on-empty must be rejected (tautological)") }
  if a,_:=Verdict(OutcomeImportError); a { t.Fatal("import error must be rejected") }
  if a,_:=Verdict(OutcomeAssertionFail); !a { t.Fatal("assertion fail is genuine -> accept") }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gate/` → Expected: FAIL — `undefined: Verdict`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/gate/gate.go
package gate
import ("context";"myintern/internal/contract")
type Outcome int
const ( OutcomePass Outcome = iota; OutcomeImportError; OutcomeAssertionFail )
type TestRunner interface{ RunAgainstEmpty(ctx context.Context, test contract.Test) (Outcome, error) }
func Verdict(o Outcome) (bool, string){
  switch o {
  case OutcomeAssertionFail: return true, "genuine: failed on empty impl"
  case OutcomePass: return false, "rejected: tautological (passed on empty impl)"
  default: return false, "rejected: test did not run (import/other)"
  }
}
```
```go
// internal/gate/fake.go
package gate
import ("context";"myintern/internal/contract")
type fakeRunner struct{ o Outcome }
func NewFakeRunner(o Outcome) TestRunner { return &fakeRunner{o:o} }
func (f *fakeRunner) RunAgainstEmpty(_ context.Context, _ contract.Test) (Outcome, error){ return f.o, nil }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gate/` → Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/gate && git commit -m "feat: RED gate verdict"
```

### Task 8: Worker + implement-node handler (end-to-end, fakes for Claude/tests, real Postgres)

**Files:**
- Create: `internal/worker/worker.go`
- Test: `internal/worker/worker_test.go`

**Interfaces:**
- Consumes: `*store.Store`, `*queue.Queue`, `*events.Logger`, `claude.Runner`, `gate.TestRunner`.
- Produces: `worker.RunOnce(ctx, deps Deps) (bool, error)` — claims one ready node, runs the implement flow (RED-gate each test, then call Claude implementer), records output + events, marks done; returns `false` when no node was ready. `Deps{Store,Queue,Log,Claude,Gate}`.

- [ ] **Step 1: Write the failing test**

```go
// internal/worker/worker_test.go
package worker
import ("context";"os";"testing";"github.com/google/uuid"
  "myintern/internal/store";"myintern/internal/queue";"myintern/internal/events"
  "myintern/internal/claude";"myintern/internal/gate";"myintern/internal/contract")
func TestRunOnceCompletesReadyNode(t *testing.T){
  u:=os.Getenv("TEST_DATABASE_URL"); if u==""{t.Skip("no db")}
  ctx:=context.Background(); s,_:=store.Open(ctx,u); defer s.Close()
  run,_:=s.CreateRun(ctx,"acme"); nid,_:=s.AddNode(ctx,run,"implement",nil)
  s.Pool().Exec(ctx,`UPDATE nodes SET status='ready' WHERE id=$1`,nid)
  deps:=Deps{
    Store:s, Queue:queue.New(s.Pool()), Log:events.New(s.Pool()),
    Claude:claude.NewFake(contract.Result{Status:"ok",Summary:"impl",
      Files:[]contract.File{{Path:"a.js",Action:"create",Content:"x"}}}),
    Gate:gate.NewFakeRunner(gate.OutcomeAssertionFail),
  }
  did,err:=RunOnce(ctx,deps); if err!=nil{t.Fatal(err)}
  if !did{t.Fatal("should have processed a node")}
  n,_:=s.GetNode(ctx,nid); if n.Status!="done"{t.Fatalf("status=%s",n.Status)}
  var ev int; s.Pool().QueryRow(ctx,`SELECT count(*) FROM events WHERE node_id=$1 AND kind='node.end'`,nid).Scan(&ev)
  if ev<1{t.Fatal("expected node.end event")}
  _ = uuid.UUID{}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker/` → Expected: FAIL — `undefined: RunOnce`.

- [ ] **Step 3: Write minimal implementation**

```go
// internal/worker/worker.go
package worker
import ("context";"encoding/json"
  "myintern/internal/store";"myintern/internal/queue";"myintern/internal/events"
  "myintern/internal/claude";"myintern/internal/gate")
type Deps struct{ Store *store.Store; Queue *queue.Queue; Log *events.Logger; Claude claude.Runner; Gate gate.TestRunner }
func RunOnce(ctx context.Context, d Deps) (bool, error){
  c,err:=d.Queue.Claim(ctx); if err!=nil{return false,err}
  if c==nil{return false,nil}
  nid:=c.ID
  d.Log.Log(ctx,events.Event{RunID:c.RunID,NodeID:&nid,Kind:"node.start",Msg:c.Type})
  // (Phase 1: implement node uses a fixed prompt; Phase 2 assembles the real bundle.)
  res,err:=d.Claude.Run(ctx,"IMPLEMENT: "+c.Type)
  if err!=nil{ d.Queue.Fail(ctx,nid); d.Log.Log(ctx,events.Event{RunID:c.RunID,NodeID:&nid,Level:"error",Kind:"node.fail",Msg:err.Error()}); return true,nil }
  // RED-gate any tests the contract carried
  for _,tc:=range res.Tests {
    o,_:=d.Gate.RunAgainstEmpty(ctx,tc)
    accept,reason:=gate.Verdict(o)
    d.Log.Log(ctx,events.Event{RunID:c.RunID,NodeID:&nid,Kind:"gate.red",Msg:reason})
    if !accept { d.Queue.Fail(ctx,nid); return true,nil }
  }
  out,_:=json.Marshal(res)
  if err:=d.Queue.Complete(ctx,nid,out);err!=nil{return true,err}
  d.Log.Log(ctx,events.Event{RunID:c.RunID,NodeID:&nid,Kind:"node.end",Msg:res.Summary})
  return true,nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/worker/` → Expected: PASS.

- [ ] **Step 5: Run the full suite**

Run: `docker compose up -d && go test ./...`
Expected: PASS (store/queue/events/worker skip if `TEST_DATABASE_URL` unset — set it).

- [ ] **Step 6: Commit**

```bash
git add internal/worker && git commit -m "feat: worker RunOnce — implement node end-to-end (spine complete)"
```

---

## Self-Review

- **Spec coverage:** Phase 0–1 cover the Architecture Reference's spine — RunState tables, event logger, SKIP LOCKED queue, dep gating, output contract + approval gate, Claude runner (fake/exec), RED-gate verdict, and an end-to-end implement node. Planner/routing, doc pipeline, template drive, agent kit, auto-mode, UI (incl. build-flow graph), storage/deploy are explicitly deferred to Phases 2–8, each its own plan.
- **Placeholders:** none in Phase 0–1 tasks — every step has runnable code/commands. Phases 2–8 are roadmap entries, not ready tasks (by design per the scope check).
- **Type consistency:** `contract.Result` shared across contract/claude/worker; `gate.Outcome`/`Verdict` and `gate.TestRunner` consistent; `store.Node`, `queue.ClaimedNode`, `events.Event` names match across tasks.

## Notes carried forward (decide before their phase)
- Prompt bundle assembly (Task 8 uses a stub prompt) → **Phase 2**.
- Real Node `test_run`/RED gate against real Mongo (shells into the target project) → **Phase 4**.
- ACP vs custom protocol for the Claude runner transport → revisit at **Phase 2**.
- Claude-auth ToS for headless → resolve before **Phase 4** (external risk).
