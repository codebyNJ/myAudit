package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rejecting a review must undo the agent's edit, not delete the user's file.
func TestRevertFromBaselineRestoresModifiedFile(t *testing.T) {
	ctx := context.Background()
	ws, err := Import(ctx, t.TempDir(), "rev1", makeRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(ws.Dir, "main.go")
	if err := os.WriteFile(target, []byte("package main // agent edit"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ws.RevertFromBaseline(ctx, "main.go"); err != nil {
		t.Fatalf("revert: %v", err)
	}

	b, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("file must still exist after rejecting a modification: %v", err)
	}
	if string(b) != "package main" {
		t.Fatalf("want imported contents restored, got %q", b)
	}
}

// A file the agent created has no baseline version, so reverting removes it.
func TestRevertFromBaselineRemovesCreatedFile(t *testing.T) {
	ctx := context.Background()
	ws, err := Import(ctx, t.TempDir(), "rev2", makeRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	created := filepath.Join(ws.Dir, "brand_new.go")
	if err := os.WriteFile(created, []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ws.RevertFromBaseline(ctx, "brand_new.go"); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatalf("agent-created file should be gone, stat err = %v", err)
	}
}

func TestRevertFromBaselineIgnoresMissingFile(t *testing.T) {
	ctx := context.Background()
	ws, err := Import(ctx, t.TempDir(), "rev3", makeRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := ws.RevertFromBaseline(ctx, "never_existed.go"); err != nil {
		t.Fatalf("reverting an absent path should be a no-op, got %v", err)
	}
}

// A partial copy must fail the import rather than be audited as complete.
func TestImportFailsOnUnreadableFile(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "ok.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(src, "secret.go")
	if err := os.WriteFile(bad, []byte("package main"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(bad); err == nil {
		t.Skip("running with read-anything privileges; unreadable-file case cannot be exercised")
	}

	if _, err := Import(context.Background(), t.TempDir(), "partial", src); err == nil {
		t.Fatal("import must report the unreadable file instead of silently skipping it")
	}
}

// A run whose workspace is gone — seeded, or reclaimed after finishing — has
// nothing to diff. Running git in a directory that does not exist fails in a
// way that read as a server fault, so the board put a 500 in the console every
// time you opened a ticket on an older run.
func TestDiffFromBaselineIsEmptyWhenWorkspaceIsGone(t *testing.T) {
	ws := Workspace{Dir: filepath.Join(t.TempDir(), "never-imported")}

	diff, err := ws.DiffFromBaseline(context.Background(), "")
	if err != nil {
		t.Fatalf("a missing workspace is nothing to diff, not an error: %v", err)
	}
	if diff != "" {
		t.Fatalf("want an empty diff, got %q", diff)
	}
}

func TestDiffFromBaselineStillDiffsARealWorkspace(t *testing.T) {
	ctx := context.Background()
	ws, err := Import(ctx, t.TempDir(), "diffrun", makeRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws.Dir, "main.go"), []byte("package main // changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := ws.DiffFromBaseline(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "main.go") {
		t.Fatalf("a real workspace should still produce its diff, got %q", diff)
	}
}
