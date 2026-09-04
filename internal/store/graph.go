package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Resource is a per-project domain entity the agent generates a feature module
// for (beyond the template's generic Item example). It is the node spec of a
// "feature" node.
type Resource struct {
	Name   string  `json:"name"`
	Fields []Field `json:"fields"`
}

// Field is one attribute of a Resource.
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// TaskSpec describes one node in a build graph. Deps are referenced by Key so a
// plan can be written before any ids exist; CreateGraph resolves them.
type TaskSpec struct {
	Key     string
	Type    string
	DepKeys []string
	// Spec is per-node data (e.g. a feature's resource + fields, or config
	// choices). Stored in nodes.input_snapshot; the worker reads it to build the
	// agent task. nil ⇒ stored as '{}'.
	Spec any
}

// CreateGraph materializes a whole run + its nodes in one transaction, wiring
// deps from keys to the generated node ids. Returns the run id and a
// key→node-id map. An unknown dep key aborts the whole graph.
func (s *Store) CreateGraph(ctx context.Context, project string, specs []TaskSpec) (uuid.UUID, map[string]uuid.UUID, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, nil, err
	}
	defer tx.Rollback(ctx)

	var run uuid.UUID
	if err := tx.QueryRow(ctx, `INSERT INTO runs(project) VALUES($1) RETURNING id`, project).Scan(&run); err != nil {
		return uuid.Nil, nil, err
	}

	ids := make(map[string]uuid.UUID, len(specs))
	// First pass: create nodes without deps so every key has an id.
	for _, sp := range specs {
		snap := []byte("{}")
		if sp.Spec != nil {
			b, err := json.Marshal(sp.Spec)
			if err != nil {
				return uuid.Nil, nil, fmt.Errorf("marshal spec for %q: %w", sp.Key, err)
			}
			snap = b
		}
		var id uuid.UUID
		if err := tx.QueryRow(ctx,
			`INSERT INTO nodes(run_id, type, input_snapshot) VALUES($1,$2,$3) RETURNING id`,
			run, sp.Type, snap).Scan(&id); err != nil {
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
		if _, err := tx.Exec(ctx, `UPDATE nodes SET deps=$2 WHERE id=$1`, ids[sp.Key], deps); err != nil {
			return uuid.Nil, nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		if err == pgx.ErrTxClosed {
			return uuid.Nil, nil, err
		}
		return uuid.Nil, nil, err
	}
	return run, ids, nil
}
