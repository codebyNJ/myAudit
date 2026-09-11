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

func (w Workspace) DiffFromBaseline(ctx context.Context, path string) (string, error) {
	base, _, err := w.Run(ctx, "git", "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(base)
	if len(fields) == 0 {
		return "", nil
	}
	args := []string{"diff", fields[0]}
	if path != "" {
		args = append(args, "--", path)
	}
	out, _, err := w.Run(ctx, "git", args...)
	return out, err
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
			return fmt.Errorf("git %v: %v: %s", args, err, out)
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(p)
			if err != nil {
				return nil
			}
			_ = os.MkdirAll(filepath.Dir(filepath.Join(dst, rel)), 0o755)
			_ = os.Symlink(target, filepath.Join(dst, rel))
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
			return nil
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
