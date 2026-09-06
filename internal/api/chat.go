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

		// "fix" intent: enqueue open/failed/parked tickets for the autonomous dev
		// loop so the work is tracked on the board — the fix isn't just described.
		var reply string
		if isFixIntent(b.Message) {
			reply = enqueueFixes(r.Context(), s, id)
		} else {
			reply = chatReply(r.Context(), s, id, b.Message)
		}
		log.Log(context.Background(), events.Event{RunID: uid, Kind: "chat.assistant", Msg: reply})
		writeJSON(w, map[string]string{"reply": reply})
	})

	// Re-run the fix for one ticket (a Failed or parked-in-Review card): flip it
	// back to 'ready' so the autonomous dev loop picks it up again, tracked.
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

	// Manual triage: edit a ticket's severity/priority and/or move its status
	// (e.g. dismissed). Snapshot fields merge into input_snapshot; status is the
	// node status column.
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
		json.NewDecoder(r.Body).Decode(&b)
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
		// Allowlisted manual status moves (dismiss / restore).
		if b.Status != "" {
			ok := map[string]bool{"open": true, "dismissed": true, "in_review": true}
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

// isFixIntent detects a "go fix things" command (vs. a question). Kept narrow and
// predictable: a message that starts with "fix" is a command to enqueue tickets.
func isFixIntent(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	return strings.HasPrefix(m, "fix")
}

// enqueueFixes flips every open/failed/parked ticket back to 'ready' so the
// autonomous dev loop picks them up — the fix is done AND tracked on the board.
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

// chatTimeout bounds one chat turn (a live agent may install/run things).
const chatTimeout = 10 * time.Minute

// chatReply is a real Claude Code turn over the workspace: full tools (Bash
// included), grounded on the run's notes — the chat is the same agent as the rest
// of the IDE, so it can actually act, not just describe. Stub when REAL_CLAUDE is
// unset (free dev mode).
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

	// Chat is a live (Bash) agent like the rest of the IDE, so it gets the same
	// guardrails: a timeout so a hung turn can't hang forever, and it honors
	// AGENT_ISOLATE (previously chat silently escaped the container jail).
	ctx, cancel := context.WithTimeout(ctx, chatTimeout)
	defer cancel()
	image := os.Getenv("AGENT_IMAGE")
	if image == "" {
		image = "myaudit-sandbox"
	}
	res, err := agent.Run(ctx, ws, task, agent.Options{
		Model:   resolveModel(ctx, s),
		Allow:   agent.LiveAllow,
		Deny:    agent.LiveDeny,
		Isolate: os.Getenv("AGENT_ISOLATE") != "",
		Image:   image,
	})
	if err != nil {
		return "chat failed: " + err.Error()
	}
	if !res.OK && res.Summary == "" {
		return "chat error: " + res.Err
	}
	return res.Summary
}
