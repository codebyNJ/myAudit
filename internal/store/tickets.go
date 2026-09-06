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
	// Dedup: if an equivalent ticket (same title + file, case-insensitive) already
	// exists in this run, return it instead of filing a duplicate card. Re-running
	// a module, or two modules reporting the same issue at the same location, then
	// collapses to one ticket rather than N.
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

// MergeNodeSnapshot merges keys into a node's input_snapshot JSON — used for
// manual triage edits (severity, priority, …) that the board reads back out.
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

// SetNodeStatus moves a card to a new lifecycle status (used to advance bug
// tickets: open → in_progress → in_review → verified, or failed/reopened).
func (s *Store) SetNodeStatus(ctx context.Context, node uuid.UUID, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE nodes SET status=? WHERE id=?`, status, node)
	return err
}
