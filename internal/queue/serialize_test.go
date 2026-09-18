package queue

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/codebyNJ/myAudit/internal/store"
)

func newQueue(t *testing.T) (*store.Store, *Queue) {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, New(s.DB())
}

// Every node of a run shares one git workspace, and a bug node commits with
// `git add -A`. Two bug nodes running at once therefore commit each other's
// edits, so the second must wait.
func TestClaimSerializesBugNodesWithinARun(t *testing.T) {
	ctx := context.Background()
	s, q := newQueue(t)
	run, _ := s.CreateRun(ctx, "x")
	if _, err := s.AddNodeFull(ctx, run, "bug", nil, nil, "ready"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNodeFull(ctx, run, "bug", nil, nil, "ready"); err != nil {
		t.Fatal(err)
	}

	first, err := q.Claim(ctx)
	if err != nil || first == nil {
		t.Fatalf("first bug should be claimable: %+v err=%v", first, err)
	}
	second, err := q.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if second != nil {
		t.Fatal("a second bug node must not run while the first holds the workspace")
	}

	if err := q.Complete(ctx, first.ID, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if next, err := q.Claim(ctx); err != nil || next == nil {
		t.Fatalf("the queued bug should run once the first finished: %+v err=%v", next, err)
	}
}

// A qa node writing test files would also be swept into a fix commit.
func TestClaimHoldsBugWhileQaRuns(t *testing.T) {
	ctx := context.Background()
	s, q := newQueue(t)
	run, _ := s.CreateRun(ctx, "x")
	if _, err := s.AddNodeFull(ctx, run, "qa", nil, nil, "ready"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddNodeFull(ctx, run, "bug", nil, nil, "ready"); err != nil {
		t.Fatal(err)
	}

	qa, err := q.Claim(ctx) // qa sorts first
	if err != nil || qa == nil || qa.Type != "qa" {
		t.Fatalf("expected the qa node first: %+v err=%v", qa, err)
	}
	if c, err := q.Claim(ctx); err != nil || c != nil {
		t.Fatalf("bug must wait for the qa node: %+v err=%v", c, err)
	}
	if err := q.Complete(ctx, qa.ID, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if c, err := q.Claim(ctx); err != nil || c == nil || c.Type != "bug" {
		t.Fatalf("bug should run after qa finished: %+v err=%v", c, err)
	}
}

// Serialization is per-run; parallelism across runs must survive.
func TestClaimStillParallelAcrossRuns(t *testing.T) {
	ctx := context.Background()
	s, q := newQueue(t)
	for _, name := range []string{"a", "b"} {
		run, _ := s.CreateRun(ctx, name)
		if _, err := s.AddNodeFull(ctx, run, "bug", nil, nil, "ready"); err != nil {
			t.Fatal(err)
		}
	}
	got, err := q.ClaimN(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("bug nodes in different runs should run together, claimed %d", len(got))
	}
}

// Budget pause has to hold: a paused run's work stays parked.
func TestPausedRunIsNotPromotedOrClaimed(t *testing.T) {
	ctx := context.Background()
	s, q := newQueue(t)
	run, _ := s.CreateRun(ctx, "x")
	if _, err := s.AddNodeFull(ctx, run, "bug", nil, nil, "pending"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().ExecContext(ctx, `UPDATE runs SET status='paused' WHERE id=?`, run); err != nil {
		t.Fatal(err)
	}

	if n, err := q.PromoteReady(ctx); err != nil || n != 0 {
		t.Fatalf("paused run must not promote work: n=%d err=%v", n, err)
	}
	if c, err := q.Claim(ctx); err != nil || c != nil {
		t.Fatalf("paused run must not be claimed: %+v err=%v", c, err)
	}
}
