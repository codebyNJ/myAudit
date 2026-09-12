package api

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/events"
	"github.com/codebyNJ/myAudit/internal/queue"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
	"github.com/codebyNJ/myAudit/internal/worker"
)

type realAgent struct {
	store       *store.Store
	isolate     bool
	image       string
	claudeBin   string
	opencodeBin string
}

func (a realAgent) Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode, onStep func(string)) (agent.Result, error) {
	return runAgent(ctx, a.store, ws, task, mode, a.isolate, a.image, a.claudeBin, a.opencodeBin, onStep)
}

type stubAgent struct{}

func (stubAgent) Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode, onStep func(string)) (agent.Result, error) {
	return agent.Result{OK: true, Summary: "[stub] node skipped (dev mode)"}, nil
}

func NewRealDeps(s *store.Store) worker.Deps {
	image := os.Getenv("AGENT_IMAGE")
	if image == "" {
		image = "myaudit-sandbox"
	}
	maxConcurrent := 1
	if n, err := strconv.Atoi(os.Getenv("MAX_CONCURRENT_CLAUDE")); err == nil && n > 0 {
		maxConcurrent = n
	}
	return worker.Deps{
		Store: s, Queue: queue.New(s.DB()), Log: events.New(s.DB()),
		Agent: realAgent{
			store: s, isolate: os.Getenv("AGENT_ISOLATE") != "", image: image,
			claudeBin: os.Getenv("CLAUDE_BIN"), opencodeBin: os.Getenv("OPENCODE_BIN"),
		},
		WorkspaceRoot: "runs", MaxRepairs: 2, MaxConcurrent: maxConcurrent,
	}
}

func NewStubDeps(s *store.Store) worker.Deps {
	return worker.Deps{
		Store: s, Queue: queue.New(s.DB()), Log: events.New(s.DB()),
		Agent:         stubAgent{},
		WorkspaceRoot: "runs", MaxRepairs: 0, MaxConcurrent: 1,
	}
}

func TickAll(ctx context.Context, deps worker.Deps) (int, error) {
	if _, err := deps.Queue.PromoteReady(ctx); err != nil {
		return 0, err
	}

	if paused, err := deps.Store.PauseOverBudget(ctx); err == nil {
		for _, id := range paused {
			deps.Log.Log(ctx, events.Event{RunID: id, Kind: "run.paused", Msg: "budget reached — parked remaining work"})
		}
	}

	n := deps.MaxConcurrent
	if n < 1 {
		n = 1
	}
	claimed, err := deps.Queue.ClaimN(ctx, n)
	if err != nil {
		return 0, err
	}

	processed := 0
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, c := range claimed {
		wg.Add(1)
		go func(c *queue.ClaimedNode) {
			defer wg.Done()
			if procErr := deps.ProcessClaimed(ctx, c); procErr == nil {
				mu.Lock()
				processed++
				mu.Unlock()
			}
		}(c)
	}
	wg.Wait()

	if finished, ferr := deps.Store.FinalizeDrainedRuns(ctx); ferr == nil {
		for _, r := range finished {
			deps.Log.Log(ctx, events.Event{RunID: r.ID, Kind: "run." + r.Status, Msg: "audit " + r.Status})

			if freed, rerr := sandbox.Reclaim(filepath.Join(deps.WorkspaceRoot, r.ID.String())); rerr == nil && freed > 0 {
				deps.Log.Log(ctx, events.Event{RunID: r.ID, Kind: "run.reclaim",
					Msg: fmt.Sprintf("reclaimed %.0f MB of dependency caches", float64(freed)/(1<<20))})
			}
		}
	}
	return processed, nil
}

func StartRunLoop(ctx context.Context, deps worker.Deps, every time.Duration) {
	if n, err := deps.Queue.RecoverStuck(ctx, 3); err == nil && n > 0 {
		slog.Info("recovered stuck nodes on startup", "count", n)
	}
	for {
		if ctx.Err() != nil {
			return
		}
		n, _ := TickAll(ctx, deps)
		wait := every
		if n == 0 {
			wait = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
	}
}
