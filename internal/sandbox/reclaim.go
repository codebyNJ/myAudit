package sandbox

import (
	"os"
	"path/filepath"
)

// heavyDirs are dependency/build caches a finished run no longer needs. They are
// re-creatable (`npm ci`, `pip install`, a rebuild) and dominate workspace size:
// a single Next.js run workspace is ~580MB, of which ~505MB is node_modules.
var heavyDirs = map[string]bool{
	"node_modules":  true,
	".next":         true,
	"dist":          true,
	"build":         true,
	"target":        true,
	".venv":         true,
	"venv":          true,
	"__pycache__":   true,
	".pytest_cache": true,
	".turbo":        true,
	".cache":        true,
	".gradle":       true,
}

// Reclaim deletes dependency and build caches from a finished run's workspace,
// returning the bytes freed. Source, tests and audit artifacts are untouched, so
// the workspace stays reviewable in the UI.
func Reclaim(dir string) (int64, error) {
	var freed int64
	var targets []string

	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries rather than aborting the sweep
		}
		if !d.IsDir() {
			return nil
		}
		if p != dir && heavyDirs[d.Name()] {
			targets = append(targets, p)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	for _, t := range targets {
		freed += dirSize(t)
		if rerr := os.RemoveAll(t); rerr != nil && err == nil {
			err = rerr
		}
	}
	return freed, err
}

func dirSize(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			n += info.Size()
		}
		return nil
	})
	return n
}
