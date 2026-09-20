package store

import (
	"context"
	"testing"
)

func TestNodeDetailsHandlesOutputWithoutFiles(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	nid, _ := s.AddNode(ctx, run, "implement", nil)

	s.db.ExecContext(ctx, `UPDATE nodes SET status='done', output='{"summary":"did a thing"}' WHERE id=?`, nid)

	cards, err := s.NodeDetailsForRun(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Fatalf("want 1 card, got %d", len(cards))
	}
	c := cards[0]
	if c.Summary != "did a thing" || c.Files != 0 {
		t.Fatalf("detail: %+v", c)
	}
}

func TestNodeDetailsForRunSurfacesCategoryAndConfidence(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")
	s.CreateBug(ctx, run, Bug{Title: "X", Severity: "high", Priority: "P0",
		Category: "security", Confidence: "medium"})

	details, err := s.NodeDetailsForRun(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 node, got %d", len(details))
	}
	if details[0].Category != "security" || details[0].Confidence != "medium" {
		t.Fatalf("category/confidence not surfaced: %+v", details[0])
	}
}

// The branch is written to the node output when the fix commits, and was
// previously dropped here — the value existed in the database but no column of
// the query selected it, so the UI could never show it.
func TestNodeDetailsForRunSurfacesBranch(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")
	nid, _ := s.AddNode(ctx, run, "bug", nil)

	if err := s.MergeNodeOutput(ctx, nid, map[string]any{
		"commit_sha": "a3f9c21deadbeef",
		"branch":     "myaudit/fcc9ddf7/9b0fe6e7",
		"pr_url":     "https://example.test/pr/1",
	}); err != nil {
		t.Fatal(err)
	}

	details, err := s.NodeDetailsForRun(ctx, run)
	if err != nil {
		t.Fatal(err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 node, got %d", len(details))
	}
	d := details[0]
	if d.Branch != "myaudit/fcc9ddf7/9b0fe6e7" {
		t.Errorf("Branch = %q, want the branch from the node output", d.Branch)
	}
	// The neighbouring columns must still line up after inserting one.
	if d.CommitSHA != "a3f9c21deadbeef" || d.PRURL != "https://example.test/pr/1" {
		t.Errorf("scan positions drifted: commit=%q pr=%q", d.CommitSHA, d.PRURL)
	}
}

// A ticket with no git work at all must read as empty, not as a stray value
// picked up from an adjacent column.
func TestNodeDetailsForRunBranchEmptyWithoutGitWork(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")
	s.CreateBug(ctx, run, Bug{Title: "not fixed yet", Severity: "low", Priority: "P2"})

	details, _ := s.NodeDetailsForRun(ctx, run)
	if len(details) != 1 {
		t.Fatalf("expected 1 node, got %d", len(details))
	}
	if details[0].Branch != "" || details[0].CommitSHA != "" {
		t.Fatalf("expected no git trail, got branch=%q commit=%q", details[0].Branch, details[0].CommitSHA)
	}
}
