package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RunSummary is a run row for list views.
type RunSummary struct {
	ID        uuid.UUID `json:"id"`
	Project   string    `json:"project"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// EventRow is one event for the log view.
type EventRow struct {
	TS     time.Time  `json:"ts"`
	Kind   string     `json:"kind"`
	Level  string     `json:"level"`
	Msg    string     `json:"msg"`
	NodeID *uuid.UUID `json:"node_id,omitempty"`
}

// GetRun loads a single run row (project/status/created), so the detail endpoint
// returns real metadata instead of a bare id.
func (s *Store) GetRun(ctx context.Context, id uuid.UUID) (RunSummary, error) {
	var r RunSummary
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project, status, created_at FROM runs WHERE id=?`, id).
		Scan(&r.ID, &r.Project, &r.Status, &r.CreatedAt)
	return r, err
}

// ListRuns returns recent runs, newest first.
func (s *Store) ListRuns(ctx context.Context, limit int) ([]RunSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, project, status, created_at FROM runs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RunSummary
	for rows.Next() {
		var r RunSummary
		if err := rows.Scan(&r.ID, &r.Project, &r.Status, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NodeDetail is a node enriched for the board/cards: attempts, timing, a
// summary distilled from its output, and how many events it has emitted.
type NodeDetail struct {
	ID        uuid.UUID  `json:"id"`
	Type      string     `json:"type"`
	Name      string     `json:"name"` // spec name from input_snapshot
	Status    string     `json:"status"`
	Deps      int        `json:"deps"`
	Attempts  int        `json:"attempts"`
	Summary   string     `json:"summary"`
	Files     int        `json:"files"` // count of changed files
	CostUSD   float64    `json:"cost_usd"`
	Events    int        `json:"events"`
	CreatedAt time.Time  `json:"created_at"`
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`
	// Ticket fields (bug/feature cards) — read from input_snapshot JSON.
	Title      string   `json:"title,omitempty"`
	File       string   `json:"file,omitempty"`
	Severity   string   `json:"severity,omitempty"`
	Priority   string   `json:"priority,omitempty"`
	Category   string   `json:"category,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	Tags       []string `json:"tags"`
}

// NodesForRun returns all nodes of a run (basic, for the graph view).
func (s *Store) NodesForRun(ctx context.Context, run uuid.UUID) ([]Node, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, type, status, deps FROM nodes WHERE run_id=? ORDER BY created_at`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		var n Node
		var deps string
		if err := rows.Scan(&n.ID, &n.RunID, &n.Type, &n.Status, &deps); err != nil {
			return nil, err
		}
		n.Deps = scanIDs(deps)
		out = append(out, n)
	}
	return out, rows.Err()
}

// NodeDetailsForRun returns enriched node cards, joining event counts.
func (s *Store) NodeDetailsForRun(ctx context.Context, run uuid.UUID) ([]NodeDetail, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT n.id, n.type, coalesce(json_extract(n.input_snapshot,'$.name'),''), n.status,
		       coalesce(json_array_length(n.deps),0), n.attempts,
		       coalesce(json_extract(n.output,'$.summary'),''),
		       CASE WHEN json_type(n.output,'$.changed')='array'
		            THEN json_array_length(n.output,'$.changed') ELSE 0 END,
		       coalesce(json_extract(n.output,'$.cost_usd'),0),
		       (SELECT count(*) FROM events e WHERE e.node_id = n.id),
		       n.created_at, n.claimed_at,
		       coalesce(json_extract(n.input_snapshot,'$.title'),''),
		       coalesce(json_extract(n.input_snapshot,'$.file'),''),
		       coalesce(json_extract(n.input_snapshot,'$.severity'),''),
		       coalesce(json_extract(n.input_snapshot,'$.priority'),''),
		       coalesce(json_extract(n.input_snapshot,'$.category'),''),
		       coalesce(json_extract(n.input_snapshot,'$.confidence'),''),
		       coalesce(json_extract(n.input_snapshot,'$.detail'),''),
		       coalesce((SELECT json_group_array(value) FROM json_each(n.input_snapshot,'$.tags')),'[]')
		FROM nodes n WHERE n.run_id=? ORDER BY n.created_at`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeDetail
	for rows.Next() {
		var d NodeDetail
		var claimed sql.NullTime
		var tags string
		if err := rows.Scan(&d.ID, &d.Type, &d.Name, &d.Status, &d.Deps, &d.Attempts, &d.Summary, &d.Files, &d.CostUSD, &d.Events, &d.CreatedAt, &claimed, &d.Title, &d.File, &d.Severity, &d.Priority, &d.Category, &d.Confidence, &d.Detail, &tags); err != nil {
			return nil, err
		}
		if claimed.Valid {
			t := claimed.Time
			d.ClaimedAt = &t
		}
		d.Tags = scanTags(tags)
		out = append(out, d)
	}
	return out, rows.Err()
}

// FileEntry is a file in a run's workspace. In tree listings Content is empty
// (fetched lazily per file); Changed marks files a node produced/flagged.
type FileEntry struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Action  string `json:"action,omitempty"`
	Changed bool   `json:"changed"`
	Review  string `json:"review,omitempty"` // "" | accepted
}

// ChangedFilesForRun returns paths a run's nodes flagged (from each node
// output's `changed` array), first-seen order, excluding rejected. Content is
// NOT included — the API layer reads file content from the workspace on disk.
func (s *Store) ChangedFilesForRun(ctx context.Context, run uuid.UUID) ([]FileEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT output FROM nodes WHERE run_id=? AND output IS NOT NULL ORDER BY created_at`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var order []string
	seen := map[string]bool{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var out struct {
			Changed []string `json:"changed"`
		}
		if json.Unmarshal(raw, &out) != nil {
			continue
		}
		for _, p := range out.Changed {
			if !seen[p] {
				seen[p] = true
				order = append(order, p)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	reviews, err := s.Reviews(ctx, run)
	if err != nil {
		return nil, err
	}
	files := make([]FileEntry, 0, len(order))
	for _, p := range order {
		if reviews[p] == "rejected" {
			continue
		}
		files = append(files, FileEntry{Path: p, Action: "modified", Review: reviews[p]})
	}
	return files, nil
}

// RunCostUSD sums the cost recorded across a run's node outputs.
func (s *Store) RunCostUSD(ctx context.Context, run uuid.UUID) (float64, error) {
	var total float64
	err := s.db.QueryRowContext(ctx,
		`SELECT coalesce(sum(json_extract(output,'$.cost_usd')),0) FROM nodes
		 WHERE run_id=? AND output IS NOT NULL AND json_extract(output,'$.cost_usd') IS NOT NULL`, run).Scan(&total)
	return total, err
}

// OpenCheckpointsForRun returns unresolved checkpoints for a run.
func (s *Store) OpenCheckpointsForRun(ctx context.Context, run uuid.UUID) ([]Checkpoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, node_id, question, resolved, COALESCE(answer,'')
		 FROM checkpoints WHERE run_id=? AND resolved=0 ORDER BY created_at`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Checkpoint
	for rows.Next() {
		var c Checkpoint
		if err := rows.Scan(&c.ID, &c.RunID, &c.NodeID, &c.Question, &c.Resolved, &c.Answer); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// EventsForRun returns a run's most recent `limit` events, oldest-first.
// It selects the newest by id DESC (id is monotonic; ts ties at 1s resolution)
// then reverses, so a long run keeps showing its LATEST activity instead of
// freezing on the first 200 events (chat/activity/verify all read this).
func (s *Store) EventsForRun(ctx context.Context, run uuid.UUID, limit int) ([]EventRow, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT ts, kind, level, COALESCE(msg,''), node_id FROM events WHERE run_id=? ORDER BY id DESC LIMIT ?`, run, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var e EventRow
		var node sql.NullString
		if err := rows.Scan(&e.TS, &e.Kind, &e.Level, &e.Msg, &node); err != nil {
			return nil, err
		}
		e.NodeID = nullUUID(node)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// reverse newest-first → oldest-first for chronological rendering
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// FlowsForRun returns the raw flows document from the most recent completed
// flows node of a run, or nil when the run has none yet.
func (s *Store) FlowsForRun(ctx context.Context, run uuid.UUID) (json.RawMessage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT output FROM nodes WHERE run_id=? AND type='flows' AND output IS NOT NULL ORDER BY created_at DESC`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var out struct {
			Flows json.RawMessage `json:"flows"`
		}
		if json.Unmarshal(raw, &out) != nil || len(out.Flows) == 0 {
			continue
		}
		return out.Flows, nil
	}
	return nil, rows.Err()
}

// HasPendingFlows reports whether a flows node is already queued or running,
// so the API doesn't stack duplicates.
func (s *Store) HasPendingFlows(ctx context.Context, run uuid.UUID) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM nodes WHERE run_id=? AND type='flows' AND status IN ('pending','ready','running')`, run).Scan(&n)
	return n > 0, err
}
