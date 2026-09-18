package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type GitInfo struct {
	HasGit        bool   `json:"has_git"`
	RemoteURL     string `json:"remote_url,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	HeadSHA       string `json:"head_sha,omitempty"`
}

type RunOpts struct {
	RepoPath  string   `json:"repo_path"`
	AuditOnly bool     `json:"audit_only"`
	BudgetUSD float64  `json:"budget_usd"`
	Git       *GitInfo `json:"git,omitempty"`
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

// PauseOverBudget parks work on runs that have spent their budget and returns
// the runs newly paused by this call (for logging). Already-paused runs are
// re-checked rather than skipped: a qa node that was still running when the
// pause landed can file new bug tickets afterwards, and without a re-check
// those tickets would be promoted and billed past the cap.
func (s *Store) PauseOverBudget(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.status,
		       COALESCE(json_extract(i.input_snapshot,'$.budget_usd'), 0) AS budget,
		       COALESCE((SELECT sum(COALESCE(json_extract(n.output,'$.cost_usd'),0)) FROM nodes n WHERE n.run_id=r.id), 0) AS spent
		FROM runs r
		JOIN nodes i ON i.run_id=r.id AND i.type='import'
		WHERE r.status IN ('running','paused')
		  AND COALESCE(json_extract(i.input_snapshot,'$.budget_usd'), 0) > 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type overBudget struct {
		id        uuid.UUID
		wasActive bool
	}
	var hit []overBudget
	for rows.Next() {
		var id uuid.UUID
		var status string
		var budget, spent float64
		if err := rows.Scan(&id, &status, &budget, &spent); err != nil {
			return nil, err
		}
		if spent >= budget {
			hit = append(hit, overBudget{id: id, wasActive: status == "running"})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var paused []uuid.UUID
	for _, h := range hit {
		if _, err := s.db.ExecContext(ctx,
			`UPDATE nodes SET status='paused' WHERE run_id=? AND status IN ('ready','pending') AND type IN ('qa','bug','flows')`, h.id); err != nil {
			return nil, err
		}
		if !h.wasActive {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE runs SET status='paused' WHERE id=? AND status='running'`, h.id); err != nil {
			return nil, err
		}
		paused = append(paused, h.id)
	}
	return paused, nil
}
