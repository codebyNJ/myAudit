package store

import (
	"context"
	"strings"
	"testing"
)

func TestOpenAndPing(t *testing.T) {
	s, err := Open(context.Background(), testURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// The board counts events per card on every 2-second poll.
func TestEventCountPerNodeUsesAnIndex(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, testURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.db.QueryContext(ctx, `EXPLAIN QUERY PLAN SELECT count(*) FROM events e WHERE e.node_id = ?`, "x")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		plan.WriteString(detail + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.String(), "idx_events_node") {
		t.Fatalf("per-node event count scans the table:\n%s", plan.String())
	}
}
