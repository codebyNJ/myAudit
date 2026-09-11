package store

import (
	"path/filepath"
	"testing"
)

func testURL(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.db")
}
