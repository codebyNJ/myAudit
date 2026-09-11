package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type RunOpts struct {
	RepoPath  string  `json:"repo_path"`
	AuditOnly bool    `json:"audit_only"`
	BudgetUSD float64 `json:"budget_usd"`
}

func (s *Store) RunOptsFor(ctx context.Context, run uuid.UUID) RunOpts {
	var raw []byte
	_ = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(input_snapshot,'{}') FROM nodes WHERE run_id=? AND type='import' ORDER BY created_at LIMIT 1`,
		run).Scan(&raw)
	var o RunOpts
	_ = json.Unmarshal(raw, &o)
	return o
}

func (s *Store) PauseOverBudget(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id,
		       COALESCE(json_extract(i.input_snapshot,'$.budget_usd'), 0) AS budget,
		       COALESCE((SELECT sum(COALESCE(json_extract(n.output,'$.cost_usd'),0)) FROM nodes n WHERE n.run_id=r.id), 0) AS spent
		FROM runs r
		JOIN nodes i ON i.run_id=r.id AND i.type='import'
		WHERE r.status='running'
		  AND COALESCE(json_extract(i.input_snapshot,'$.budget_usd'), 0) > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hit []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		var budget, spent float64
		if err := rows.Scan(&id, &budget, &spent); err != nil {
			return nil, err
		}
		if spent >= budget {
			hit = append(hit, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range hit {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE nodes SET status='paused' WHERE run_id=? AND status IN ('ready','pending') AND type IN ('qa','bug','flows')`, id); err != nil {
			return nil, err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE runs SET status='paused' WHERE id=? AND status='running'`, id); err != nil {
			return nil, err
		}
	}
	return hit, nil
}
