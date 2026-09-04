package events

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func pool(t *testing.T) *pgxpool.Pool {
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		t.Skip("no db")
	}
	p, _ := pgxpool.New(context.Background(), u)
	return p
}

func TestLogWritesRow(t *testing.T) {
	ctx := context.Background()
	p := pool(t)
	defer p.Close()
	run := uuid.New()
	if _, err := p.Exec(ctx, `INSERT INTO runs(id,project) VALUES($1,'t')`, run); err != nil {
		t.Fatal(err)
	}
	l := New(p)
	l.Log(ctx, Event{RunID: run, Kind: "node.start", Level: "info", Msg: "hi"})
	var n int
	p.QueryRow(ctx, `SELECT count(*) FROM events WHERE run_id=$1 AND kind='node.start'`, run).Scan(&n)
	if n != 1 {
		t.Fatalf("rows=%d", n)
	}
}
