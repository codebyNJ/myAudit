package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// TaskSpec describes one node in a graph. Deps are referenced by Key so a plan
// can be written before any ids exist; CreateGraph resolves them.
type TaskSpec struct {
	Key     string
	Type    string
	DepKeys []string
	// Spec is per-node data stored in nodes.input_snapshot; the worker reads it
	// to build the agent task. nil ⇒ stored as '{}'.
	Spec any
}

// CreateGraph materializes a whole run + its nodes in one transaction, wiring
// deps from keys to the generated node ids. Returns the run id and a
// key→node-id map. An unknown dep key aborts the whole graph.
func (s *Store) CreateGraph(ctx context.Context, project string, specs []TaskSpec) (uuid.UUID, map[string]uuid.UUID, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.Nil, nil, err
	}
	defer tx.Rollback()

	run := uuid.New()
	if _, err := tx.ExecContext(ctx, `INSERT INTO runs(id, project) VALUES(?,?)`, run, project); err != nil {
		return uuid.Nil, nil, err
	}

	ids := make(map[string]uuid.UUID, len(specs))
	// First pass: create nodes without deps so every key has an id.
	for _, sp := range specs {
		snap := "{}"
		if sp.Spec != nil {
			b, err := json.Marshal(sp.Spec)
			if err != nil {
				return uuid.Nil, nil, fmt.Errorf("marshal spec for %q: %w", sp.Key, err)
			}
			snap = string(b)
		}
		id := uuid.New()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO nodes(id, run_id, type, input_snapshot) VALUES(?,?,?,?)`,
			id, run, sp.Type, snap); err != nil {
			return uuid.Nil, nil, err
		}
		ids[sp.Key] = id
	}
	// Second pass: resolve dep keys and set the deps array.
	for _, sp := range specs {
		if len(sp.DepKeys) == 0 {
			continue
		}
		deps := make([]uuid.UUID, 0, len(sp.DepKeys))
		for _, dk := range sp.DepKeys {
			did, ok := ids[dk]
			if !ok {
				return uuid.Nil, nil, fmt.Errorf("unknown dep key %q for %q", dk, sp.Key)
			}
			deps = append(deps, did)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE nodes SET deps=? WHERE id=?`, marshalIDs(deps), ids[sp.Key]); err != nil {
			return uuid.Nil, nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, nil, err
	}
	return run, ids, nil
}
