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
		{Key: "map", Type: "map", DepKeys: []string{"import"}},
		{Key: "qa", Type: "qa", DepKeys: []string{"map"}, Spec: map[string]any{"module": "auth", "path": "src/auth", "title": "QA · auth", "tags": []string{"qa", "module:auth"}}},
	})
	if err != nil {
		slog.Error("graph", "err", err)
		os.Exit(1)
	}
	set := func(key, status string) {
		s.DB().ExecContext(ctx, `UPDATE nodes SET status=? WHERE id=?`, status, ids[key])
	}
	set("import", "done")
	set("map", "done")
	set("qa", "running")

	_, _ = s.CreateBug(ctx, run, store.Bug{
		Title: "password compared with == (timing leak)", File: "src/auth/login.go:42",
		Severity: "high", Priority: "P0", Detail: "Use a constant-time compare.",
		Tags: []string{"from:qa", "module:auth", "high"},
	}, ids["qa"])

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
	ev("map", "map.module", "auth → qa card")
	ev("qa", "node.start", "QA · auth — exercising the module")
	ev("qa", "finding", "auth: password compared with == (timing leak) — high")

	_ = s.PutNotes(ctx, run, "# Audit map\n\nDemo notes: a small web app with a login flow.\n")
	fmt.Println("seeded run:", run)
}
