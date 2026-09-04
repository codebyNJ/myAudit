// Package api exposes read-only HTTP endpoints over RunState for the UI, plus
// the embedded static page.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"myaudit/internal/events"
	"myaudit/internal/store"
)

// RunDetail is a run plus its nodes and event log.
type RunDetail struct {
	Run         store.RunSummary   `json:"run"`
	Nodes       []store.Node       `json:"nodes"`
	Events      []store.EventRow   `json:"events"`
	Checkpoints []store.Checkpoint `json:"checkpoints"`
	Files       []store.FileEntry  `json:"files"`
	CostUSD     float64            `json:"cost_usd"`
}

// NewMux wires the JSON API and the static UI over the store.
func NewMux(s *store.Store, static http.Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/runs", func(w http.ResponseWriter, r *http.Request) {
		runs, err := s.ListRuns(r.Context(), 50)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if runs == nil {
			runs = []store.RunSummary{} // encode empty as [] not null
		}
		writeJSON(w, runs)
	})

	mux.HandleFunc("GET /api/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		nodes, err := s.NodesForRun(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		events, err := s.EventsForRun(r.Context(), id, 200)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		checkpoints, err := s.OpenCheckpointsForRun(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		// Full workspace tree (scaffold + generated), lazily loaded — content is
		// fetched per-file via GET /api/runs/{id}/file. Feature-changed files are
		// flagged so the UI can highlight the deltas.
		changed, err := s.ChangedFilesForRun(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		changedSet := make(map[string]bool, len(changed))
		for _, f := range changed {
			changedSet[f.Path] = true
		}
		reviews, _ := s.Reviews(r.Context(), id)
		files := listWorkspaceFiles(id.String(), changedSet, reviews)
		cost, _ := s.RunCostUSD(r.Context(), id)
		writeJSON(w, RunDetail{Run: store.RunSummary{ID: id}, Nodes: nodes, Events: events, Checkpoints: checkpoints, Files: files, CostUSD: cost})
	})

	mux.HandleFunc("POST /api/runs", func(w http.ResponseWriter, r *http.Request) {
		var cfg struct {
			Project   string           `json:"project"`
			Tenancy   string           `json:"tenancy"` // multi | single (template default: multi)
			Cache     string           `json:"cache"`   // off | shadow | on
			Resources []store.Resource `json:"resources"`
		}
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil || cfg.Project == "" {
			http.Error(w, "project required", 400)
			return
		}
		// New graph: the template already IS the base app (auth, tenancy, workspaces,
		// RBAC, cache, frontend) — produced for $0 by the scaffold node. The agent is
		// invoked only per requested domain resource (the per-project delta).
		//   scaffold(deterministic) → config(deterministic) → feature:<R>(agent)… → finalize
		specs := []store.TaskSpec{
			{Key: "scaffold", Type: "scaffold", Spec: map[string]any{"project": cfg.Project}},
			{Key: "config", Type: "config", DepKeys: []string{"scaffold"},
				Spec: map[string]any{"tenancy": cfg.Tenancy, "cache": cfg.Cache}},
		}
		featureKeys := []string{}
		for _, res := range cfg.Resources {
			if res.Name == "" {
				continue
			}
			key := "feature_" + res.Name
			featureKeys = append(featureKeys, key)
			specs = append(specs, store.TaskSpec{
				Key: key, Type: "feature", DepKeys: []string{"config"}, Spec: res,
			})
		}
		// finalize depends on every feature (or config if none) so it runs last.
		finalizeDeps := featureKeys
		if len(finalizeDeps) == 0 {
			finalizeDeps = []string{"config"}
		}
		specs = append(specs, store.TaskSpec{Key: "finalize", Type: "finalize", DepKeys: finalizeDeps})
		run, _, err := s.CreateGraph(r.Context(), cfg.Project, specs)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": run.String()})
	})

	mux.HandleFunc("POST /api/checkpoints/{id}/resolve", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		var b struct {
			Answer string `json:"answer"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		if err := s.ResolveCheckpoint(r.Context(), id, b.Answer); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(200)
	})

	// One file's content from the run workspace (lazy-loaded by the Dev tab).
	mux.HandleFunc("GET /api/runs/{id}/file", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		p := r.URL.Query().Get("path")
		clean := filepath.Clean(p)
		if p == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			http.Error(w, "bad path", 400)
			return
		}
		b, err := os.ReadFile(filepath.Join("runs", id.String(), clean))
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		writeJSON(w, map[string]string{"path": clean, "content": string(b)})
	})

	// Save edits to a file in the run workspace.
	mux.HandleFunc("PUT /api/runs/{id}/file", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		var b struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		clean := filepath.Clean(b.Path)
		if b.Path == "" || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			http.Error(w, "bad path", 400)
			return
		}
		full := filepath.Join("runs", id.String(), clean)
		if err := os.WriteFile(full, []byte(b.Content), 0o644); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		events.New(s.DB()).Log(r.Context(), events.Event{RunID: id, Kind: "file.edit", Msg: clean})
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/runs/{id}/review", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		var b struct {
			Path   string `json:"path"`
			Status string `json:"status"` // accepted | rejected
		}
		json.NewDecoder(r.Body).Decode(&b)
		if b.Path == "" || (b.Status != "accepted" && b.Status != "rejected") {
			http.Error(w, "path and status (accepted|rejected) required", 400)
			return
		}
		if err := s.SetReview(r.Context(), id, b.Path, b.Status); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if b.Status == "rejected" {
			// remove the generated file from the run workspace
			clean := filepath.Clean(b.Path)
			if !strings.HasPrefix(clean, "..") && !filepath.IsAbs(clean) {
				os.Remove(filepath.Join("runs", id.String(), clean))
			}
		}
		events.New(s.DB()).Log(r.Context(), events.Event{RunID: id, Kind: "review." + b.Status, Msg: b.Path})
		w.WriteHeader(200)
	})

	mux.HandleFunc("GET /api/runs/{id}/board", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		cards, err := s.NodeDetailsForRun(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if cards == nil {
			cards = []store.NodeDetail{}
		}
		slog.Info("board.read", "run", id, "cards", len(cards))
		writeJSON(w, cards)
	})

	mux.HandleFunc("GET /api/runs/{id}/notes", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		n, err := s.GetNotes(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"content": n})
	})

	mux.HandleFunc("PUT /api/runs/{id}/notes", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		var b struct {
			Content string `json:"content"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		if err := s.PutNotes(r.Context(), id, b.Content); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/runs/{id}/steer", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		var b struct {
			Message string `json:"message"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		if b.Message == "" {
			http.Error(w, "message required", 400)
			return
		}
		events.New(s.DB()).Log(r.Context(), events.Event{RunID: id, Kind: "steer", Msg: b.Message})
		w.WriteHeader(202)
	})

	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		st, err := s.GetSettings(r.Context())
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, st)
	})

	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var patch map[string]any
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			http.Error(w, "bad body", 400)
			return
		}
		merged, err := s.PutSettings(r.Context(), patch)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, merged)
	})

	if static != nil {
		mux.Handle("GET /", static)
	}
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
