package api

import (
	"os"
	"path/filepath"
	"sort"

	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

func listWorkspaceFiles(runID string, changed map[string]bool, reviews map[string]string) []store.FileEntry {
	root := filepath.Join("runs", runID)
	out := []store.FileEntry{}
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if p != root && sandbox.SkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)

		rel = filepath.ToSlash(rel)
		if err != nil || reviews[rel] == "rejected" {
			return nil
		}
		out = append(out, store.FileEntry{Path: rel, Changed: changed[rel], Review: reviews[rel]})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
