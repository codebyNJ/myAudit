package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the Postgres-backed home of all RunState.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects a pgx pool to the given Postgres URL.
func Open(ctx context.Context, url string) (*Store, error) {
	p, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Store{pool: p}, nil
}

// Pool exposes the underlying pool for packages that run their own SQL.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Ping verifies connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }
