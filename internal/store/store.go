package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Store is the SQLite-backed home of all RunState. One file, one connection —
// this is a local single-user IDE, not a shared control plane.
type Store struct {
	db *sql.DB
}

// Open opens (creating if absent) the SQLite database at path and applies the
// embedded schema idempotently. path is a filesystem path, e.g. "myaudit.db".
func Open(ctx context.Context, path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// ponytail: single connection = trivially serialized writes, no SQLITE_BUSY
	// dance. Raise if a local IDE ever needs real DB concurrency.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// DB exposes the underlying handle for packages that run their own SQL
// (queue, events) and for tests.
func (s *Store) DB() *sql.DB { return s.db }

// Ping verifies connectivity.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Close releases the handle.
func (s *Store) Close() { s.db.Close() }
