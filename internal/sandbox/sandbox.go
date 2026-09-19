package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Workspace struct{ Dir string }

var excludeDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true, ".next": true,
	"target": true, ".venv": true, "venv": true, "__pycache__": true, "vendor": true,
	".svelte-kit": true, "coverage": true, ".gradle": true,
	".vercel": true, ".turbo": true, ".output": true, ".cache": true,
}

func SkipDir(name string) bool { return excludeDirs[name] }

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

// RunOutput runs cmd and reports its combined output and exit code. A command
// that runs and exits non-zero comes back as (output, code, nil) — the caller
// decides whether that is a failure; only an inability to run at all returns a
// non-nil error. errors.As rather than a bare type assertion, so a wrapped
// ExitError is still recognised.
func RunOutput(cmd *exec.Cmd) (string, int, error) {
	b, err := cmd.CombinedOutput()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return string(b), ee.ExitCode(), nil
		}
		return string(b), -1, err
	}
	return string(b), 0, nil
}

func (w Workspace) Run(ctx context.Context, name string, args ...string) (out string, code int, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = w.Dir
	cmd.Stdin = nil
	return RunOutput(cmd)
}

func (w Workspace) Diff(ctx context.Context) (string, error) {
	if _, _, err := w.Run(ctx, "git", "add", "-A"); err != nil {
		return "", err
	}
	out, _, err := w.Run(ctx, "git", "diff", "--cached")
	return out, err
}

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

// baselineCommit is the "import baseline" commit made by ensureGitBaseline.
func (w Workspace) baselineCommit(ctx context.Context) (string, error) {
	out, _, err := w.Run(ctx, "git", "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func (w Workspace) DiffFromBaseline(ctx context.Context, path string) (string, error) {
	base, err := w.baselineCommit(ctx)
	if err != nil || base == "" {
		return "", err
	}
	args := []string{"diff", base}
	if path != "" {
		args = append(args, "--", path)
	}
	out, _, err := w.Run(ctx, "git", args...)
	return out, err
}

// RevertFromBaseline undoes the agent's work on one file: a file that existed
// at import is restored to its imported contents, a file the agent created is
// removed. Deleting unconditionally would destroy the user's original file
// whenever the agent merely modified it.
func (w Workspace) RevertFromBaseline(ctx context.Context, path string) error {
	if _, err := os.Stat(w.Dir); os.IsNotExist(err) {
		return nil // no workspace on disk: nothing to undo
	}
	base, err := w.baselineCommit(ctx)
	if err != nil {
		return err
	}
	if base != "" {
		if _, code, err := w.Run(ctx, "git", "checkout", base, "--", path); err != nil {
			return err
		} else if code == 0 {
			return nil
		}
	}
	// Not in the baseline tree: the agent created it, so removal is the revert.
	if err := os.Remove(filepath.Join(w.Dir, filepath.Clean(path))); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (w Workspace) Commit(ctx context.Context, msg string) error {
	if _, _, err := w.Run(ctx, "git", "add", "-A"); err != nil {
		return err
	}
	out, code, err := w.Run(ctx, "git",
		"-c", "user.email=myaudit@local", "-c", "user.name=myaudit", "commit", "-q", "-m", msg)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("git commit: %s", out)
	}
	return nil
}

func ensureGitBaseline(ctx context.Context, dir string) error {
	_ = os.RemoveAll(filepath.Join(dir, ".git"))
	steps := [][]string{
		{"-C", dir, "init", "-q"},
		{"-C", dir, "add", "-A"},
		{"-C", dir, "-c", "user.email=myaudit@local", "-c", "user.name=myaudit", "commit", "-q", "--allow-empty", "-m", "import baseline"},
	}
	for _, args := range steps {
		if out, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("git %v: %w: %s", args, err, out)
		}
	}
	return nil
}

// vanished reports whether err is just a file that went away mid-walk (the
// source repo may be live). Those are skipped; every other error fails the
// import, because a silently partial copy gets audited as if it were complete.
func vanished(err error) bool { return os.IsNotExist(err) }

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			if vanished(err) {
				return nil
			}
			return fmt.Errorf("read %s: %w", p, err)
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", p, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				if vanished(err) {
					return nil
				}
				return fmt.Errorf("readlink %s: %w", rel, err)
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(dst, rel)), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(target, filepath.Join(dst, rel)); err != nil && !os.IsExist(err) {
				return fmt.Errorf("symlink %s: %w", rel, err)
			}
			return nil
		}
		if info.IsDir() {
			if rel != "." && excludeDirs[info.Name()] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if err := copyFile(p, filepath.Join(dst, rel), info); err != nil {
			if vanished(err) {
				return nil
			}
			return fmt.Errorf("copy %s: %w", rel, err)
		}
		return nil
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
