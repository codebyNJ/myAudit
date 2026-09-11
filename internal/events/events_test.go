package events

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/codebyNJ/myAudit/internal/store"
)

func TestLogWritesRow(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(t.TempDir(), "e.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	run, _ := s.CreateRun(ctx, "t")

	New(s.DB()).Log(ctx, Event{RunID: run, Kind: "node.start", Level: "info", Msg: "hi"})

	var n int
	s.DB().QueryRowContext(ctx, `SELECT count(*) FROM events WHERE run_id=? AND kind='node.start'`, run).Scan(&n)
	if n != 1 {
		t.Fatalf("rows=%d", n)
	}
}
