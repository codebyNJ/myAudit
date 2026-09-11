package store

import (
	"context"

	"github.com/google/uuid"
)

func (s *Store) SetReview(ctx context.Context, run uuid.UUID, path, status string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO file_reviews(run_id, path, status, updated_at) VALUES(?,?,?,CURRENT_TIMESTAMP)
		 ON CONFLICT(run_id, path) DO UPDATE SET status=excluded.status, updated_at=CURRENT_TIMESTAMP`,
		run, path, status)
	return err
}

func (s *Store) Reviews(ctx context.Context, run uuid.UUID) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path, status FROM file_reviews WHERE run_id=?`, run)
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
