package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)


// wsPath resolves a workspace-relative path to an absolute path under the run's
// workspace, rejecting empty/absolute/escaping paths. ok=false ⇒ already 400'd.
func wsPath(w http.ResponseWriter, r *http.Request, rel string) (id uuid.UUID, full string, ok bool) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "bad id", 400)
		return id, "", false
	}
	clean := filepath.Clean(rel)
	if rel == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		http.Error(w, "bad path", 400)
		return id, "", false
	}
	return id, filepath.Join("runs", id.String(), clean), true
}

// registerFileOps adds VSCode-style file management + find-in-files over a run's
// workspace: search, create, rename, delete.
func registerFileOps(mux *http.ServeMux, s *store.Store) {
	// Find-in-files: substring match across the workspace. Skips excluded/binary/
	// large files; caps results so a huge repo can't blow up the response.
	mux.HandleFunc("GET /api/runs/{id}/search", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		q := r.URL.Query().Get("q")
		if len(strings.TrimSpace(q)) < 2 {
			writeJSON(w, []searchHit{})
			return
		}
		writeJSON(w, searchWorkspace(filepath.Join("runs", id.String()), q, 200))
	})

	// Create a file (or folder with {"dir":true}). Parents are created.
	mux.HandleFunc("POST /api/runs/{id}/file/new", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Path string `json:"path"`
			Dir  bool   `json:"dir"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		_, full, ok := wsPath(w, r, b.Path)
		if !ok {
			return
		}
		if _, err := os.Stat(full); err == nil {
			http.Error(w, "already exists", 409)
			return
		}
		if b.Dir {
			if err := os.MkdirAll(full, 0o755); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if err := os.WriteFile(full, nil, 0o644); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		w.WriteHeader(201)
	})

	// Rename/move a file or folder within the workspace.
	mux.HandleFunc("POST /api/runs/{id}/file/rename", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		id, from, ok := wsPath(w, r, b.From)
		if !ok {
			return
		}
		toClean := filepath.Clean(b.To)
		if b.To == "" || strings.HasPrefix(toClean, "..") || filepath.IsAbs(toClean) {
			http.Error(w, "bad path", 400)
			return
		}
		to := filepath.Join("runs", id.String(), toClean)
		if _, err := os.Stat(to); err == nil {
			http.Error(w, "target exists", 409)
			return
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if err := os.Rename(from, to); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(200)
	})

	// Delete a file or folder (recursive).
	mux.HandleFunc("DELETE /api/runs/{id}/file", func(w http.ResponseWriter, r *http.Request) {
		_, full, ok := wsPath(w, r, r.URL.Query().Get("path"))
		if !ok {
			return
		}
		if err := os.RemoveAll(full); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(200)
	})
}

type searchHit struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// searchWorkspace does a case-insensitive substring scan, returning up to max
// hits. Binary and oversized files are skipped; long lines are trimmed.
func searchWorkspace(root, q string, max int) []searchHit {
	needle := strings.ToLower(q)
	out := []searchHit{}
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || len(out) >= max {
			if len(out) >= max {
				return filepath.SkipAll
			}
			return nil
		}
		if info.IsDir() {
			if p != root && sandbox.SkipDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > 512*1024 { // skip large/generated files
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil || isBinary(b) {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(strings.ToLower(line), needle) {
				out = append(out, searchHit{Path: rel, Line: i + 1, Text: trim(line)})
				if len(out) >= max {
					break
				}
			}
		}
		return nil
	})
	return out
}

func isBinary(b []byte) bool {
	n := len(b)
	if n > 1024 {
		n = 1024
	}
	for i := 0; i < n; i++ {
		if b[i] == 0 {
			return true
		}
	}
	return false
}

func trim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
