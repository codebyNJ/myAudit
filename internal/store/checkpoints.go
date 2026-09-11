package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

type Checkpoint struct {
	ID       uuid.UUID `json:"id"`
	RunID    uuid.UUID `json:"run_id"`
	NodeID   uuid.UUID `json:"node_id"`
	Question string    `json:"question"`
	Resolved bool      `json:"resolved"`
	Answer   string    `json:"answer"`
}

func (s *Store) RaiseCheckpoint(ctx context.Context, run, node uuid.UUID, question string, options []string) (uuid.UUID, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback()

	opts, _ := json.Marshal(options)
	id := uuid.New()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO checkpoints(id, run_id, node_id, question, options) VALUES(?,?,?,?,?)`,
		id, run, node, question, string(opts)); err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE nodes SET status='blocked' WHERE id=?`, node); err != nil {
		return uuid.Nil, err
	}
	return id, tx.Commit()
}

func (s *Store) ResolveCheckpoint(ctx context.Context, checkpoint uuid.UUID, answer string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var node uuid.UUID
	if err := tx.QueryRowContext(ctx,
		`UPDATE checkpoints SET answer=?, resolved=1, resolved_at=CURRENT_TIMESTAMP WHERE id=? RETURNING node_id`,
		answer, checkpoint).Scan(&node); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, node); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) OpenCheckpointForNode(ctx context.Context, node uuid.UUID) (Checkpoint, bool, error) {
	var cp Checkpoint
	err := s.db.QueryRowContext(ctx,
		`SELECT id, run_id, node_id, question, resolved, COALESCE(answer,'')
		 FROM checkpoints WHERE node_id=? AND resolved=0 LIMIT 1`, node).
		Scan(&cp.ID, &cp.RunID, &cp.NodeID, &cp.Question, &cp.Resolved, &cp.Answer)
	if errors.Is(err, sql.ErrNoRows) {
		return Checkpoint{}, false, nil
	}
	if err != nil {
		return Checkpoint{}, false, err
	}
	return cp, true, nil
}
