package worker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myintern/internal/agent"
	"myintern/internal/events"
	"myintern/internal/queue"
	"myintern/internal/sandbox"
	"myintern/internal/store"
)

// recordingAgent fakes the generation seam: records calls, optionally writes a
// file into the workspace, returns a configured Result. (Real claude is
// exercised in the agent package's integration test, per "no mock in the real
// generation path" — this is dispatch-logic testing.)
type recordingAgent struct {
	calls     int
	result    agent.Result
	writeFile string
}

func (f *recordingAgent) Run(ctx context.Context, ws sandbox.Workspace, task string) (agent.Result, error) {
	f.calls++
	if f.writeFile != "" {
		p := filepath.Join(ws.Dir, f.writeFile)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	return f.result, nil
}

type fakeVerifier struct{ err error }

func (v fakeVerifier) Verify(ctx context.Context, ws sandbox.Workspace) error { return v.err }

// --- pure-unit tests (no DB) ---

func TestBuildFeatureTask(t *testing.T) {
	task := buildFeatureTask(store.Resource{Name: "Invoice", Fields: []store.Field{{Name: "amount", Type: "number"}}})
	// Natural prompt: carries the resource + fields and points at the in-repo
	// reference (Item) + CLAUDE.md, without a rigid file checklist.
	for _, want := range []string{"Invoice", "amount (number)", "Item", "CLAUDE.md"} {
		if !strings.Contains(task, want) {
			t.Fatalf("task missing %q:\n%s", want, task)
		}
	}
}

func TestChangedFiles(t *testing.T) {
	d := "diff --git a/src/x.js b/src/x.js\n@@\n+a\ndiff --git a/src/y.js b/src/y.js\n@@\n+b\n"
	f := changedFiles(d)
	if len(f) != 2 || f[0] != "src/x.js" || f[1] != "src/y.js" {
		t.Fatalf("changedFiles=%v", f)
	}
}

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{"My App!": "my-app", "Lumen": "lumen", "  a  b  ": "a-b"} {
		if got := slugify(in); got != want {
			t.Fatalf("slugify(%q)=%q want %q", in, got, want)
		}
	}
}

// --- DB-backed dispatch tests ---

func TestFeatureCompletesOnAgentOKAndVerifyPass(t *testing.T) {
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("no db")
	}
	ctx := context.Background()
	s, _ := store.Open(ctx, u)
	defer s.Close()
	s.Pool().Exec(ctx, `TRUNCATE runs, nodes, events, checkpoints RESTART IDENTITY CASCADE`)

	run, _ := s.CreateRun(ctx, "proj")
	nid, _ := s.AddNode(ctx, run, "feature", nil)
	spec, _ := json.Marshal(store.Resource{Name: "Project", Fields: []store.Field{{Name: "title", Type: "string"}}})
	s.Pool().Exec(ctx, `UPDATE nodes SET status='ready', input_snapshot=$2 WHERE id=$1`, nid, spec)

	fa := &recordingAgent{result: agent.Result{OK: true, Summary: "added Project", CostUSD: 0.02, Tokens: 100}, writeFile: "backend/src/models/project.model.js"}
	deps := Deps{
		Store: s, Queue: queue.New(s.Pool()), Log: events.New(s.Pool()),
		Agent: fa, Verify: fakeVerifier{nil}, WorkspaceRoot: t.TempDir(), MaxRepairs: 2,
	}
	if _, err := RunOnce(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if fa.calls != 1 {
		t.Fatalf("agent should run once, got %d", fa.calls)
	}
	n, _ := s.GetNode(ctx, nid)
	if n.Status != "done" {
		t.Fatalf("status=%s", n.Status)
	}
}

func TestFeatureRepairsThenCheckpoint(t *testing.T) {
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("no db")
	}
	ctx := context.Background()
	s, _ := store.Open(ctx, u)
	defer s.Close()
	s.Pool().Exec(ctx, `TRUNCATE runs, nodes, events, checkpoints RESTART IDENTITY CASCADE`)

	run, _ := s.CreateRun(ctx, "proj")
	nid, _ := s.AddNode(ctx, run, "feature", nil)
	spec, _ := json.Marshal(store.Resource{Name: "Project"})
	s.Pool().Exec(ctx, `UPDATE nodes SET status='ready', input_snapshot=$2 WHERE id=$1`, nid, spec)

	fa := &recordingAgent{result: agent.Result{OK: true, Summary: "tried"}}
	deps := Deps{
		Store: s, Queue: queue.New(s.Pool()), Log: events.New(s.Pool()),
		Agent: fa, Verify: fakeVerifier{errors.New("build broke")}, WorkspaceRoot: t.TempDir(), MaxRepairs: 2,
	}
	if _, err := RunOnce(ctx, deps); err != nil {
		t.Fatal(err)
	}
	if fa.calls != 3 { // initial + 2 repairs
		t.Fatalf("agent should run MaxRepairs+1=3 times, got %d", fa.calls)
	}
	var ev int
	s.Pool().QueryRow(ctx, `SELECT count(*) FROM events WHERE node_id=$1 AND kind='checkpoint.raise'`, nid).Scan(&ev)
	if ev < 1 {
		t.Fatal("expected a checkpoint after exhausting repairs")
	}
}
