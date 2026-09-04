package store

import (
	"context"
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

// ListRuns returns recent runs, newest first.
func (s *Store) ListRuns(ctx context.Context, limit int) ([]RunSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, project, status, created_at FROM runs ORDER BY created_at DESC LIMIT $1`, limit)
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
	Name      string     `json:"name"` // resource name for feature nodes (from input_snapshot)
	Status    string     `json:"status"`
	Deps      int        `json:"deps"`
	Attempts  int        `json:"attempts"`
	Summary   string     `json:"summary"`
	Files     int        `json:"files"` // count of changed files
	CostUSD   float64    `json:"cost_usd"`
	Events    int        `json:"events"`
	CreatedAt time.Time  `json:"created_at"`
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`
}

// NodesForRun returns all nodes of a run (basic, for the graph view).
func (s *Store) NodesForRun(ctx context.Context, run uuid.UUID) ([]Node, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, run_id, type, status, deps FROM nodes WHERE run_id=$1 ORDER BY created_at`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.RunID, &n.Type, &n.Status, &n.Deps); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// NodeDetailsForRun returns enriched node cards, joining event counts.
func (s *Store) NodeDetailsForRun(ctx context.Context, run uuid.UUID) ([]NodeDetail, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT n.id, n.type, coalesce(n.input_snapshot->>'name',''), n.status,
		       coalesce(array_length(n.deps,1),0), n.attempts,
		       coalesce(n.output->>'summary',''),
		       CASE WHEN jsonb_typeof(n.output->'changed') = 'array'
		            THEN jsonb_array_length(n.output->'changed') ELSE 0 END,
		       coalesce((n.output->>'cost_usd')::float8, 0),
		       (SELECT count(*) FROM events e WHERE e.node_id = n.id),
		       n.created_at, n.claimed_at
		FROM nodes n WHERE n.run_id=$1 ORDER BY n.created_at`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NodeDetail
	for rows.Next() {
		var d NodeDetail
		if err := rows.Scan(&d.ID, &d.Type, &d.Name, &d.Status, &d.Deps, &d.Attempts, &d.Summary, &d.Files, &d.CostUSD, &d.Events, &d.CreatedAt, &d.ClaimedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// FileEntry is a file in a run's workspace. In tree listings Content is empty
// (fetched lazily per file); Changed marks files a feature node produced.
type FileEntry struct {
	Path    string `json:"path"`
	Content string `json:"content,omitempty"`
	Action  string `json:"action,omitempty"`
	Changed bool   `json:"changed"`
	Review  string `json:"review,omitempty"` // "" | accepted
}

// ChangedFilesForRun returns the paths a run's feature nodes changed (from each
// node output's `changed` array), first-seen order, excluding rejected. Content
// is NOT included — the agent wrote the files to disk; the API layer reads their
// content from the workspace. Accepted files carry their review status.
func (s *Store) ChangedFilesForRun(ctx context.Context, run uuid.UUID) ([]FileEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT output FROM nodes WHERE run_id=$1 AND output IS NOT NULL ORDER BY created_at`, run)
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

// ResourcesForRun returns the domain resources a run's feature nodes were
// created for (from each feature node's input_snapshot), in graph order.
func (s *Store) ResourcesForRun(ctx context.Context, run uuid.UUID) ([]Resource, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT input_snapshot FROM nodes WHERE run_id=$1 AND type='feature' ORDER BY created_at`, run)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Resource
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r Resource
		if json.Unmarshal(raw, &r) == nil && r.Name != "" {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

// RunCostUSD sums the cost recorded across a run's node outputs.
func (s *Store) RunCostUSD(ctx context.Context, run uuid.UUID) (float64, error) {
	var total float64
	err := s.pool.QueryRow(ctx,
		`SELECT coalesce(sum((output->>'cost_usd')::float8),0) FROM nodes WHERE run_id=$1 AND output ? 'cost_usd'`, run).Scan(&total)
	return total, err
}

// OpenCheckpointsForRun returns unresolved checkpoints for a run.
func (s *Store) OpenCheckpointsForRun(ctx context.Context, run uuid.UUID) ([]Checkpoint, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, run_id, node_id, question, resolved, COALESCE(answer,'')
		 FROM checkpoints WHERE run_id=$1 AND NOT resolved ORDER BY created_at`, run)
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

// EventsForRun returns a run's events, oldest first.
func (s *Store) EventsForRun(ctx context.Context, run uuid.UUID, limit int) ([]EventRow, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.pool.Query(ctx,
		`SELECT ts, kind, level, COALESCE(msg,''), node_id FROM events WHERE run_id=$1 ORDER BY ts LIMIT $2`, run, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var e EventRow
		if err := rows.Scan(&e.TS, &e.Kind, &e.Level, &e.Msg, &e.NodeID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
