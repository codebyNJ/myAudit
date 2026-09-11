package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/codebyNJ/myAudit/internal/api"
	"github.com/codebyNJ/myAudit/internal/config"
	"github.com/codebyNJ/myAudit/internal/store"
	"github.com/codebyNJ/myAudit/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}
	s, err := store.Open(context.Background(), cfg.DBPath)
	if err != nil {
		slog.Error("store", "err", err)
		os.Exit(1)
	}
	defer s.Close()

	var deps worker.Deps
	if os.Getenv("REAL_CLAUDE") != "" {
		deps = api.NewRealDeps(s)
		slog.Info("run loop: REAL claude-code agent mode")
	} else {
		deps = api.NewStubDeps(s)
		slog.Info("run loop: stub mode ($0 agent; scaffold real)")
	}

	pace := time.Second
	if os.Getenv("REAL_CLAUDE") != "" {
		pace = 2 * time.Second
	}
	if ms, _ := strconv.Atoi(os.Getenv("RUN_PACE_MS")); ms > 0 {
		pace = time.Duration(ms) * time.Millisecond
	}
	go api.StartRunLoop(context.Background(), deps, pace)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		api.Previews.StopAll()
		os.Exit(0)
	}()
	defer api.Previews.StopAll()

	addr := ":7788"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	slog.Info("myAudit UI serving", "addr", "http://localhost"+addr)
	if err := http.ListenAndServe(addr, api.NewMux(s, api.StaticHandler())); err != nil {
		if errors.Is(err, syscall.EADDRINUSE) {
			slog.Error("port already in use — another myAudit (or app) is on "+addr+
				". Stop it, or set PORT to a free port (e.g. PORT=7799).", "err", err)
		} else {
			slog.Error("serve", "err", err)
		}
		os.Exit(1)
	}
}
