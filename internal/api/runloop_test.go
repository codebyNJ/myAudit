package api

import (
	"context"
	"testing"

	"myaudit/internal/store"
)

// TickAll should advance a run: a deterministic verify node (no deps, no model,
// no test runner in an empty workspace) promotes to ready and completes.
func TestTickAllAdvancesRun(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _, _ := s.CreateGraph(ctx, "acme", []store.TaskSpec{{Key: "verify", Type: "verify"}})
	deps := NewStubDeps(s)
	for i := 0; i < 5; i++ {
		if _, err := TickAll(ctx, deps); err != nil {
			t.Fatal(err)
		}
	}
	var done int
	s.DB().QueryRowContext(ctx, `SELECT count(*) FROM nodes WHERE run_id=? AND status='done'`, run).Scan(&done)
	if done != 1 {
		t.Fatalf("verify node should be done after ticks, got %d", done)
	}
}
