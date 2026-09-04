// Package sandbox creates a per-run project workspace by copying the
// battle-tested template and running its own deterministic init script. No
// model is involved here — this is the $0 scaffold path that produces the
// entire base app (auth, tenancy, workspaces, RBAC, cache, frontend).
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

// Workspace is a scaffolded project directory for one run.
type Workspace struct{ Dir string }

// excludeDirs are skipped when copying the template — VCS metadata and heavy
// build artifacts we never want in a fresh scaffold.
var excludeDirs = map[string]bool{".git": true, "node_modules": true, "dist": true, "build": true, ".next": true}

// Scaffold copies TEMPLATE_PATH into <root>/<runID> (skipping excludeDirs), then
// runs the template's own scripts/init-project.js to rewrite the default
// identity (Acme→name / acme→slug), seed backend/.env, and initialize git.
// Returns the workspace with a committed HEAD baseline for per-node diffs.
func Scaffold(ctx context.Context, root, runID, name, slug string) (Workspace, error) {
	tmpl := os.Getenv("TEMPLATE_PATH")
	if tmpl == "" {
		return Workspace{}, fmt.Errorf("TEMPLATE_PATH not set")
	}
	dir := filepath.Join(root, runID)
	if err := copyDir(tmpl, dir); err != nil {
		return Workspace{}, fmt.Errorf("copy template: %w", err)
	}
	// The template ships a zero-dependency initializer; run it in the copy.
	cmd := exec.CommandContext(ctx, "node", "scripts/init-project.js", name, slug)
	cmd.Dir = dir
	cmd.Stdin = nil
	if out, err := cmd.CombinedOutput(); err != nil {
		return Workspace{}, fmt.Errorf("init-project: %v: %s", err, out)
	}
	// init-project resets git and commits, but git identity may be unset (CI).
	// Guarantee a committed HEAD so per-node git diffs have a baseline.
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
		"-c", "user.email=myintern@local", "-c", "user.name=myIntern", "commit", "-q", "-m", msg)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("git commit: %s", out)
	}
	return nil
}

// ensureGitBaseline makes the workspace a single clean commit of the exact
// post-scaffold tree. init-project.js commits and then deletes itself, leaving
// a dirty tree; rather than depend on its git state, we reset to one baseline
// commit so a fresh scaffold has an empty diff.
func ensureGitBaseline(ctx context.Context, dir string) error {
	_ = os.RemoveAll(filepath.Join(dir, ".git"))
	steps := [][]string{
		{"-C", dir, "init", "-q"},
		{"-C", dir, "add", "-A"},
		{"-C", dir, "-c", "user.email=myintern@local", "-c", "user.name=myIntern", "commit", "-q", "-m", "scaffold baseline"},
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
