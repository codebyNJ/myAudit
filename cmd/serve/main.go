// Command serve exposes the run API + UI over Postgres.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"myintern/internal/api"
	"myintern/internal/config"
	"myintern/internal/store"
	"myintern/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	s, err := store.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("store", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	// Drive ready runs so the UI shows live progression. REAL_CLAUDE=1 drives
	// feature nodes with the real Claude Code agent; default is the $0 stub agent
	// (scaffold still runs for real — it's deterministic and free).
	var deps worker.Deps
	if os.Getenv("REAL_CLAUDE") != "" {
		deps = api.NewRealDeps(s)
		slog.Info("run loop: REAL claude-code agent mode")
	} else {
		deps = api.NewStubDeps(s)
		slog.Info("run loop: stub mode ($0 agent; scaffold real)")
	}
	// Pace between processed nodes: real agent calls are throttled a little to
	// avoid bursting the request-rate limit; the stub is effectively instant.
	// Override with RUN_PACE_MS.
	pace := time.Second
	if os.Getenv("REAL_CLAUDE") != "" {
		pace = 2 * time.Second
	}
	if ms, _ := strconv.Atoi(os.Getenv("RUN_PACE_MS")); ms > 0 {
		pace = time.Duration(ms) * time.Millisecond
	}
	go api.StartRunLoop(context.Background(), deps, pace)

	addr := ":7788"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	slog.Info("myIntern UI serving", "addr", "http://localhost"+addr)
	if err := http.ListenAndServe(addr, api.NewMux(s, api.StaticHandler())); err != nil {
		slog.Error("serve", "err", err)
		os.Exit(1)
	}
}
