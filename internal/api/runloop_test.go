package api

import (
	"context"
	"testing"

	"github.com/codebyNJ/myAudit/internal/store"
)

func TestResolveModel(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()

	t.Setenv("CLAUDE_MODEL", "")
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

func TestNewRealDepsReadsClaudeBinFromEnv(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	t.Setenv("CLAUDE_BIN", "")
	deps := NewRealDeps(s)
	ra, ok := deps.Agent.(realAgent)
	if !ok {
		t.Fatalf("expected realAgent, got %T", deps.Agent)
	}
	if ra.claudeBin != "" {
		t.Fatalf("unset CLAUDE_BIN should leave claudeBin empty, got %q", ra.claudeBin)
	}

	t.Setenv("CLAUDE_BIN", "/opt/claude/bin/claude")
	deps = NewRealDeps(s)
	ra, ok = deps.Agent.(realAgent)
	if !ok {
		t.Fatalf("expected realAgent, got %T", deps.Agent)
	}
	if ra.claudeBin != "/opt/claude/bin/claude" {
		t.Fatalf("CLAUDE_BIN should be read into realAgent.claudeBin, got %q", ra.claudeBin)
	}
}

func TestNewRealDepsReadsMaxConcurrentFromEnv(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	t.Setenv("MAX_CONCURRENT_CLAUDE", "")
	deps := NewRealDeps(s)
	if deps.MaxConcurrent != 1 {
		t.Fatalf("unset MAX_CONCURRENT_CLAUDE should default to 1 (serial), got %d", deps.MaxConcurrent)
	}

	t.Setenv("MAX_CONCURRENT_CLAUDE", "3")
	deps = NewRealDeps(s)
	if deps.MaxConcurrent != 3 {
		t.Fatalf("MAX_CONCURRENT_CLAUDE=3 should set MaxConcurrent=3, got %d", deps.MaxConcurrent)
	}
}

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

func TestTickAllClaimsAndDispatchesUpToMaxConcurrent(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")
	for i := 0; i < 3; i++ {
		id, _ := s.AddNode(ctx, run, "qa", nil)
		s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, id)
	}

	deps := NewStubDeps(s)
	deps.MaxConcurrent = 2

	n, err := TickAll(ctx, deps)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected TickAll to report 2 nodes processed, got %d", n)
	}

	var doneCount int
	s.DB().QueryRowContext(ctx, `SELECT count(*) FROM nodes WHERE run_id=? AND status='done'`, run).Scan(&doneCount)
	if doneCount != 2 {
		t.Fatalf("expected 2 nodes marked done after one tick, got %d", doneCount)
	}
}

func TestTickAllChecksBudgetBeforeClaiming(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()

	_, ids, err := s.CreateGraph(ctx, "budgeted", []store.TaskSpec{
		{Key: "import", Type: "import", Spec: map[string]any{
			"repo_path": "/tmp/x", "budget_usd": 1.0,
		}},
		{Key: "map", Type: "map", DepKeys: []string{"import"}},
		{Key: "qa", Type: "qa", DepKeys: []string{"map"}, Spec: map[string]any{"title": "QA"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	s.DB().ExecContext(ctx, `UPDATE nodes SET status='done', output=? WHERE id=?`, `{"cost_usd":0.6}`, ids["import"])
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='done', output=? WHERE id=?`, `{"cost_usd":0.5}`, ids["map"])
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, ids["qa"])

	deps := NewStubDeps(s)
	deps.MaxConcurrent = 2

	n, err := TickAll(ctx, deps)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected 0 nodes processed (budget already exceeded), got %d", n)
	}

	qaNode, err := s.GetNode(ctx, ids["qa"])
	if err != nil {
		t.Fatal(err)
	}
	if qaNode.Status != "paused" {
		t.Fatalf("expected qa node parked 'paused' by budget check before claiming, got %q", qaNode.Status)
	}
}
