package api

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
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
	maxConcurrent := resolveMaxConcurrent()
	setAgentLimit(maxConcurrent)
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
	setAgentLimit(1)
	return worker.Deps{
		Store: s, Queue: queue.New(s.DB()), Log: events.New(s.DB()),
		Agent:         stubAgent{},
		WorkspaceRoot: "runs", MaxRepairs: 0, MaxConcurrent: 1,
	}
}

// prepare promotes nodes whose dependencies are met and parks over-budget runs.
func prepare(ctx context.Context, deps worker.Deps) error {
	if _, err := deps.Queue.PromoteReady(ctx); err != nil {
		return err
	}
	if paused, err := deps.Store.PauseOverBudget(ctx); err == nil {
		for _, id := range paused {
			deps.Log.Log(ctx, events.Event{RunID: id, Kind: "run.paused", Msg: "budget reached — parked remaining work"})
		}
	}
	return nil
}

// dispatch claims ready nodes while slots has room and runs each on its own
// goroutine, freeing its slot the moment that node ends. Waiting for a whole
// batch instead left slots idle behind the slowest node — up to a 20-minute
// fix — while other work sat ready.
func dispatch(ctx context.Context, deps worker.Deps, slots chan struct{}, done func(error)) error {
	for {
		select {
		case slots <- struct{}{}:
		default:
			return nil // every slot busy
		}
		c, err := deps.Queue.Claim(ctx)
		if err != nil || c == nil {
			<-slots
			return err
		}
		go func() {
			defer func() { <-slots }()
			done(deps.ProcessClaimed(ctx, c))
		}()
	}
}

// finalize closes runs with nothing left to do and reclaims their caches.
func finalize(ctx context.Context, deps worker.Deps) {
	finished, err := deps.Store.FinalizeDrainedRuns(ctx)
	if err != nil {
		return
	}
	for _, r := range finished {
		deps.Log.Log(ctx, events.Event{RunID: r.ID, Kind: "run." + r.Status, Msg: "audit " + r.Status})
		if freed, rerr := sandbox.Reclaim(filepath.Join(deps.WorkspaceRoot, r.ID.String())); rerr == nil && freed > 0 {
			deps.Log.Log(ctx, events.Event{RunID: r.ID, Kind: "run.reclaim",
				Msg: fmt.Sprintf("reclaimed %.0f MB of dependency caches", float64(freed)/(1<<20))})
		}
	}
}

// TickAll is one synchronous pass: claim what is ready, wait for it, finalize.
func TickAll(ctx context.Context, deps worker.Deps) (int, error) {
	if err := prepare(ctx, deps); err != nil {
		return 0, err
	}
	n := max(deps.MaxConcurrent, 1)
	slots := make(chan struct{}, n)
	var processed atomic.Int64
	err := dispatch(ctx, deps, slots, func(e error) {
		if e == nil {
			processed.Add(1)
		}
	})
	for range n { // each slot comes back once its node has ended
		slots <- struct{}{}
	}
	finalize(ctx, deps)
	return int(processed.Load()), err
}

func StartRunLoop(ctx context.Context, deps worker.Deps, every time.Duration) {
	if n, err := deps.Queue.RecoverStuck(ctx, 3); err == nil && n > 0 {
		slog.Info("recovered stuck nodes on startup", "count", n)
	}
	slots := make(chan struct{}, max(deps.MaxConcurrent, 1))
	for ctx.Err() == nil {
		if prepare(ctx, deps) == nil {
			_ = dispatch(ctx, deps, slots, func(error) {})
		}
		finalize(ctx, deps)
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}
