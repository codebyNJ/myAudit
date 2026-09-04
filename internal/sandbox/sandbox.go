// Package sandbox creates a per-run workspace by copying an imported codebase
// into an isolated directory with a committed git baseline. All node work
// (understanding, test generation, review) happens on this copy, never the
// user's original repo.
package sandbox

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Workspace is a per-run working copy of an imported codebase.
type Workspace struct{ Dir string }

// excludeDirs are skipped when copying a repo — VCS metadata and heavy build
// artifacts we never want in the working copy.
var excludeDirs = map[string]bool{".git": true, "node_modules": true, "dist": true, "build": true, ".next": true}

// Import copies the repo at src into <root>/<runID> (skipping excludeDirs) and
// gives it a single committed baseline so per-node git diffs have a clean start.
// The user's original repo is never touched.
func Import(ctx context.Context, root, runID, src string) (Workspace, error) {
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return Workspace{}, fmt.Errorf("import source %q is not a directory", src)
	}
	dir := filepath.Join(root, runID)
	if err := copyDir(src, dir); err != nil {
		return Workspace{}, fmt.Errorf("copy repo: %w", err)
	}
	if err := ensureGitBaseline(ctx, dir); err != nil {
		return Workspace{}, err
	}
	return Workspace{Dir: dir}, nil
}

// Run executes a command inside the workspace and returns combined output and
// exit code. A non-zero exit is returned in code (not err); err is only for
// failures to start/execute.
func (w Workspace) Run(ctx context.Context, name string, args ...string) (out string, code int, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = w.Dir
	cmd.Stdin = nil
	b, err := cmd.CombinedOutput()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return string(b), ee.ExitCode(), nil
		}
		return string(b), -1, err
	}
	return string(b), 0, nil
}

// Diff returns the uncommitted changes in the workspace (staged + unstaged),
// i.e. what the current node produced since the last Commit/baseline.
func (w Workspace) Diff(ctx context.Context) (string, error) {
	if _, _, err := w.Run(ctx, "git", "add", "-A"); err != nil {
		return "", err
	}
	out, _, err := w.Run(ctx, "git", "diff", "--cached")
	return out, err
}

// ChangedPaths returns the workspace-relative paths changed since the last
// commit (the current node's edits) — used to verify only what the agent
// touched, not the whole tree.
func (w Workspace) ChangedPaths(ctx context.Context) ([]string, error) {
	if _, _, err := w.Run(ctx, "git", "add", "-A"); err != nil {
		return nil, err
	}
	out, _, err := w.Run(ctx, "git", "diff", "--cached", "--name-only")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// Commit snapshots the current tree as one node's result, so the next node's
// Diff starts clean.
func (w Workspace) Commit(ctx context.Context, msg string) error {
	if _, _, err := w.Run(ctx, "git", "add", "-A"); err != nil {
		return err
	}
	out, code, err := w.Run(ctx, "git",
		"-c", "user.email=myaudit@local", "-c", "user.name=myIntern", "commit", "-q", "-m", msg)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("git commit: %s", out)
	}
	return nil
}

// ensureGitBaseline makes the workspace a single clean commit of the imported
// tree, so a node's Diff starts from an empty delta regardless of the source
// repo's own git state.
func ensureGitBaseline(ctx context.Context, dir string) error {
	_ = os.RemoveAll(filepath.Join(dir, ".git"))
	steps := [][]string{
		{"-C", dir, "init", "-q"},
		{"-C", dir, "add", "-A"},
		{"-C", dir, "-c", "user.email=myaudit@local", "-c", "user.name=myaudit", "commit", "-q", "-m", "import baseline"},
	}
	for _, args := range steps {
		if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %v: %s", args, err, out)
		}
	}
	return nil
}

// copyDir recursively copies src into dst, skipping excludeDirs by name.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if rel != "." && excludeDirs[info.Name()] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		return copyFile(p, filepath.Join(dst, rel), info)
	})
}

func copyFile(src, dst string, info os.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
