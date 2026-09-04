package queue

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ClaimedNode is a node a worker has exclusively taken to run.
type ClaimedNode struct {
	ID, RunID uuid.UUID
	Type      string
	Attempts  int
	Spec      []byte // nodes.input_snapshot — per-node data (resource fields, config)
}

// Queue is the Postgres-backed job queue. It is also the state store — a job's
// state is a row, so the whole system is queryable at any instant.
type Queue struct {
	pool *pgxpool.Pool
}

// New builds a Queue over the pool.
func New(p *pgxpool.Pool) *Queue { return &Queue{pool: p} }

// Claim atomically takes one ready node (FOR UPDATE SKIP LOCKED so concurrent
// workers never collide) and marks it running. Returns nil when none is ready.
func (q *Queue) Claim(ctx context.Context) (*ClaimedNode, error) {
	row := q.pool.QueryRow(ctx, `
		UPDATE nodes SET status='running', claimed_at=now(), attempts=attempts+1
		WHERE id = (
			SELECT id FROM nodes WHERE status='ready'
			ORDER BY created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING id, run_id, type, attempts, COALESCE(input_snapshot::text,'{}')`)
	var c ClaimedNode
	var spec string
	err := row.Scan(&c.ID, &c.RunID, &c.Type, &c.Attempts, &spec)
	c.Spec = []byte(spec)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

// Complete marks a node done and stores its output.
func (q *Queue) Complete(ctx context.Context, id uuid.UUID, output []byte) error {
	_, err := q.pool.Exec(ctx, `UPDATE nodes SET status='done', output=$2 WHERE id=$1`, id, output)
	return err
}

// Fail marks a node failed.
func (q *Queue) Fail(ctx context.Context, id uuid.UUID) error {
	_, err := q.pool.Exec(ctx, `UPDATE nodes SET status='failed' WHERE id=$1`, id)
	return err
}

// Requeue puts a running node back to ready for another attempt (retry).
func (q *Queue) Requeue(ctx context.Context, id uuid.UUID) error {
	_, err := q.pool.Exec(ctx, `UPDATE nodes SET status='ready' WHERE id=$1`, id)
	return err
}

// PromoteReady moves pending nodes whose every dependency is done to ready.
// Returns the number promoted.
func (q *Queue) PromoteReady(ctx context.Context) (int, error) {
	tag, err := q.pool.Exec(ctx, `
		UPDATE nodes n SET status='ready'
		WHERE n.status='pending'
		  AND NOT EXISTS (
			SELECT 1 FROM unnest(n.deps) AS d
			JOIN nodes dn ON dn.id = d
			WHERE dn.status <> 'done'
		  )`)
	return int(tag.RowsAffected()), err
}
