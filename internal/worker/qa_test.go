package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"myaudit/internal/agent"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

// TestQALedFlow drives the whole reshaped pipeline with a fake agent:
// map fans out per-module qa cards, qa files a bug ticket blocked on its module
// (QA-over-dev), promotion unblocks it, and the autonomous dev applies a fix that
// lands in review because the suite isn't runnable here. No real model, no cost.
func TestQALedFlow(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")

	// Build a real workspace (two code modules + git baseline) via import.
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "moduleA"), 0o755)
	os.MkdirAll(filepath.Join(src, "lib"), 0o755)
	os.WriteFile(filepath.Join(src, "moduleA", "x.go"), []byte("package a\n"), 0o644)
	os.WriteFile(filepath.Join(src, "lib", "y.go"), []byte("package lib\n"), 0o644)
	root := t.TempDir()
	if _, err := sandbox.Import(ctx, root, run.String(), src); err != nil {
		t.Fatal(err)
	}

	// A ready map node (import already happened above).
	mapID, _ := s.AddNode(ctx, run, "map", nil)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, mapID)

	// 1) map → spawns one qa card per module.
	if _, err := RunOnce(ctx, newDeps(s, &recordingAgent{result: agent.Result{OK: true, Summary: "product map"}}, root)); err != nil {
		t.Fatal(err)
	}
	qaCards := nodesOfType(t, s, run, "qa")
	if len(qaCards) < 2 {
		t.Fatalf("map should fan out ≥2 qa cards, got %d", len(qaCards))
	}
	for _, q := range qaCards {
		if q.Status != "pending" {
			t.Fatalf("qa card should start pending, got %s", q.Status)
		}
	}

	// 2) promote + run one qa card → files a bug ticket, blocked on the qa card.
	q := newDeps(s, &recordingAgent{result: agent.Result{OK: true,
		Summary: `[{"title":"nil deref in handler","file":"moduleA/x.go:1","severity":"high","detail":"reproduce: call with empty body"}]`}}, root)
	if _, err := q.Queue.PromoteReady(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := RunOnce(ctx, q); err != nil {
		t.Fatal(err)
	}
	bugs := nodesOfType(t, s, run, "bug")
	if len(bugs) != 1 {
		t.Fatalf("qa should file exactly 1 bug, got %d", len(bugs))
	}
	if bugs[0].Status != "pending" {
		t.Fatalf("bug must be blocked (pending) until its QA is done, got %s", bugs[0].Status)
	}

	// 3) drain the board: remaining qa cards run first (QA over dev), then the
	// bug is unblocked and the autonomous dev fixes it.
	bugDeps := newDeps(s, &recordingAgent{result: agent.Result{OK: true, Summary: "guarded the nil case"}, writeFile: "moduleA/x.go"}, root)
	for i := 0; i < 10; i++ {
		bugDeps.Queue.PromoteReady(ctx)
		did, err := RunOnce(ctx, bugDeps)
		if err != nil {
			t.Fatal(err)
		}
		if !did {
			break
		}
	}
	bugs = nodesOfType(t, s, run, "bug")
	// No test runner in this workspace → fix can't be verified → parks in review,
	// never a false "fixed" (the old vitest-not-found false-positive is gone).
	if bugs[0].Status != "in_review" {
		t.Fatalf("unverifiable fix should land in review, got %s", bugs[0].Status)
	}
	notes, _ := s.GetNotes(ctx, run)
	if !strings.Contains(notes, "Fix —") {
		t.Fatalf("fix should be logged to notes; notes=%q", notes)
	}
}

func TestClassifyTestsDistinguishesNotRunnable(t *testing.T) {
	// The core of the false-positive fix: "command not found" is not a defect.
	if !looksNotRunnable("sh: vitest: command not found") {
		t.Fatal("vitest-not-found must be classified not-runnable")
	}
	if looksNotRunnable("Expected 2 but received 3") {
		t.Fatal("a real assertion failure must NOT be not-runnable")
	}
}

func nodesOfType(t *testing.T, s *store.Store, run uuid.UUID, typ string) []store.NodeDetail {
	t.Helper()
	cards, err := s.NodeDetailsForRun(context.Background(), run)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.NodeDetail
	for _, c := range cards {
		if c.Type == typ {
			out = append(out, c)
		}
	}
	return out
}
