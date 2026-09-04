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

	run, ids, err := s.CreateGraph(ctx, "acme-saas", []store.TaskSpec{
		{Key: "scaffold", Type: "scaffold"},
		{Key: "schema", Type: "implement", DepKeys: []string{"scaffold"}},
		{Key: "auth", Type: "implement", DepKeys: []string{"schema"}},
		{Key: "landing", Type: "implement", DepKeys: []string{"auth"}},
	})
	if err != nil {
		slog.Error("graph", "err", err)
		os.Exit(1)
	}
	set := func(key, status string) {
		s.DB().ExecContext(ctx, `UPDATE nodes SET status=? WHERE id=?`, status, ids[key])
	}
	set("scaffold", "done")
	set("schema", "done")
	set("auth", "running")
	set("landing", "pending")

	log := events.New(s.DB())
	ev := func(node, kind, msg string) {
		var np *uuid.UUID
		if node != "" {
			id := ids[node]
			np = &id
		}
		log.Log(ctx, events.Event{RunID: run, NodeID: np, Kind: kind, Msg: msg})
	}
	ev("", "run.start", "acme-saas — Fastify + MongoDB")
	ev("scaffold", "node.start", "init-project.js")
	ev("scaffold", "node.end", "env seeded, git reset")
	ev("schema", "node.start", "workspaces + RBAC models")
	ev("schema", "node.end", "GREEN 5/5")
	ev("auth", "node.start", "JWT + Google OAuth")
	ev("auth", "gate.red", "genuine: reset.expired failed on empty impl")
	ev("auth", "checkpoint.raise", "Confirm Google-only sign-in?")

	fmt.Println("seeded run:", run)
}
