package publish

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	return dir
}

func headRef(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// Push PR works inside the developer's own clone; it must hand the clone back
// on the branch they were using, not leave them on the myaudit/... branch.
func TestCheckoutWorkBranchRestoresOriginalBranch(t *testing.T) {
	ctx := context.Background()
	dir := gitRepo(t)
	if out, err := exec.Command("git", "-C", dir, "checkout", "-q", "-b", "my-feature").CombinedOutput(); err != nil {
		t.Fatalf("checkout: %v: %s", err, out)
	}

	restore, err := checkoutWorkBranch(ctx, dir, "myaudit/aaa/bbb", "main")
	if err != nil {
		t.Fatalf("checkoutWorkBranch: %v", err)
	}
	if got := headRef(t, dir); got != "myaudit/aaa/bbb" {
		t.Fatalf("expected to be on the work branch, got %q", got)
	}

	restore()
	if got := headRef(t, dir); got != "my-feature" {
		t.Fatalf("original branch not restored: got %q, want my-feature", got)
	}
	restore() // idempotent
	if got := headRef(t, dir); got != "my-feature" {
		t.Fatalf("second restore moved HEAD: got %q", got)
	}
}

func TestCurrentRefFallsBackToCommitWhenDetached(t *testing.T) {
	ctx := context.Background()
	dir := gitRepo(t)
	sha, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(sha))
	if out, err := exec.Command("git", "-C", dir, "checkout", "-q", "--detach", want).CombinedOutput(); err != nil {
		t.Fatalf("detach: %v: %s", err, out)
	}

	if got := currentRef(ctx, dir); got != want {
		t.Fatalf("detached HEAD should resolve to the commit: got %q, want %q", got, want)
	}
}
