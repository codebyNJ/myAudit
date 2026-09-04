package store

import (
	"context"

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

// CreateRun starts a new build session and returns its id.
func (s *Store) CreateRun(ctx context.Context, project string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `INSERT INTO runs(project) VALUES($1) RETURNING id`, project).Scan(&id)
	return id, err
}

// AddNode appends a node to a run's graph. deps may be nil.
func (s *Store) AddNode(ctx context.Context, run uuid.UUID, typ string, deps []uuid.UUID) (uuid.UUID, error) {
	if deps == nil {
		deps = []uuid.UUID{}
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO nodes(run_id, type, deps) VALUES($1,$2,$3) RETURNING id`,
		run, typ, deps).Scan(&id)
	return id, err
}

// GetNode fetches a node by id.
func (s *Store) GetNode(ctx context.Context, id uuid.UUID) (Node, error) {
	var n Node
	err := s.pool.QueryRow(ctx,
		`SELECT id, run_id, type, status, deps FROM nodes WHERE id=$1`, id).
		Scan(&n.ID, &n.RunID, &n.Type, &n.Status, &n.Deps)
	return n, err
}
