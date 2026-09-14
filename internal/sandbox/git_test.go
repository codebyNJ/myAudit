package sandbox

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProbeRepoNoGit(t *testing.T) {
	dir := t.TempDir()
	info := ProbeRepo(context.Background(), dir)
	if info.HasGit {
		t.Fatal("expected no git")
	}
}

func TestProbeRepoWithOrigin(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t.com"},
		{"config", "user.name", "t"},
		{"commit", "--allow-empty", "-q", "-m", "init"},
	} {
		cmdArgs := append([]string{"-C", dir}, args...)
		if out, err := exec.CommandContext(ctx, "git", cmdArgs...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	remote := filepath.Join(t.TempDir(), "remote.git")
	exec.CommandContext(ctx, "git", "init", "--bare", "-q", remote).Run()
	exec.CommandContext(ctx, "git", "-C", dir, "remote", "add", "origin", remote).Run()
	exec.CommandContext(ctx, "git", "-C", dir, "push", "-u", "origin", "HEAD").Run()

	info := ProbeRepo(ctx, dir)
	if !info.HasGit {
		t.Fatal("expected has_git")
	}
	if info.RemoteURL == "" {
		t.Fatal("expected remote_url")
	}
	if info.DefaultBranch == "" {
		t.Fatal("expected default_branch")
	}
	if info.HeadSHA == "" {
		t.Fatal("expected head_sha")
	}
}

func TestHeadSHAAndFormatPatch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	ws := Workspace{Dir: dir}
	if err := ensureGitBaseline(ctx, dir); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644)
	if err := ws.Commit(ctx, "add f"); err != nil {
		t.Fatal(err)
	}
	sha, err := ws.HeadSHA(ctx)
	if err != nil || sha == "" {
		t.Fatalf("HeadSHA: %v", err)
	}
	patch, err := ws.FormatPatch(ctx, sha)
	if err != nil || !strings.Contains(patch, "f.txt") {
		t.Fatalf("FormatPatch: %v patch=%q", err, patch)
	}
}
