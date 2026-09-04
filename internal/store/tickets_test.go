package store

import (
	"context"
	"testing"
)

func TestCreateBugShowsOnBoardWithTags(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")

	bid, err := s.CreateBug(ctx, run, Bug{
		Title: "Null deref in login", File: "auth.go:42", Severity: "high", Priority: "P0",
		Detail: "x may be nil", Tags: []string{"from:review", "high"},
	})
	if err != nil {
		t.Fatal(err)
	}

	cards, err := s.NodeDetailsForRun(ctx, run)
	if err != nil || len(cards) != 1 {
		t.Fatalf("want 1 card, got %d err=%v", len(cards), err)
	}
	c := cards[0]
	if c.Type != "bug" || c.Status != "open" {
		t.Fatalf("bug should be type=bug status=open: %+v", c)
	}
	if c.Title != "Null deref in login" || c.Severity != "high" || c.Priority != "P0" {
		t.Fatalf("ticket fields not surfaced: %+v", c)
	}
	if len(c.Tags) != 2 || c.Tags[0] != "from:review" {
		t.Fatalf("tags not surfaced: %v", c.Tags)
	}

	// A bug ticket must never be claimable by the worker queue.
	var n int
	s.db.QueryRowContext(ctx, `SELECT count(*) FROM nodes WHERE id=? AND status='ready'`, bid).Scan(&n)
	if n != 0 {
		t.Fatal("bug ticket should not be in a claimable state")
	}
}

func TestSetNodeTagsRoundtrip(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")
	nid, _ := s.AddNode(ctx, run, "understand", nil)

	if err := s.SetNodeTags(ctx, nid, []string{"needs-triage", "P1"}); err != nil {
		t.Fatal(err)
	}
	cards, _ := s.NodeDetailsForRun(ctx, run)
	if len(cards) != 1 || len(cards[0].Tags) != 2 || cards[0].Tags[1] != "P1" {
		t.Fatalf("tags: %+v", cards)
	}
}
