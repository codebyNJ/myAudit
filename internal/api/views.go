package api

import (
	"os"
	"path/filepath"
	"sort"

	"myaudit/internal/store"
)

// skipTreeDir are directories excluded from the workspace file tree.
var skipTreeDir = map[string]bool{".git": true, "node_modules": true, "dist": true, "build": true, ".next": true}

// listWorkspaceFiles walks a run's on-disk workspace and returns every file,
// sorted by path, flagging changed files and carrying review status. Rejected
// files are omitted. Content is not read here (fetched per-file by the UI).
func listWorkspaceFiles(runID string, changed map[string]bool, reviews map[string]string) []store.FileEntry {
	root := filepath.Join("runs", runID)
	out := []store.FileEntry{}
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if p != root && skipTreeDir[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil || reviews[rel] == "rejected" {
			return nil
		}
		out = append(out, store.FileEntry{Path: rel, Changed: changed[rel], Review: reviews[rel]})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
