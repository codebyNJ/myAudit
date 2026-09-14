package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/events"
	"github.com/codebyNJ/myAudit/internal/publish"
	"github.com/codebyNJ/myAudit/internal/store"
)

func registerPublish(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("POST /api/runs/{id}/nodes/{nid}/push-pr", func(w http.ResponseWriter, r *http.Request) {
		runID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad run id", 400)
			return
		}
		nodeID, err := uuid.Parse(r.PathValue("nid"))
		if err != nil {
			http.Error(w, "bad node id", 400)
			return
		}

		var nodeType string
		if err := s.DB().QueryRowContext(r.Context(),
			`SELECT type FROM nodes WHERE id=? AND run_id=?`, nodeID, runID).Scan(&nodeType); err != nil || nodeType != "bug" {
			http.Error(w, "not a bug ticket", 404)
			return
		}

		out, err := s.NodeOutput(r.Context(), nodeID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if prURL, _ := out["pr_url"].(string); prURL != "" {
			writeJSON(w, map[string]string{"pr_url": prURL, "branch": strVal(out["branch"])})
			return
		}
		commitSHA, _ := out["commit_sha"].(string)
		if commitSHA == "" {
			http.Error(w, "ticket has no fix commit to publish", 400)
			return
		}

		opts := s.RunOptsFor(r.Context(), runID)
		if opts.RepoPath == "" {
			http.Error(w, "run has no source repo path", 400)
			return
		}

		var title string
		var snap []byte
		_ = s.DB().QueryRowContext(r.Context(),
			`SELECT COALESCE(input_snapshot,'{}') FROM nodes WHERE id=?`, nodeID).Scan(&snap)
		var bug map[string]any
		_ = json.Unmarshal(snap, &bug)
		if t, _ := bug["title"].(string); t != "" {
			title = t
		}

		res, err := publish.PublishFix(r.Context(), publish.Options{
			RunID:      runID,
			NodeID:     nodeID,
			RepoPath:   opts.RepoPath,
			Git:        opts.Git,
			SandboxDir: filepath.Join("runs", runID.String()),
			CommitSHA:  commitSHA,
			Title:      title,
		})
		if err != nil {
			_ = s.MergeNodeOutput(r.Context(), nodeID, map[string]any{"pr_status": "failed"})
			events.New(s.DB()).Log(r.Context(), events.Event{
				RunID: runID, NodeID: &nodeID, Kind: "pr.failed", Msg: err.Error(),
			})
			http.Error(w, err.Error(), 500)
			return
		}

		_ = s.MergeNodeOutput(r.Context(), nodeID, map[string]any{
			"pr_url": res.PRURL, "pr_status": "pushed", "branch": res.Branch,
		})
		events.New(s.DB()).Log(r.Context(), events.Event{
			RunID: runID, NodeID: &nodeID, Kind: "pr.pushed", Msg: res.PRURL,
		})
		w.WriteHeader(201)
		writeJSON(w, res)
	})
}

func strVal(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
