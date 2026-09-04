package queue

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestPromoteReadyRespectsDeps(t *testing.T) {
	ctx := context.Background()
	p := pool(t)
	defer p.Close()
	reset(t, p)
	run := uuid.New()
	p.Exec(ctx, `INSERT INTO runs(id,project) VALUES($1,'t')`, run)
	var a, b uuid.UUID
	p.QueryRow(ctx, `INSERT INTO nodes(run_id,type,status) VALUES($1,'a','done') RETURNING id`, run).Scan(&a)
	p.QueryRow(ctx, `INSERT INTO nodes(run_id,type,status,deps) VALUES($1,'b','pending',$2) RETURNING id`, run, []uuid.UUID{a}).Scan(&b)
	q := New(p)
	n, err := q.PromoteReady(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatal("b should be promoted")
	}
	var st string
	p.QueryRow(ctx, `SELECT status FROM nodes WHERE id=$1`, b).Scan(&st)
	if st != "ready" {
		t.Fatalf("status=%s", st)
	}
}

func TestPromoteReadyHoldsBlockedNode(t *testing.T) {
	ctx := context.Background()
	p := pool(t)
	defer p.Close()
	reset(t, p)
	run := uuid.New()
	p.Exec(ctx, `INSERT INTO runs(id,project) VALUES($1,'t')`, run)
	var a, b uuid.UUID
	p.QueryRow(ctx, `INSERT INTO nodes(run_id,type,status) VALUES($1,'a','running') RETURNING id`, run).Scan(&a)
	p.QueryRow(ctx, `INSERT INTO nodes(run_id,type,status,deps) VALUES($1,'b','pending',$2) RETURNING id`, run, []uuid.UUID{a}).Scan(&b)
	New(p).PromoteReady(ctx)
	var st string
	p.QueryRow(ctx, `SELECT status FROM nodes WHERE id=$1`, b).Scan(&st)
	if st != "pending" {
		t.Fatalf("dep not done -> should stay pending, got %s", st)
	}
}
