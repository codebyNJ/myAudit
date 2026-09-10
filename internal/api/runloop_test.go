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

// NewRealDeps should read CLAUDE_BIN so a user whose claude install isn't on
// PATH (or who wants a wrapper script) can point myAudit at it explicitly.
func TestNewRealDepsReadsClaudeBinFromEnv(t *testing.T) {
	s := newStore(t)
	defer s.Close()

	t.Setenv("CLAUDE_BIN", "")
	deps := NewRealDeps(s)
	ra, ok := deps.Agent.(realAgent)
	if !ok {
		t.Fatalf("expected realAgent, got %T", deps.Agent)
	}
	if ra.bin != "" {
		t.Fatalf("unset CLAUDE_BIN should leave bin empty (agent.Options defaults it to \"claude\"), got %q", ra.bin)
	}

	t.Setenv("CLAUDE_BIN", "/opt/claude/bin/claude")
	deps = NewRealDeps(s)
	ra, ok = deps.Agent.(realAgent)
	if !ok {
		t.Fatalf("expected realAgent, got %T", deps.Agent)
	}
	if ra.bin != "/opt/claude/bin/claude" {
		t.Fatalf("CLAUDE_BIN should be read into realAgent.bin, got %q", ra.bin)
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
