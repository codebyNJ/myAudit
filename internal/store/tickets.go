package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type Bug struct {
	Title      string   `json:"title"`
	Name       string   `json:"name"`
	File       string   `json:"file"`
	Severity   string   `json:"severity"`
	Priority   string   `json:"priority"`
	Category   string   `json:"category,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
	Detail     string   `json:"detail"`
	Tags       []string `json:"tags"`
	Kind       string   `json:"type"`
}

func (s *Store) CreateBug(ctx context.Context, run uuid.UUID, b Bug, deps ...uuid.UUID) (uuid.UUID, error) {
	if b.Name == "" {
		b.Name = b.Title
	}
	if b.Kind == "" {
		b.Kind = "bug"
	}
	if b.Tags == nil {
		b.Tags = []string{}
	}

	var existingID uuid.UUID
	dup := s.db.QueryRowContext(ctx, `
		SELECT id FROM nodes WHERE run_id=? AND type='bug'
		  AND lower(trim(COALESCE(json_extract(input_snapshot,'$.title'),'')))=lower(trim(?))
		  AND lower(trim(COALESCE(json_extract(input_snapshot,'$.file'),'')))=lower(trim(?))
		LIMIT 1`, run, b.Title, b.File)
	if err := dup.Scan(&existingID); err == nil {
		return existingID, nil
	}
	snap, _ := json.Marshal(b)
	id := uuid.New()
	status := "open"
	if len(deps) > 0 {
		status = "pending"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nodes(id, run_id, type, status, deps, input_snapshot) VALUES(?,?,?,?,?,?)`,
		id, run, "bug", status, marshalIDs(deps), string(snap))
	return id, err
}

func (s *Store) SetNodeTags(ctx context.Context, node uuid.UUID, tags []string) error {
	if tags == nil {
		tags = []string{}
	}
	var raw []byte
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(input_snapshot,'{}') FROM nodes WHERE id=?`, node).Scan(&raw)
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	m["tags"] = tags
	b, _ := json.Marshal(m)
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET input_snapshot=? WHERE id=?`, string(b), node)
	return err
}

func (s *Store) ReopenableBugs(ctx context.Context, run uuid.UUID) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM nodes WHERE run_id=? AND type='bug' AND status IN ('open','failed','in_review')`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) MergeNodeSnapshot(ctx context.Context, node uuid.UUID, patch map[string]any) error {
	var raw []byte
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(input_snapshot,'{}') FROM nodes WHERE id=?`, node).Scan(&raw)
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	for k, v := range patch {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET input_snapshot=? WHERE id=?`, string(b), node)
	return err
}

func (s *Store) SetNodeStatus(ctx context.Context, node uuid.UUID, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET status=? WHERE id=?`, status, node)
	return err
}
