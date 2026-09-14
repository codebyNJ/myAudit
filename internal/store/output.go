package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

func (s *Store) MergeNodeOutput(ctx context.Context, node uuid.UUID, patch map[string]any) error {
	var raw []byte
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(output,'{}') FROM nodes WHERE id=?`, node).Scan(&raw)
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	for k, v := range patch {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET output=? WHERE id=?`, string(b), node)
	return err
}

func (s *Store) NodeOutput(ctx context.Context, node uuid.UUID) (map[string]any, error) {
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(output,'{}') FROM nodes WHERE id=?`, node).Scan(&raw); err != nil {
		return nil, err
	}
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	return m, nil
}
