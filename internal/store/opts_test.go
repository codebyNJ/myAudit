package store

import (
	"context"
	"testing"
)

func TestRunOptsAndPauseOverBudget(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()

	run, ids, err := s.CreateGraph(ctx, "budgeted", []TaskSpec{
		{Key: "import", Type: "import", Spec: map[string]any{
			"repo_path": "/tmp/x", "audit_only": true, "budget_usd": 1.0,
		}},
		{Key: "map", Type: "map", DepKeys: []string{"import"}},
		{Key: "qa", Type: "qa", DepKeys: []string{"map"}, Spec: map[string]any{"title": "QA"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	opts := s.RunOptsFor(ctx, run)
	if !opts.AuditOnly || opts.BudgetUSD != 1.0 {
		t.Fatalf("opts=%+v", opts)
	}

	s.DB().ExecContext(ctx, `UPDATE nodes SET status='done', output=? WHERE id=?`, `{"cost_usd":0.6}`, ids["import"])
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='done', output=? WHERE id=?`, `{"cost_usd":0.5}`, ids["map"])
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, ids["qa"])

	hit, err := s.PauseOverBudget(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hit) != 1 || hit[0] != run {
		t.Fatalf("paused=%v", hit)
	}
	n, _ := s.GetNode(ctx, ids["qa"])
	if n.Status != "paused" {
		t.Fatalf("qa status=%s", n.Status)
	}
	r, _ := s.GetRun(ctx, run)
	if r.Status != "paused" {
		t.Fatalf("run status=%s", r.Status)
	}
}
