package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

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

func registerFileOps(mux *http.ServeMux, s *store.Store) {

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

	mux.HandleFunc("POST /api/runs/{id}/file/new", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Path string `json:"path"`
			Dir  bool   `json:"dir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
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

	mux.HandleFunc("POST /api/runs/{id}/file/rename", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			From string `json:"from"`
			To   string `json:"to"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
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

func searchWorkspace(root, q string, max int) []searchHit {
	needle := strings.ToLower(q)
	out := []searchHit{}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
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
		if info.Size() > 512*1024 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil || isBinary(b) {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(strings.ToLower(line), needle) {
				out = append(out, searchHit{Path: filepath.ToSlash(rel), Line: i + 1, Text: trim(line)})
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
