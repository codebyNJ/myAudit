package store

import (
	"context"
	"strings"
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

	var n int
	s.db.QueryRowContext(ctx, `SELECT count(*) FROM nodes WHERE id=? AND status='ready'`, bid).Scan(&n)
	if n != 0 {
		t.Fatal("bug ticket should not be in a claimable state")
	}
}

func TestCreateBugDedups(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")

	b := Bug{Title: "Missing rel=noreferrer", File: "TopBar.tsx:34", Severity: "low"}
	id1, _ := s.CreateBug(ctx, run, b)
	id2, _ := s.CreateBug(ctx, run, b)
	if id1 != id2 {
		t.Fatalf("duplicate finding should return the same ticket: %s vs %s", id1, id2)
	}

	id3, _ := s.CreateBug(ctx, run, Bug{Title: "Missing rel=noreferrer", File: "PoweredBy.tsx:5", Severity: "low"})
	if id3 == id1 {
		t.Fatal("different file should be a distinct ticket")
	}
	cards, _ := s.NodeDetailsForRun(ctx, run)
	bugs := 0
	for _, c := range cards {
		if c.Type == "bug" {
			bugs++
		}
	}
	if bugs != 2 {
		t.Fatalf("want 2 tickets after dedup, got %d", bugs)
	}
}

func TestCreateBugPersistsCategoryAndConfidence(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(ctx, testURL(t))
	defer s.Close()
	run, _ := s.CreateRun(ctx, "proj")

	id, err := s.CreateBug(ctx, run, Bug{
		Title: "SQL injection", Severity: "high", Priority: "P0",
		Category: "security", Confidence: "high",
	})
	if err != nil {
		t.Fatal(err)
	}

	var raw string
	s.db.QueryRowContext(ctx, `SELECT input_snapshot FROM nodes WHERE id=?`, id).Scan(&raw)
	if !strings.Contains(raw, `"category":"security"`) || !strings.Contains(raw, `"confidence":"high"`) {
		t.Fatalf("category/confidence not persisted in input_snapshot: %s", raw)
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
