package api

import (
	"context"
	"testing"

	"myaudit/internal/store"
)

// resolveModel precedence: CLAUDE_MODEL env override > Settings tier > default.
func TestResolveModel(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()

	t.Setenv("CLAUDE_MODEL", "") // no explicit override
	if m := resolveModel(ctx, s); m != defaultModel {
		t.Fatalf("default should be %s, got %s", defaultModel, m)
	}
	if _, err := s.PutSettings(ctx, map[string]any{"model_tier": "sonnet-5"}); err != nil {
		t.Fatal(err)
	}
	if m := resolveModel(ctx, s); m != "claude-sonnet-5" {
		t.Fatalf("settings tier should apply when no env, got %s", m)
	}
	t.Setenv("CLAUDE_MODEL", "custom-x")
	if m := resolveModel(ctx, s); m != "custom-x" {
		t.Fatalf("env must override settings, got %s", m)
	}
}

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
