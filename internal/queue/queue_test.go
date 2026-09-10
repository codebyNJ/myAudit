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

// A dependency that ends FAILED still unblocks its dependents — a failed node
// must not strand the rest of the graph in 'pending' forever.
func TestPromoteReadyUnblocksOnFailedDep(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	q := New(s.DB())
	run, _ := s.CreateRun(ctx, "x")
	a, _ := s.AddNode(ctx, run, "qa", nil)
	b, _ := s.AddNode(ctx, run, "bug", []uuid.UUID{a})
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='failed' WHERE id=?`, a)

	if _, err := q.PromoteReady(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetNode(ctx, b); got.Status != "ready" {
		t.Fatalf("dependent should promote once its dep is terminal (failed), got %s", got.Status)
	}
}

// ClaimN returns up to n ready nodes in one batch, capped by however many are
// actually ready — bounded concurrency needs to grab a batch, not one at a time.
func TestClaimNReturnsUpToNReadyNodes(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	q := New(s.DB())
	run, _ := s.CreateRun(ctx, "proj")

	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		id, _ := s.AddNode(ctx, run, "qa", nil)
		s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, id)
		ids = append(ids, id)
	}

	claimed, err := q.ClaimN(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 2 {
		t.Fatalf("expected 2 claimed nodes (capped by n=2), got %d", len(claimed))
	}
	for _, c := range claimed {
		var status string
		s.DB().QueryRowContext(ctx, `SELECT status FROM nodes WHERE id=?`, c.ID).Scan(&status)
		if status != "running" {
			t.Fatalf("claimed node should be marked running, got %s", status)
		}
	}

	// third node should still be ready, untouched
	remaining, err := q.ClaimN(ctx, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected exactly 1 remaining ready node, got %d", len(remaining))
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
