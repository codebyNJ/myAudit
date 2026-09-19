package demo

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed all:fixture
var fixture embed.FS

var (
	resolveOnce sync.Once
	resolved    string
	resolveErr  error
)

// Dir returns an absolute path to the bundled demo repo on disk.
// Dev clones can still use ./demo at the repo root; shipped binaries use an
// embedded copy extracted under the user cache directory.
func Dir() (string, error) {
	resolveOnce.Do(func() {
		resolved, resolveErr = resolve()
	})
	return resolved, resolveErr
}

func resolve() (string, error) {
	candidates := []string{}
	if p := os.Getenv("MYAUDIT_DEMO"); p != "" {
		candidates = append(candidates, p)
	}
	if p, err := filepath.Abs("demo"); err == nil {
		candidates = append(candidates, p)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "demo"))
	}
	for _, p := range candidates {
		// #nosec G703 -- candidates are built from the executable path and
		// compile-time constants, never from request input.
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p, nil
		}
	}
	return extractToCache()
}

func extractToCache() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(base, "myaudit", "demo")
	if info, err := os.Stat(filepath.Join(dest, "index.js")); err == nil && !info.IsDir() {
		return dest, nil
	}
	if err := os.RemoveAll(dest); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	if err := extractFixture(dest); err != nil {
		return "", err
	}
	return dest, nil
}

func extractFixture(dest string) error {
	return fs.WalkDir(fixture, "fixture", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "fixture" {
			return nil
		}
		rel, ok := strings.CutPrefix(path, "fixture/")
		if !ok {
			return nil
		}
		target := filepath.Join(dest, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(fixture, path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
