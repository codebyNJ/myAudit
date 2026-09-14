package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitInfo describes the source repository at import time.
type GitInfo struct {
	HasGit        bool   `json:"has_git"`
	RemoteURL     string `json:"remote_url,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	HeadSHA       string `json:"head_sha,omitempty"`
}

// ProbeRepo inspects path for a git worktree and origin metadata.
func ProbeRepo(ctx context.Context, path string) GitInfo {
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return GitInfo{}
	}
	out, code, err := gitAt(ctx, path, "rev-parse", "--is-inside-work-tree")
	if err != nil || code != 0 || strings.TrimSpace(out) != "true" {
		return GitInfo{}
	}
	info := GitInfo{HasGit: true}
	if u, _, _ := gitAt(ctx, path, "remote", "get-url", "origin"); strings.TrimSpace(u) != "" {
		info.RemoteURL = strings.TrimSpace(u)
	}
	if ref, _, _ := gitAt(ctx, path, "symbolic-ref", "refs/remotes/origin/HEAD"); strings.TrimSpace(ref) != "" {
		ref = strings.TrimSpace(ref)
		if i := strings.LastIndex(ref, "/"); i >= 0 {
			info.DefaultBranch = ref[i+1:]
		}
	}
	if info.DefaultBranch == "" {
		if b, _, _ := gitAt(ctx, path, "branch", "--show-current"); strings.TrimSpace(b) != "" {
			info.DefaultBranch = strings.TrimSpace(b)
		}
	}
	if info.DefaultBranch == "" {
		info.DefaultBranch = "main"
	}
	if sha, _, _ := gitAt(ctx, path, "rev-parse", "HEAD"); strings.TrimSpace(sha) != "" {
		info.HeadSHA = strings.TrimSpace(sha)
	}
	return info
}

func gitAt(ctx context.Context, dir string, args ...string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	b, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(b), ee.ExitCode(), nil
		}
		return string(b), -1, err
	}
	return string(b), 0, nil
}

// HeadSHA returns the current commit in the workspace.
func (w Workspace) HeadSHA(ctx context.Context) (string, error) {
	out, code, err := w.Run(ctx, "git", "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git rev-parse HEAD: %s", out)
	}
	return strings.TrimSpace(out), nil
}

// FormatPatch returns a mailbox patch for a single commit.
func (w Workspace) FormatPatch(ctx context.Context, sha string) (string, error) {
	out, code, err := w.Run(ctx, "git", "format-patch", "-1", sha, "--stdout")
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("git format-patch: %s", out)
	}
	return out, nil
}

// RepoGit runs git in an arbitrary directory (source repo_path).
func RepoGit(ctx context.Context, dir string, args ...string) (string, int, error) {
	return gitAt(ctx, dir, args...)
}
