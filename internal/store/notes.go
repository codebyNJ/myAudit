package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

func (s *Store) GetNotes(ctx context.Context, run uuid.UUID) (string, error) {
	var content string
	err := s.db.QueryRowContext(ctx, `SELECT content FROM notes WHERE run_id=?`, run).Scan(&content)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return content, err
}

func (s *Store) PutNotes(ctx context.Context, run uuid.UUID, content string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notes(run_id, content, updated_at) VALUES(?,?,CURRENT_TIMESTAMP)
		 ON CONFLICT(run_id) DO UPDATE SET content=excluded.content, updated_at=CURRENT_TIMESTAMP`,
		run, content)
	return err
}

// AppendNotes adds to the run's notes in one statement. Read-modify-write from
// Go loses findings whenever two nodes of a run write notes at the same time;
// SQLite concatenating server-side cannot.
func (s *Store) AppendNotes(ctx context.Context, run uuid.UUID, text string) error {
	if text == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO notes(run_id, content, updated_at) VALUES(?,?,CURRENT_TIMESTAMP)
		 ON CONFLICT(run_id) DO UPDATE SET content=content||excluded.content, updated_at=CURRENT_TIMESTAMP`,
		run, text)
	return err
}

func (s *Store) GetSettings(ctx context.Context) (map[string]any, error) {
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT data FROM settings WHERE id=1`).Scan(&raw); err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Store) PutSettings(ctx context.Context, patch map[string]any) (map[string]any, error) {
	cur, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	for k, v := range patch {
		cur[k] = v
	}
	raw, _ := json.Marshal(cur)
	if _, err := s.db.ExecContext(ctx, `UPDATE settings SET data=?, updated_at=CURRENT_TIMESTAMP WHERE id=1`, raw); err != nil {
		return nil, err
	}
	return cur, nil
}
