package queue

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

// reset gives each queue test an exclusive clean slate. Queue tests assert on
// the global ready-set (Claim pops the oldest ready node across all runs), so
// they must not see other tests' rows. Run the DB suite with `go test -p 1`.
func reset(t *testing.T, p *pgxpool.Pool) {
	if _, err := p.Exec(context.Background(), `TRUNCATE runs, nodes, events RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
}

func TestClaimReturnsReadyNodeOnce(t *testing.T) {
	ctx := context.Background()
	p := pool(t)
	defer p.Close()
	reset(t, p)
	run := uuid.New()
	p.Exec(ctx, `INSERT INTO runs(id,project) VALUES($1,'t')`, run)
	var nid uuid.UUID
	p.QueryRow(ctx, `INSERT INTO nodes(run_id,type,status) VALUES($1,'implement','ready') RETURNING id`, run).Scan(&nid)
	q := New(p)
	c1, err := q.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c1 == nil || c1.ID != nid {
		t.Fatal("first claim should get the ready node")
	}
	c2, err := q.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c2 != nil {
		t.Fatal("second claim should be empty (node already running)")
	}
}
