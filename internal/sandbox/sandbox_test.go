package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeRepo builds a small source repo to import: a file, plus a node_modules
// dir that must be excluded from the copy.
func makeRepo(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "main.go"), []byte("package main"), 0o644)
	os.MkdirAll(filepath.Join(src, "node_modules", "foo"), 0o755)
	os.WriteFile(filepath.Join(src, "node_modules", "foo", "x.js"), []byte("x"), 0o644)
	return src
}

func TestImportCopiesAndBaselines(t *testing.T) {
	ws, err := Import(context.Background(), t.TempDir(), "run1", makeRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir, "main.go")); err != nil {
		t.Fatalf("main.go not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir, "node_modules")); err == nil {
		t.Fatal("node_modules should have been excluded from the copy")
	}
	if out, err := exec.Command("git", "-C", ws.Dir, "rev-parse", "HEAD").Output(); err != nil || len(out) == 0 {
		t.Fatalf("expected a committed git HEAD baseline: %v", err)
	}
}

// A symlink-to-directory (e.g. Vercel .func output) must not abort the import.
func TestImportHandlesSymlinkToDir(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "real"), 0o755)
	os.WriteFile(filepath.Join(src, "real", "f.txt"), []byte("hi"), 0o644)
	if err := os.Symlink("real", filepath.Join(src, "link.func")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	ws, err := Import(context.Background(), t.TempDir(), "sl", src)
	if err != nil {
		t.Fatalf("import must not fail on a symlinked dir: %v", err)
	}
	fi, err := os.Lstat(filepath.Join(ws.Dir, "link.func"))
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink should be preserved as a symlink: %v", err)
	}
}

func TestImportRejectsMissingSource(t *testing.T) {
	if _, err := Import(context.Background(), t.TempDir(), "run0", "/no/such/dir"); err == nil {
		t.Fatal("import of a missing directory should error")
	}
}

func TestRunAndDiffCapture(t *testing.T) {
	ctx := context.Background()
	ws, err := Import(ctx, t.TempDir(), "run2", makeRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	// A fresh import has a committed baseline → no diff.
	if d, err := ws.Diff(ctx); err != nil || strings.TrimSpace(d) != "" {
		t.Fatalf("expected empty diff on fresh import, got %q (err %v)", d, err)
	}
	// A change shows up in the diff.
	if _, code, err := ws.Run(ctx, "sh", "-c", "echo hi > NEWFILE.txt"); err != nil || code != 0 {
		t.Fatalf("run failed: code=%d err=%v", code, err)
	}
	d, err := ws.Diff(ctx)
	if err != nil || !strings.Contains(d, "NEWFILE.txt") {
		t.Fatalf("diff should mention NEWFILE.txt, got %q (err %v)", d, err)
	}
	// After committing the node, the diff resets to empty.
	if err := ws.Commit(ctx, "node: add NEWFILE"); err != nil {
		t.Fatal(err)
	}
	if d, err := ws.Diff(ctx); err != nil || strings.TrimSpace(d) != "" {
		t.Fatalf("expected empty diff after commit, got %q (err %v)", d, err)
	}
}
