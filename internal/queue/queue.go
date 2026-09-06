package queue

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// ClaimedNode is a node a worker has exclusively taken to run.
type ClaimedNode struct {
	ID, RunID uuid.UUID
	Type      string
	Attempts  int
	Spec      []byte // nodes.input_snapshot — per-node data
}

// Queue is the SQLite-backed job queue. A job's state is a row, so the whole
// system is queryable at any instant.
type Queue struct {
	db *sql.DB
}

// New builds a Queue over the DB handle.
func New(db *sql.DB) *Queue { return &Queue{db: db} }

// Claim atomically takes one ready node and marks it running. Returns nil when
// none is ready.
//
// ponytail: a single UPDATE ... WHERE id=(SELECT ... LIMIT 1) RETURNING is
// atomic under SQLite's statement-level write lock — no FOR UPDATE SKIP LOCKED
// needed. Fine for a local single-process run loop; revisit if we ever run
// multiple worker processes against one file.
//
// Claim order encodes "QA over dev": import/map first, then qa (find everything),
// then bug (dev fixes) last, tie-broken by created_at. So the board drains all
// discovery before any fix begins.
func (q *Queue) Claim(ctx context.Context) (*ClaimedNode, error) {
	var c ClaimedNode
	var spec sql.NullString
	err := q.db.QueryRowContext(ctx, `
		UPDATE nodes SET status='running', claimed_at=CURRENT_TIMESTAMP, attempts=attempts+1
		WHERE id = (SELECT id FROM nodes WHERE status='ready'
			ORDER BY CASE type WHEN 'import' THEN 0 WHEN 'map' THEN 1 WHEN 'qa' THEN 2 ELSE 3 END, created_at
			LIMIT 1)
		RETURNING id, run_id, type, attempts, COALESCE(input_snapshot,'{}')`).
		Scan(&c.ID, &c.RunID, &c.Type, &c.Attempts, &spec)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.Spec = []byte(spec.String)
	if !spec.Valid || spec.String == "" {
		c.Spec = []byte("{}")
	}
	return &c, nil
}

// Complete marks a node done and stores its output.
func (q *Queue) Complete(ctx context.Context, id uuid.UUID, output []byte) error {
	return q.Finish(ctx, id, output, "done")
}

// Finish stores a node's output and sets its terminal status in ONE write, so a
// non-done outcome (failed / in_review) can't be lost by a fire-and-forget
// follow-up update — the bug handler relies on this for its "never a false
// fixed" guarantee.
func (q *Queue) Finish(ctx context.Context, id uuid.UUID, output []byte, status string) error {
	_, err := q.db.ExecContext(ctx, `UPDATE nodes SET status=?, output=? WHERE id=?`, status, string(output), id)
	return err
}

// Fail marks a node failed.
func (q *Queue) Fail(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, `UPDATE nodes SET status='failed' WHERE id=?`, id)
	return err
}

// Requeue puts a running node back to ready for another attempt (retry).
func (q *Queue) Requeue(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, id)
	return err
}

// PromoteReady moves pending nodes whose every dependency is done to ready.
// Returns the number promoted.
func (q *Queue) PromoteReady(ctx context.Context) (int, error) {
	res, err := q.db.ExecContext(ctx, `
		UPDATE nodes SET status='ready'
		WHERE status='pending'
		  AND NOT EXISTS (
			SELECT 1 FROM json_each(nodes.deps) AS d
			JOIN nodes dn ON dn.id = d.value
			WHERE dn.status <> 'done'
		  )`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
