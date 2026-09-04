package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"myaudit/internal/agent"
	"myaudit/internal/events"
	"myaudit/internal/queue"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

// recordingAgent fakes the Claude Code seam: records calls + the readOnly flag,
// optionally writes a file into the workspace, returns a configured Result.
type recordingAgent struct {
	calls        int
	lastReadOnly bool
	result       agent.Result
	writeFile    string
}

func (f *recordingAgent) Run(ctx context.Context, ws sandbox.Workspace, task string, readOnly bool) (agent.Result, error) {
	f.calls++
	f.lastReadOnly = readOnly
	if f.writeFile != "" {
		p := filepath.Join(ws.Dir, f.writeFile)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	return f.result, nil
}

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func newDeps(s *store.Store, a Agent, root string) Deps {
	return Deps{
		Store: s, Queue: queue.New(s.DB()), Log: events.New(s.DB()),
		Agent: a, WorkspaceRoot: root, MaxRepairs: 2,
	}
}

// --- pure-unit tests ---

func TestChangedFiles(t *testing.T) {
	d := "diff --git a/src/x.js b/src/x.js\n@@\n+a\ndiff --git a/src/y.js b/src/y.js\n@@\n+b\n"
	f := changedFiles(d)
	if len(f) != 2 || f[0] != "src/x.js" || f[1] != "src/y.js" {
		t.Fatalf("changedFiles=%v", f)
	}
}

func TestDetectTestCmd(t *testing.T) {
	node := t.TempDir()
	os.WriteFile(filepath.Join(node, "package.json"), []byte("{}"), 0o644)
	if name, _, ok := detectTestCmd(node); !ok || name != "npm" {
		t.Fatalf("npm: %s ok=%v", name, ok)
	}
	golang := t.TempDir()
	os.WriteFile(filepath.Join(golang, "go.mod"), []byte("module x"), 0o644)
	if name, _, ok := detectTestCmd(golang); !ok || name != "go" {
		t.Fatalf("go: %s ok=%v", name, ok)
	}
	if _, _, ok := detectTestCmd(t.TempDir()); ok {
		t.Fatal("empty dir should have no runner")
	}
}

// --- DB-backed dispatch tests (SQLite) ---

func TestUnderstandWritesNotesReadOnly(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")
	nid, _ := s.AddNode(ctx, run, "understand", nil)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, nid)

	fa := &recordingAgent{result: agent.Result{OK: true, Summary: "does X. Flow: login → session"}}
	if _, err := RunOnce(ctx, newDeps(s, fa, t.TempDir())); err != nil {
		t.Fatal(err)
	}
	if fa.calls != 1 || !fa.lastReadOnly {
		t.Fatalf("understand should call agent once read-only: calls=%d ro=%v", fa.calls, fa.lastReadOnly)
	}
	if n, _ := s.GetNode(ctx, nid); n.Status != "done" {
		t.Fatalf("status=%s", n.Status)
	}
	notes, _ := s.GetNotes(ctx, run)
	if !strings.Contains(notes, "does X") {
		t.Fatalf("notes missing understanding: %q", notes)
	}
}

func TestVerifySkipsWhenNoRunner(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")
	nid, _ := s.AddNode(ctx, run, "verify", nil)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, nid)

	// Workspace dir has no project markers → verify skips (still completes).
	if _, err := RunOnce(ctx, newDeps(s, &recordingAgent{}, t.TempDir())); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.GetNode(ctx, nid); n.Status != "done" {
		t.Fatalf("verify should complete, status=%s", n.Status)
	}
}
