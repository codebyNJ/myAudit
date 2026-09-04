package store

import (
	"context"

	"github.com/google/uuid"
)

// SetReview records an accept/reject decision for a generated file.
func (s *Store) SetReview(ctx context.Context, run uuid.UUID, path, status string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO file_reviews(run_id, path, status, updated_at) VALUES($1,$2,$3,now())
		 ON CONFLICT (run_id, path) DO UPDATE SET status=EXCLUDED.status, updated_at=now()`,
		run, path, status)
	return err
}

// Reviews returns the current review status per path for a run.
func (s *Store) Reviews(ctx context.Context, run uuid.UUID) (map[string]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT path, status FROM file_reviews WHERE run_id=$1`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var p, st string
		if err := rows.Scan(&p, &st); err != nil {
			return nil, err
		}
		out[p] = st
	}
	return out, rows.Err()
}
