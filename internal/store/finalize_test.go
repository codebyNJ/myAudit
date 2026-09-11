package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestFinalizeDrainedRuns(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()

	a, _ := s.CreateRun(ctx, "a")
	ia, _ := s.AddNode(ctx, a, "import", nil)
	ma, _ := s.AddNode(ctx, a, "map", []uuid.UUID{ia})
	setStatus(t, s, ia, "done")
	setStatus(t, s, ma, "done")

	b, _ := s.CreateRun(ctx, "b")
	ib, _ := s.AddNode(ctx, b, "import", nil)
	mb, _ := s.AddNode(ctx, b, "map", []uuid.UUID{ib})
	setStatus(t, s, ib, "done")
	setStatus(t, s, mb, "failed")

	c, _ := s.CreateRun(ctx, "c")
	ic, _ := s.AddNode(ctx, c, "import", nil)
	setStatus(t, s, ic, "running")

	finished, err := s.FinalizeDrainedRuns(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]string{}
	for _, r := range finished {
		got[r.ID] = r.Status
	}
	if got[a] != "done" {
		t.Fatalf("run A should finalize done, got %q", got[a])
	}
	if got[b] != "failed" {
		t.Fatalf("run B (map failed) should finalize failed, got %q", got[b])
	}
	if _, ok := got[c]; ok {
		t.Fatal("run C still has a running node; must not be finalized")
	}

	if r, _ := s.GetRun(ctx, a); r.Status != "done" {
		t.Fatalf("run A status not persisted: %q", r.Status)
	}
	if again, _ := s.FinalizeDrainedRuns(ctx); len(again) != 0 {
		t.Fatalf("re-finalize should be a no-op, got %d", len(again))
	}
}

func TestCancelRun(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "x")
	pend, _ := s.AddNode(ctx, run, "qa", nil)
	rdy, _ := s.AddNode(ctx, run, "bug", nil)
	runNode, _ := s.AddNode(ctx, run, "bug", nil)
	doneNode, _ := s.AddNode(ctx, run, "import", nil)
	setStatus(t, s, rdy, "ready")
	setStatus(t, s, runNode, "running")
	setStatus(t, s, doneNode, "done")

	if err := s.CancelRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	want := map[uuid.UUID]string{pend: "cancelled", rdy: "cancelled", runNode: "running", doneNode: "done"}
	for id, exp := range want {
		if got, _ := s.GetNode(ctx, id); got.Status != exp {
			t.Fatalf("node %s: want %s, got %s", id, exp, got.Status)
		}
	}
	if r, _ := s.GetRun(ctx, run); r.Status != "cancelled" {
		t.Fatalf("run should be cancelled, got %s", r.Status)
	}
}

func TestEventsForRunKeepsNewest(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "e")

	for i := 0; i < 10; i++ {
		if _, err := s.db.ExecContext(ctx,
			`INSERT INTO events(run_id, level, kind, msg) VALUES(?,?,?,?)`,
			run, "info", "ev", string(rune('A'+i))); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.EventsForRun(ctx, run, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 events, got %d", len(got))
	}

	if got[0].Msg != "H" || got[2].Msg != "J" {
		t.Fatalf("should keep newest 3 in chronological order, got %q..%q", got[0].Msg, got[2].Msg)
	}
}

func setStatus(t *testing.T, s *Store, id uuid.UUID, status string) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), `UPDATE nodes SET status=? WHERE id=?`, status, id); err != nil {
		t.Fatal(err)
	}
}
