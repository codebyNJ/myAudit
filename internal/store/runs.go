package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// Node is a task in the build graph.
type Node struct {
	ID     uuid.UUID   `json:"id"`
	RunID  uuid.UUID   `json:"run_id"`
	Type   string      `json:"type"`
	Status string      `json:"status"`
	Deps   []uuid.UUID `json:"deps"`
}

// CreateRun starts a new session and returns its id.
func (s *Store) CreateRun(ctx context.Context, project string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.db.ExecContext(ctx, `INSERT INTO runs(id, project) VALUES(?,?)`, id, project)
	return id, err
}

// AddNode appends a node to a run's graph. deps may be nil.
func (s *Store) AddNode(ctx context.Context, run uuid.UUID, typ string, deps []uuid.UUID) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nodes(id, run_id, type, deps) VALUES(?,?,?,?)`,
		id, run, typ, marshalIDs(deps))
	return id, err
}

// AddNodeFull inserts a node with a spec payload and an explicit initial status.
// The map step uses it to spawn per-module qa cards as 'pending', so they promote
// to 'ready' once their deps (import→map) are done — the dynamic board fan-out.
func (s *Store) AddNodeFull(ctx context.Context, run uuid.UUID, typ string, deps []uuid.UUID, spec any, status string) (uuid.UUID, error) {
	snap := "{}"
	if spec != nil {
		b, err := json.Marshal(spec)
		if err != nil {
			return uuid.Nil, err
		}
		snap = string(b)
	}
	if status == "" {
		status = "pending"
	}
	id := uuid.New()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nodes(id, run_id, type, status, deps, input_snapshot) VALUES(?,?,?,?,?,?)`,
		id, run, typ, status, marshalIDs(deps), snap)
	return id, err
}

// GetNode fetches a node by id.
func (s *Store) GetNode(ctx context.Context, id uuid.UUID) (Node, error) {
	var n Node
	var deps string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, run_id, type, status, deps FROM nodes WHERE id=?`, id).
		Scan(&n.ID, &n.RunID, &n.Type, &n.Status, &deps)
	n.Deps = scanIDs(deps)
	return n, err
}
