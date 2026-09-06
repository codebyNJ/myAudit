package api

import (
	"context"
	"testing"

	"myaudit/internal/store"
)

// TickAll should advance a run end-to-end on the stub: a map node promotes,
// completes, fans out a qa card (empty workspace → the "(root)" module), the qa
// card completes, the run drains, and FinalizeDrainedRuns marks it done.
func TestTickAllAdvancesRun(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _, _ := s.CreateGraph(ctx, "acme", []store.TaskSpec{{Key: "map", Type: "map"}})
	deps := NewStubDeps(s)
	for i := 0; i < 8; i++ {
		if _, err := TickAll(ctx, deps); err != nil {
			t.Fatal(err)
		}
	}
	var mapDone, qa int
	s.DB().QueryRowContext(ctx, `SELECT count(*) FROM nodes WHERE run_id=? AND type='map' AND status='done'`, run).Scan(&mapDone)
	s.DB().QueryRowContext(ctx, `SELECT count(*) FROM nodes WHERE run_id=? AND type='qa'`, run).Scan(&qa)
	if mapDone != 1 {
		t.Fatalf("map node should be done, got %d", mapDone)
	}
	if qa < 1 {
		t.Fatalf("map should fan out at least one qa card, got %d", qa)
	}
	if r, _ := s.GetRun(ctx, run); r.Status != "done" {
		t.Fatalf("drained run should finalize done, got %q", r.Status)
	}
}
