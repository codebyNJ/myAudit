package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type Node struct {
	ID     uuid.UUID   `json:"id"`
	RunID  uuid.UUID   `json:"run_id"`
	Type   string      `json:"type"`
	Status string      `json:"status"`
	Deps   []uuid.UUID `json:"deps"`
}

func (s *Store) CreateRun(ctx context.Context, project string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.db.ExecContext(ctx, `INSERT INTO runs(id, project) VALUES(?,?)`, id, project)
	return id, err
}

func (s *Store) CancelRun(ctx context.Context, run uuid.UUID) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE nodes SET status='cancelled' WHERE run_id=? AND status IN ('pending','ready')`, run); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE runs SET status='cancelled' WHERE id=?`, run)
	return err
}

func (s *Store) FinalizeDrainedRuns(ctx context.Context) ([]RunSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.project, r.created_at FROM runs r
		WHERE r.status='running'
		  AND EXISTS (SELECT 1 FROM nodes n WHERE n.run_id=r.id)
		  AND NOT EXISTS (SELECT 1 FROM nodes n WHERE n.run_id=r.id
		                  AND n.status IN ('pending','ready','running'))`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var drained []RunSummary
	for rows.Next() {
		var r RunSummary
		if err := rows.Scan(&r.ID, &r.Project, &r.CreatedAt); err != nil {
			return nil, err
		}
		drained = append(drained, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range drained {
		var rootFail int
		_ = s.db.QueryRowContext(ctx,
			`SELECT count(*) FROM nodes WHERE run_id=? AND type IN ('import','map') AND status='failed'`,
			drained[i].ID).Scan(&rootFail)
		drained[i].Status = "done"
		if rootFail > 0 {
			drained[i].Status = "failed"
		}
		if _, err := s.db.ExecContext(ctx,
			`UPDATE runs SET status=? WHERE id=? AND status='running'`,
			drained[i].Status, drained[i].ID); err != nil {
			return nil, err
		}
	}
	return drained, nil
}

func (s *Store) AddNode(ctx context.Context, run uuid.UUID, typ string, deps []uuid.UUID) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nodes(id, run_id, type, deps) VALUES(?,?,?,?)`,
		id, run, typ, marshalIDs(deps))
	return id, err
}

func (s *Store) AddNodeFull(ctx context.Context, run uuid.UUID, typ string, deps []uuid.UUID, spec any, status string) (uuid.UUID, error) {
	snap := "{}"
	if spec != nil {
		b, err := json.Marshal(spec)
		if err != nil {
			return uuid.Nil, err
		}
		snap = string(b)
	}
	if status == "" {
		status = "pending"
	}
	id := uuid.New()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nodes(id, run_id, type, status, deps, input_snapshot) VALUES(?,?,?,?,?,?)`,
		id, run, typ, status, marshalIDs(deps), snap)
	return id, err
}

func (s *Store) GetNode(ctx context.Context, id uuid.UUID) (Node, error) {
	var n Node
	var deps string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, run_id, type, status, deps FROM nodes WHERE id=?`, id).
		Scan(&n.ID, &n.RunID, &n.Type, &n.Status, &deps)
	n.Deps = scanIDs(deps)
	return n, err
}
