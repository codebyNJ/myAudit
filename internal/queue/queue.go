package queue

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

type ClaimedNode struct {
	ID, RunID uuid.UUID
	Type      string
	Attempts  int
	Spec      []byte
}

type Queue struct {
	db *sql.DB
}

func New(db *sql.DB) *Queue { return &Queue{db: db} }

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

func (q *Queue) ClaimN(ctx context.Context, n int) ([]*ClaimedNode, error) {
	var out []*ClaimedNode
	for i := 0; i < n; i++ {
		c, err := q.Claim(ctx)
		if err != nil {
			return out, err
		}
		if c == nil {
			break
		}
		out = append(out, c)
	}
	return out, nil
}

func (q *Queue) Complete(ctx context.Context, id uuid.UUID, output []byte) error {
	return q.Finish(ctx, id, output, "done")
}

func (q *Queue) Finish(ctx context.Context, id uuid.UUID, output []byte, status string) error {
	_, err := q.db.ExecContext(ctx, `UPDATE nodes SET status=?, output=? WHERE id=?`, status, string(output), id)
	return err
}

func (q *Queue) Fail(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, `UPDATE nodes SET status='failed' WHERE id=?`, id)
	return err
}

func (q *Queue) Requeue(ctx context.Context, id uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, id)
	return err
}

func (q *Queue) RecoverStuck(ctx context.Context, maxAttempts int) (int, error) {
	if _, err := q.db.ExecContext(ctx,
		`UPDATE nodes SET status='failed' WHERE status='running' AND attempts >= ?`, maxAttempts); err != nil {
		return 0, err
	}
	res, err := q.db.ExecContext(ctx,
		`UPDATE nodes SET status='ready' WHERE status='running' AND attempts < ?`, maxAttempts)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (q *Queue) PromoteReady(ctx context.Context) (int, error) {

	res, err := q.db.ExecContext(ctx, `
		UPDATE nodes SET status='ready'
		WHERE status='pending'
		  AND NOT EXISTS (
			SELECT 1 FROM json_each(nodes.deps) AS d
			JOIN nodes dn ON dn.id = d.value
			WHERE dn.status IN ('pending','ready','running')
		  )`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
