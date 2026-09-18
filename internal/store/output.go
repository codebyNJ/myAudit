package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// MergeNodeOutput applies patch to the node's output in one statement; see
// MergeNodeSnapshot for why the read-modify-write it replaces was unsafe.
func (s *Store) MergeNodeOutput(ctx context.Context, node uuid.UUID, patch map[string]any) error {
	b, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE nodes SET output=json_patch(COALESCE(output,'{}'), ?) WHERE id=?`,
		string(b), node)
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
