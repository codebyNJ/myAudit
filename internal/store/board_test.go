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
	// output with a summary but NO files key (the case that broke jsonb_array_length)
	s.Pool().Exec(ctx, `UPDATE nodes SET status='done', output='{"summary":"did a thing"}' WHERE id=$1`, nid)

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
