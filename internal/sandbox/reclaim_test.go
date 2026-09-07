package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReclaimDropsCachesKeepsSource(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "app/page.tsx"), "export default 1")
	write(t, filepath.Join(dir, "lib/player.test.ts"), "test")
	write(t, filepath.Join(dir, ".myaudit/preview/app.png"), "png")
	write(t, filepath.Join(dir, "node_modules/react/index.js"), "heavy")
	write(t, filepath.Join(dir, "app/.next/build.js"), "heavy")
	write(t, filepath.Join(dir, "target/debug/bin"), "heavy")

	freed, err := Reclaim(dir)
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if freed <= 0 {
		t.Fatalf("expected freed bytes, got %d", freed)
	}

	for _, keep := range []string{"app/page.tsx", "lib/player.test.ts", ".myaudit/preview/app.png"} {
		if _, err := os.Stat(filepath.Join(dir, keep)); err != nil {
			t.Errorf("reclaim deleted %s, which must be kept: %v", keep, err)
		}
	}
	for _, gone := range []string{"node_modules", "app/.next", "target"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed", gone)
		}
	}
}

// A workspace that IS itself named node_modules must not delete itself.
func TestReclaimIgnoresRootName(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "node_modules")
	write(t, filepath.Join(dir, "src/main.go"), "package main")

	if _, err := Reclaim(dir); err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "src/main.go")); err != nil {
		t.Errorf("reclaim removed the workspace root: %v", err)
	}
}

func TestReclaimMissingDirIsNotFatal(t *testing.T) {
	if _, err := Reclaim(filepath.Join(t.TempDir(), "nope")); err != nil {
		t.Fatalf("expected nil error for a missing workspace, got %v", err)
	}
}
