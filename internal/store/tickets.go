package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Bug is a ticket auto-filed from a finding (verify failure or review issue). It
// is stored as a node with type='bug' and a lifecycle status ('open' initially),
// so it shows on the same board but is never claimed by the worker queue (which
// only takes status='ready').
type Bug struct {
	Title    string   `json:"title"`
	Name     string   `json:"name"`     // short label for the card
	File     string   `json:"file"`     // file:line or path
	Severity string   `json:"severity"` // high | medium | low
	Priority string   `json:"priority"` // P0 | P1 | P2
	Detail   string   `json:"detail"`   // the finding body
	Tags     []string `json:"tags"`
	Kind     string   `json:"type"` // "bug" | "feature" | "chore" (card type)
}

// CreateBug inserts a bug ticket node in the 'open' lifecycle state.
func (s *Store) CreateBug(ctx context.Context, run uuid.UUID, b Bug) (uuid.UUID, error) {
	if b.Name == "" {
		b.Name = b.Title
	}
	if b.Kind == "" {
		b.Kind = "bug"
	}
	if b.Tags == nil {
		b.Tags = []string{}
	}
	snap, _ := json.Marshal(b)
	id := uuid.New()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nodes(id, run_id, type, status, input_snapshot) VALUES(?,?,?, 'open', ?)`,
		id, run, "bug", string(snap))
	return id, err
}

// SetNodeTags replaces a node's tags (merged into its input_snapshot JSON).
// Works for any card — audit node or bug ticket.
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

// SetNodeStatus moves a card to a new lifecycle status (used to advance bug
// tickets: open → in_progress → in_review → verified, or failed/reopened).
func (s *Store) SetNodeStatus(ctx context.Context, node uuid.UUID, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET status=? WHERE id=?`, status, node)
	return err
}
