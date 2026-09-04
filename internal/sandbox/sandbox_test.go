package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffoldCopiesTemplateAndInits(t *testing.T) {
	if os.Getenv("TEMPLATE_PATH") == "" {
		t.Skip("TEMPLATE_PATH not set (real template required — no mocks)")
	}
	ws, err := Scaffold(context.Background(), t.TempDir(), "run1", "Lumen", "lumen")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir, "backend/src/server.js")); err != nil {
		t.Fatalf("missing server.js: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(ws.Dir, "package.json"))
	if strings.Contains(string(b), `"acme"`) {
		t.Fatalf("identity not rewritten, still contains acme: %s", b)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir, "frontend/node_modules")); err == nil {
		t.Fatal("node_modules should have been excluded from the copy")
	}
	if out, err := exec.Command("git", "-C", ws.Dir, "rev-parse", "HEAD").Output(); err != nil || len(out) == 0 {
		t.Fatalf("expected a committed git HEAD baseline: %v", err)
	}
}

func TestRunAndDiffCapture(t *testing.T) {
	if os.Getenv("TEMPLATE_PATH") == "" {
		t.Skip("TEMPLATE_PATH not set")
	}
	ws, err := Scaffold(context.Background(), t.TempDir(), "run2", "Lumen", "lumen")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// A fresh scaffold has a committed baseline → no diff.
	if d, err := ws.Diff(ctx); err != nil || strings.TrimSpace(d) != "" {
		t.Fatalf("expected empty diff on fresh scaffold, got %q (err %v)", d, err)
	}

	// Run a command that changes the tree; the diff must reflect it.
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

