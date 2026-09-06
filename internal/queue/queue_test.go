package queue

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"myaudit/internal/store"
)

// Exercises the SQLite port's tricky bits: dependency gating via json_each,
// the atomic single-statement Claim, and "nothing ready → nil".
func TestClaimGatingAndComplete(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	q := New(s.DB())

	run, _ := s.CreateRun(ctx, "x")
	a, _ := s.AddNode(ctx, run, "understand", nil)
	b, _ := s.AddNode(ctx, run, "testgen", []uuid.UUID{a}) // depends on a

	// Only a (no deps) promotes; b is gated on a.
	if n, err := q.PromoteReady(ctx); err != nil || n != 1 {
		t.Fatalf("promote1: n=%d err=%v (want 1)", n, err)
	}
	c, err := q.Claim(ctx)
	if err != nil || c == nil || c.ID != a || c.Type != "understand" {
		t.Fatalf("claim a: %+v err=%v", c, err)
	}
	// a is running, b still gated → nothing to claim.
	if c2, err := q.Claim(ctx); err != nil || c2 != nil {
		t.Fatalf("claim2 should be nil, got %+v err=%v", c2, err)
	}
	// Completing a unblocks b.
	if err := q.Complete(ctx, a, []byte(`{"summary":"ok"}`)); err != nil {
		t.Fatal(err)
	}
	if n, err := q.PromoteReady(ctx); err != nil || n != 1 {
		t.Fatalf("promote2: n=%d err=%v (want 1)", n, err)
	}
	c3, err := q.Claim(ctx)
	if err != nil || c3 == nil || c3.ID != b {
		t.Fatalf("claim b: %+v err=%v", c3, err)
	}
}

// RecoverStuck requeues 'running' nodes under the attempt cap and fails poison
// ones — so a crash/restart self-heals instead of wedging the run forever.
func TestRecoverStuck(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	q := New(s.DB())
	run, _ := s.CreateRun(ctx, "x")

	stuck, _ := s.AddNode(ctx, run, "qa", nil)   // attempts 1 → requeue
	poison, _ := s.AddNode(ctx, run, "bug", nil) // attempts 3 → fail
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='running', attempts=1 WHERE id=?`, stuck)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='running', attempts=3 WHERE id=?`, poison)

	n, err := q.RecoverStuck(ctx, 3)
	if err != nil || n != 1 {
		t.Fatalf("RecoverStuck: n=%d err=%v (want 1 requeued)", n, err)
	}
	if got, _ := s.GetNode(ctx, stuck); got.Status != "ready" {
		t.Fatalf("stuck node should be requeued ready, got %s", got.Status)
	}
	if got, _ := s.GetNode(ctx, poison); got.Status != "failed" {
		t.Fatalf("poison node (>= maxAttempts) should fail, got %s", got.Status)
	}
}
