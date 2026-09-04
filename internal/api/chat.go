package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"myaudit/internal/agent"
	"myaudit/internal/events"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
)

// registerChat adds the conversational chat endpoint and manual card tagging.
func registerChat(mux *http.ServeMux, s *store.Store) {
	// Conversational chat: the user's message is answered by claude -p run
	// read-only over the imported workspace + the run's notes. Both turns are
	// persisted as events (chat.user / chat.assistant) so the thread survives
	// reloads and shows on every page.
	mux.HandleFunc("POST /api/runs/{id}/chat", func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "bad id", 400)
			return
		}
		var b struct {
			Message string `json:"message"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		if strings.TrimSpace(b.Message) == "" {
			http.Error(w, "message required", 400)
			return
		}
		log := events.New(s.DB())
		uid := id
		log.Log(r.Context(), events.Event{RunID: id, Kind: "chat.user", Msg: b.Message})

		reply := chatReply(r.Context(), s, id, b.Message)
		log.Log(context.Background(), events.Event{RunID: uid, Kind: "chat.assistant", Msg: reply})
		writeJSON(w, map[string]string{"reply": reply})
	})

	// Manual tagging: replace a card's tags (bug ticket or audit node).
	mux.HandleFunc("POST /api/runs/{id}/nodes/{nid}/tags", func(w http.ResponseWriter, r *http.Request) {
		nid, err := uuid.Parse(r.PathValue("nid"))
		if err != nil {
			http.Error(w, "bad node id", 400)
			return
		}
		var b struct {
			Tags []string `json:"tags"`
		}
		json.NewDecoder(r.Body).Decode(&b)
		if err := s.SetNodeTags(r.Context(), nid, b.Tags); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(200)
	})
}

// chatReply grounds a read-only claude turn on the run's notes + the imported
// workspace. Falls back to a stub when REAL_CLAUDE is unset (free dev mode).
func chatReply(ctx context.Context, s *store.Store, run uuid.UUID, msg string) string {
	if os.Getenv("REAL_CLAUDE") == "" {
		return "[stub] chat is disabled in dev mode. Run with the real agent (make run) to chat about the code."
	}
	notes, _ := s.GetNotes(ctx, run)
	ws := sandbox.Workspace{Dir: filepath.Join("runs", run.String())}
	task := "You are helping a developer understand and audit this codebase. " +
		"Answer their question concisely using the code in this workspace. Do NOT modify any files.\n\n"
	if strings.TrimSpace(notes) != "" {
		task += "Prior analysis of this codebase:\n" + notes + "\n\n"
	}
	task += "Developer's question: " + msg + "\n\nYour answer:"

	res, err := agent.Run(ctx, ws, task, agent.Options{
		Model: os.Getenv("CLAUDE_MODEL"),
		Allow: agent.ReadOnlyAllow,
		Deny:  agent.ReadOnlyDeny,
	})
	if err != nil {
		return "chat failed: " + err.Error()
	}
	if !res.OK && res.Summary == "" {
		return "chat error: " + res.Err
	}
	return res.Summary
}
