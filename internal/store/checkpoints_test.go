package store

import (
	"context"
	"testing"
)

func TestRaiseAndResolveCheckpoint(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()

	run, _ := s.CreateRun(ctx, "acme")
	nid, _ := s.AddNode(ctx, run, "implement", nil)
	s.Pool().Exec(ctx, `UPDATE nodes SET status='running' WHERE id=$1`, nid)

	cpID, err := s.RaiseCheckpoint(ctx, run, nid, "multi or single?", []string{"multi", "single"})
	if err != nil {
		t.Fatal(err)
	}
	n, _ := s.GetNode(ctx, nid)
	if n.Status != "blocked" {
		t.Fatalf("raising a checkpoint should block the node, got %s", n.Status)
	}
	cp, ok, err := s.OpenCheckpointForNode(ctx, nid)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || cp.Question != "multi or single?" {
		t.Fatalf("open checkpoint not found: %+v ok=%v", cp, ok)
	}

	if err := s.ResolveCheckpoint(ctx, cpID, "multi"); err != nil {
		t.Fatal(err)
	}
	n, _ = s.GetNode(ctx, nid)
	if n.Status != "ready" {
		t.Fatalf("resolving should requeue node to ready, got %s", n.Status)
	}
	if _, ok, _ := s.OpenCheckpointForNode(ctx, nid); ok {
		t.Fatal("checkpoint should be resolved (no open checkpoint)")
	}
}
