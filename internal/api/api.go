// Package api exposes read-only HTTP endpoints over RunState for the UI, plus
// the embedded static page.
package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"myaudit/internal/events"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
	"myaudit/internal/worker"
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
		// Default 200 newest events; ?events=N (capped) lets Activity/API callers
		// pull more history when a long run blows past the default.
		evLimit := 200
		if n, err := strconv.Atoi(r.URL.Query().Get("events")); err == nil && n > 0 {
			evLimit = min(n, 5000)
		}
		events, err := s.EventsForRun(r.Context(), id, evLimit)
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
		run, err := s.GetRun(r.Context(), id)
		if err != nil {
			run = store.RunSummary{ID: id} // fall back to a bare id if the row is gone
		}
		writeJSON(w, RunDetail{Run: run, Nodes: nodes, Events: events, Checkpoints: checkpoints, Files: files, CostUSD: cost})
	})

	mux.HandleFunc("POST /api/runs", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RepoPath  string  `json:"repo_path"`
			Project   string  `json:"project"`
			AuditOnly bool    `json:"audit_only"`
			BudgetUSD float64 `json:"budget_usd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RepoPath == "" {
			http.Error(w, "repo_path required", 400)
			return
		}
		// A relative path resolves against the SERVER's cwd (the myAudit repo), not
		// the user's — silently the wrong thing. Require absolute + give actionable
		// errors for the common mistakes.
		if !filepath.IsAbs(body.RepoPath) {
			http.Error(w, "repo_path must be an absolute path (e.g. /Users/you/project)", 400)
			return
		}
		if info, err := os.Stat(body.RepoPath); err != nil {
			http.Error(w, "no such directory: "+body.RepoPath, 400)
			return
		} else if !info.IsDir() {
			http.Error(w, body.RepoPath+" is not a directory", 400)
			return
		}
		project := body.Project
		if project == "" {
			project = filepath.Base(strings.TrimRight(body.RepoPath, "/"))
		}
		importSpec := map[string]any{"repo_path": body.RepoPath}
		if body.AuditOnly {
			importSpec["audit_only"] = true
		}
		if body.BudgetUSD > 0 {
			importSpec["budget_usd"] = body.BudgetUSD
		}
		// The audit graph starts tiny and grows itself (a living board): import the
		// repo, then map it into modules. The map step spawns one qa card per module
		// at runtime; each qa card files bug tickets, and each bug is an autonomous
		// dev fix — all added dynamically, so the board fills as work is discovered.
		//   import → map ─┬→ qa(module A) ─→ bug… (dev fixes, blocked until QA)
		//                 └→ qa(module B) ─→ bug…
		specs := []store.TaskSpec{
			{Key: "import", Type: "import", Spec: importSpec},
			{Key: "map", Type: "map", DepKeys: []string{"import"}},
		}
		run, _, err := s.CreateGraph(r.Context(), project, specs)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(map[string]string{"id": run.String()})
	})

	// Absolute path to the bundled demo repo (F4 sample run).
	mux.HandleFunc("GET /api/demo", func(w http.ResponseWriter, r *http.Request) {
		p, err := filepath.Abs("demo")
		if err != nil {
			http.Error(w, "demo repo not found — run from the myAudit project root", 404)
			return
		}
		if info, err := os.Stat(p); err != nil || !info.IsDir() {
			http.Error(w, "demo repo not found — run from the myAudit project root", 404)
			return
		}
		writeJSON(w, map[string]string{"path": p})
	})

	// Stop a running audit: mark queued nodes + the run cancelled (no new work),
	// and interrupt the in-flight node's agent.
	mux.HandleFunc("POST /api/runs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		if err := s.CancelRun(r.Context(), id); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		worker.CancelRun(id.String())
		events.New(s.DB()).Log(r.Context(), events.Event{RunID: id, Kind: "run.cancelled", Msg: "audit cancelled by user"})
		w.WriteHeader(200)
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

	// Raw bytes of a workspace file (for images like QA preview screenshots, which
	// can't ride the JSON /file endpoint). Content-type is sniffed from the bytes.
	mux.HandleFunc("GET /api/runs/{id}/raw", func(w http.ResponseWriter, r *http.Request) {
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
		http.ServeFile(w, r, filepath.Join("runs", id.String(), clean))
	})

	// Unified diff of what the audit changed (optionally one path) vs the import
	// baseline — how a user reviews an autonomous fix.
	mux.HandleFunc("GET /api/runs/{id}/diff", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		path := r.URL.Query().Get("path")
		if path != "" {
			clean := filepath.Clean(path)
			if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
				http.Error(w, "bad path", 400)
				return
			}
			path = clean
		}
		ws := sandbox.Workspace{Dir: filepath.Join("runs", id.String())}
		diff, err := ws.DiffFromBaseline(r.Context(), path)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]string{"path": path, "diff": diff})
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
		_ = os.MkdirAll(filepath.Dir(full), 0o755) // allow saving into a new nested path
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

	mux.HandleFunc("GET /api/runs/{id}/flows", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		raw, err := s.FlowsForRun(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		pending, _ := s.HasPendingFlows(r.Context(), id)
		w.Header().Set("content-type", "application/json")
		if len(raw) == 0 {
			fmt.Fprintf(w, `{"ready":false,"pending":%t}`, pending)
			return
		}
		fmt.Fprintf(w, `{"ready":true,"pending":%t,"flows":%s}`, pending, raw)
	})

	mux.HandleFunc("POST /api/runs/{id}/flows", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		if pending, _ := s.HasPendingFlows(r.Context(), id); pending {
			w.WriteHeader(202)
			return
		}
		if _, err := s.AddNodeFull(r.Context(), id, "flows", nil,
			map[string]any{"title": "Flows · data & product", "tags": []string{"flows"}}, "ready"); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(202)
	})

	mux.HandleFunc("GET /api/runs/{id}/live", liveHandler)

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

	registerFileOps(mux, s)
	registerChat(mux, s)
	registerExport(mux, s)
	registerHealth(mux)

	if static != nil {
		mux.Handle("GET /", static)
	}
	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
