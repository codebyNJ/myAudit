package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchName(t *testing.T) {
	run := "fcc9ddf7-379c-40e9-9f12-710e6918a2dd"
	node := "9b0fe6e7-ec6f-4a9f-a32e-b879750511f3"
	if got, want := BranchName(run, node), "myaudit/fcc9ddf7/9b0fe6e7"; got != want {
		t.Fatalf("BranchName = %q, want %q", got, want)
	}
	// Short or empty ids must not panic or slice out of range.
	for _, c := range []struct{ run, node, want string }{
		{"abc", "de", "myaudit/abc/de"},
		{"", "", "myaudit//"},
	} {
		if got := BranchName(c.run, c.node); got != c.want {
			t.Errorf("BranchName(%q,%q) = %q, want %q", c.run, c.node, got, c.want)
		}
	}
}

func TestCreateBranchPointsAtCommitWithoutMovingHead(t *testing.T) {
	ctx := context.Background()
	ws := Workspace{Dir: t.TempDir()}
	if err := ensureGitBaseline(ctx, ws.Dir); err != nil {
		t.Fatal(err)
	}
	sha, err := ws.HeadSHA(ctx)
	if err != nil {
		t.Fatal(err)
	}
	headBefore := gitOut(t, ws.Dir, "rev-parse", "--abbrev-ref", "HEAD")

	if err := ws.CreateBranch(ctx, "myaudit/aaaaaaaa/bbbbbbbb", sha); err != nil {
		t.Fatal(err)
	}
	if got := gitOut(t, ws.Dir, "rev-parse", "myaudit/aaaaaaaa/bbbbbbbb"); got != sha {
		t.Fatalf("branch points at %q, want %q", got, sha)
	}
	// The whole point of a bare ref: concurrent fixes share this working tree.
	if got := gitOut(t, ws.Dir, "rev-parse", "--abbrev-ref", "HEAD"); got != headBefore {
		t.Fatalf("HEAD moved to %q, want %q", got, headBefore)
	}
}

// A retried fix node commits again and must be able to re-point its branch.
func TestCreateBranchIsForced(t *testing.T) {
	ctx := context.Background()
	ws := Workspace{Dir: t.TempDir()}
	if err := ensureGitBaseline(ctx, ws.Dir); err != nil {
		t.Fatal(err)
	}
	first, _ := ws.HeadSHA(ctx)
	if err := ws.CreateBranch(ctx, "myaudit/aa/bb", first); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(ws.Dir, "fix.txt"), []byte("fixed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ws.Commit(ctx, "second"); err != nil {
		t.Fatal(err)
	}
	second, _ := ws.HeadSHA(ctx)
	if second == first {
		t.Fatal("second commit did not produce a new sha")
	}
	if err := ws.CreateBranch(ctx, "myaudit/aa/bb", second); err != nil {
		t.Fatalf("re-pointing an existing branch failed: %v", err)
	}
	if got := gitOut(t, ws.Dir, "rev-parse", "myaudit/aa/bb"); got != second {
		t.Fatalf("branch still at %q, want %q", got, second)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}
