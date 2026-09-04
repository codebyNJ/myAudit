package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// GetNotes returns a run's notes ("" if none yet).
func (s *Store) GetNotes(ctx context.Context, run uuid.UUID) (string, error) {
	var content string
	err := s.pool.QueryRow(ctx, `SELECT content FROM notes WHERE run_id=$1`, run).Scan(&content)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", nil
		}
		return "", err
	}
	return content, nil
}

// PutNotes upserts a run's notes.
func (s *Store) PutNotes(ctx context.Context, run uuid.UUID, content string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO notes(run_id, content, updated_at) VALUES($1,$2,now())
		 ON CONFLICT (run_id) DO UPDATE SET content=EXCLUDED.content, updated_at=now()`,
		run, content)
	return err
}

// GetSettings returns the singleton settings blob.
func (s *Store) GetSettings(ctx context.Context) (map[string]any, error) {
	var raw []byte
	if err := s.pool.QueryRow(ctx, `SELECT data FROM settings WHERE id=1`).Scan(&raw); err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// PutSettings merges the given keys into the settings blob.
func (s *Store) PutSettings(ctx context.Context, patch map[string]any) (map[string]any, error) {
	cur, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	for k, v := range patch {
		cur[k] = v
	}
	raw, _ := json.Marshal(cur)
	if _, err := s.pool.Exec(ctx, `UPDATE settings SET data=$1, updated_at=now() WHERE id=1`, raw); err != nil {
		return nil, err
	}
	return cur, nil
}
