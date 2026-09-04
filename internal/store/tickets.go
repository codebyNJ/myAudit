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

// CreateBug inserts a bug ticket node. With no deps it lands in the manual 'open'
// state (never queue-claimed) — the legacy behavior. Passed a dep (its module's
// qa node), it lands 'pending' and blocked on that dep, so the autonomous dev fix
// loop can only pick it up AFTER that module's QA is done ("QA over dev"): the
// queue's PromoteReady flips it to 'ready' once the qa node completes.
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

// ReopenableBugs returns the ids of bug tickets that can be (re-)queued for the
// autonomous dev loop: still open, failed a prior attempt, or parked in review.
// Used by the chat "fix" command.
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

// SetNodeStatus moves a card to a new lifecycle status (used to advance bug
// tickets: open → in_progress → in_review → verified, or failed/reopened).
func (s *Store) SetNodeStatus(ctx context.Context, node uuid.UUID, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET status=? WHERE id=?`, status, node)
	return err
}
