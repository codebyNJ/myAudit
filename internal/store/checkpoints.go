package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Checkpoint is a human-in-the-loop interrupt raised by a node.
type Checkpoint struct {
	ID       uuid.UUID `json:"id"`
	RunID    uuid.UUID `json:"run_id"`
	NodeID   uuid.UUID `json:"node_id"`
	Question string    `json:"question"`
	Resolved bool      `json:"resolved"`
	Answer   string    `json:"answer"`
}

// RaiseCheckpoint records an interrupt and blocks the node until it is
// resolved. Returns the checkpoint id.
func (s *Store) RaiseCheckpoint(ctx context.Context, run, node uuid.UUID, question string, options []string) (uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	opts, _ := json.Marshal(options)
	var id uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO checkpoints(run_id, node_id, question, options) VALUES($1,$2,$3,$4) RETURNING id`,
		run, node, question, opts).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE nodes SET status='blocked' WHERE id=$1`, node); err != nil {
		return uuid.Nil, err
	}
	return id, tx.Commit(ctx)
}

// ResolveCheckpoint stores the answer and requeues the node to ready so it
// re-runs with the decision available.
func (s *Store) ResolveCheckpoint(ctx context.Context, checkpoint uuid.UUID, answer string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var node uuid.UUID
	if err := tx.QueryRow(ctx,
		`UPDATE checkpoints SET answer=$2, resolved=true, resolved_at=now() WHERE id=$1 RETURNING node_id`,
		checkpoint, answer).Scan(&node); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE nodes SET status='ready' WHERE id=$1`, node); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// OpenCheckpointForNode returns the unresolved checkpoint for a node, if any.
func (s *Store) OpenCheckpointForNode(ctx context.Context, node uuid.UUID) (Checkpoint, bool, error) {
	var cp Checkpoint
	err := s.pool.QueryRow(ctx,
		`SELECT id, run_id, node_id, question, resolved, COALESCE(answer,'')
		 FROM checkpoints WHERE node_id=$1 AND NOT resolved LIMIT 1`, node).
		Scan(&cp.ID, &cp.RunID, &cp.NodeID, &cp.Question, &cp.Resolved, &cp.Answer)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Checkpoint{}, false, nil
		}
		return Checkpoint{}, false, err
	}
	return cp, true, nil
}
