// Command seed creates a demo run so the UI has something to show.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/uuid"

	"myaudit/internal/config"
	"myaudit/internal/events"
	"myaudit/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()
	s, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		slog.Error("store", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	run, ids, err := s.CreateGraph(ctx, "demo-repo", []store.TaskSpec{
		{Key: "import", Type: "import", Spec: map[string]any{"repo_path": "/path/to/demo-repo"}},
		{Key: "understand", Type: "understand", DepKeys: []string{"import"}},
		{Key: "testgen", Type: "testgen", DepKeys: []string{"understand"}},
		{Key: "verify", Type: "verify", DepKeys: []string{"testgen"}},
		{Key: "review", Type: "review", DepKeys: []string{"understand"}},
	})
	if err != nil {
		slog.Error("graph", "err", err)
		os.Exit(1)
	}
	set := func(key, status string) {
		s.DB().ExecContext(ctx, `UPDATE nodes SET status=? WHERE id=?`, status, ids[key])
	}
	set("import", "done")
	set("understand", "done")
	set("testgen", "running")
	set("verify", "pending")
	set("review", "running")

	log := events.New(s.DB())
	ev := func(node, kind, msg string) {
		var np *uuid.UUID
		if node != "" {
			id := ids[node]
			np = &id
		}
		log.Log(ctx, events.Event{RunID: run, NodeID: np, Kind: kind, Msg: msg})
	}
	ev("", "run.start", "demo-repo — importing codebase")
	ev("import", "node.end", "copied repo into workspace")
	ev("understand", "node.start", "reading codebase")
	ev("understand", "understand.done", "wrote understanding + flows to notes")
	ev("testgen", "node.start", "writing a test for the login flow")
	ev("review", "finding", "auth: password compared with == (timing leak) — high")

	_ = s.PutNotes(ctx, run, "# Understanding\n\nDemo notes: a small web app with a login flow.\n")
	fmt.Println("seeded run:", run)
}
