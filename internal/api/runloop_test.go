package api

import (
	"context"
	"testing"

	"myaudit/internal/store"
)

// TickAll should advance a run: a deterministic config node (no deps, no model)
// promotes to ready and completes via the stub deps.
func TestTickAllAdvancesRun(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _, _ := s.CreateGraph(ctx, "acme", []store.TaskSpec{{Key: "config", Type: "config"}})
	deps := NewStubDeps(s)
	for i := 0; i < 5; i++ {
		if _, err := TickAll(ctx, deps); err != nil {
			t.Fatal(err)
		}
	}
	var done int
	s.Pool().QueryRow(ctx, `SELECT count(*) FROM nodes WHERE run_id=$1 AND status='done'`, run).Scan(&done)
	if done != 1 {
		t.Fatalf("config node should be done after ticks, got %d", done)
	}
}
