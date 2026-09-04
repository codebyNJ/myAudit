package store

import (
	"context"
	"path/filepath"
	"testing"
)

// testURL returns a fresh SQLite file path in the test's temp dir, so every
// test gets an isolated database (no shared-DB TRUNCATE dance).
func testURL(t *testing.T) string {
	return filepath.Join(t.TempDir(), "test.db")
}

func TestOpenAndPing(t *testing.T) {
	s, err := Open(context.Background(), testURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}
