package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/events"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

func registerChat(mux *http.ServeMux, s *store.Store) {

	mux.HandleFunc("POST /api/runs/{id}/chat", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		var b struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		if strings.TrimSpace(b.Message) == "" {
			http.Error(w, "message required", 400)
			return
		}
		log := events.New(s.DB())
		uid := id
		log.Log(r.Context(), events.Event{RunID: id, Kind: "chat.user", Msg: b.Message})

		var reply string
		if isFixIntent(b.Message) {
			reply = enqueueFixes(r.Context(), s, id)
		} else {
			reply = chatReply(r.Context(), s, id, b.Message)
		}
		log.Log(context.Background(), events.Event{RunID: uid, Kind: "chat.assistant", Msg: reply})
		writeJSON(w, map[string]string{"reply": reply})
	})

	mux.HandleFunc("POST /api/runs/{id}/nodes/{nid}/enqueue", func(w http.ResponseWriter, r *http.Request) {
		nid, err := uuid.Parse(r.PathValue("nid"))
		if err != nil {
			http.Error(w, "bad node id", 400)
			return
		}
		if err := s.SetNodeStatus(r.Context(), nid, "ready"); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(202)
	})

	mux.HandleFunc("PATCH /api/runs/{id}/nodes/{nid}", func(w http.ResponseWriter, r *http.Request) {
		nid, err := uuid.Parse(r.PathValue("nid"))
		if err != nil {
			http.Error(w, "bad node id", 400)
			return
		}
		var b struct {
			Severity string `json:"severity"`
			Priority string `json:"priority"`
			Status   string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		patch := map[string]any{}
		if b.Severity == "high" || b.Severity == "medium" || b.Severity == "low" {
			patch["severity"] = b.Severity
		}
		if b.Priority == "P0" || b.Priority == "P1" || b.Priority == "P2" {
			patch["priority"] = b.Priority
		}
		if len(patch) > 0 {
			if err := s.MergeNodeSnapshot(r.Context(), nid, patch); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}

		if b.Status != "" {
			ok := map[string]bool{"open": true, "dismissed": true, "in_review": true, "done": true}
			if !ok[b.Status] {
				http.Error(w, "status not allowed", 400)
				return
			}
			if err := s.SetNodeStatus(r.Context(), nid, b.Status); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		w.WriteHeader(200)
	})

	mux.HandleFunc("POST /api/runs/{id}/nodes/{nid}/tags", func(w http.ResponseWriter, r *http.Request) {
		nid, err := uuid.Parse(r.PathValue("nid"))
		if err != nil {
			http.Error(w, "bad node id", 400)
			return
		}
		var b struct {
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		if err := s.SetNodeTags(r.Context(), nid, b.Tags); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(200)
	})
}

func isFixIntent(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	return strings.HasPrefix(m, "fix")
}

func enqueueFixes(ctx context.Context, s *store.Store, run uuid.UUID) string {
	ids, err := s.ReopenableBugs(ctx, run)
	if err != nil {
		return "couldn't queue fixes: " + err.Error()
	}
	if len(ids) == 0 {
		return "Nothing to fix — no open, failed, or in-review tickets on the board right now."
	}
	for _, id := range ids {
		_ = s.SetNodeStatus(ctx, id, "ready")
	}
	return fmt.Sprintf("Queued %d ticket(s) for the autonomous dev — they'll move to **In progress** on the board, get a root-cause fix + regression, and auto-close on green. Watch the board.", len(ids))
}

const chatTimeout = 10 * time.Minute

func chatReply(ctx context.Context, s *store.Store, run uuid.UUID, msg string) string {
	if os.Getenv("REAL_CLAUDE") == "" {
		return "[stub] chat is disabled in dev mode. Run with the real agent (make run) to chat about the code."
	}
	notes, _ := s.GetNotes(ctx, run)
	ws := sandbox.Workspace{Dir: filepath.Join("runs", run.String())}
	task := "You are the engineer working inside this codebase's workspace. You have full tools " +
		"(read, edit, and a shell). Help the developer: answer questions about the code, and when they ask " +
		"you to change or fix something, do it directly in the workspace.\n\n"
	if strings.TrimSpace(notes) != "" {
		task += "Prior analysis + audit log for this codebase:\n" + notes + "\n\n"
	}
	task += "Developer: " + msg + "\n\nRespond concisely; if you changed files, say what and why."

	ctx, cancel := context.WithTimeout(ctx, chatTimeout)
	defer cancel()
	image := os.Getenv("AGENT_IMAGE")
	if image == "" {
		image = "myaudit-sandbox"
	}
	res, err := runAgent(ctx, s, ws, task, agent.Live, os.Getenv("AGENT_ISOLATE") != "", image, os.Getenv("CLAUDE_BIN"), os.Getenv("OPENCODE_BIN"), nil)
	if err != nil {
		return "chat failed: " + err.Error()
	}
	if !res.OK && res.Summary == "" {
		return "chat error: " + res.Err
	}
	return res.Summary
}
