package api

import (
	"context"
	"log/slog"
	"os"
	"time"

	"myaudit/internal/agent"
	"myaudit/internal/events"
	"myaudit/internal/queue"
	"myaudit/internal/sandbox"
	"myaudit/internal/store"
	"myaudit/internal/worker"
)

// realAgent implements worker.Agent by driving the real Claude Code CLI with the
// default allow/deny tool policy. Model comes from CLAUDE_MODEL (default Haiku);
// AGENT_ISOLATE=1 runs claude inside a container.
type realAgent struct {
	store   *store.Store
	isolate bool
	image   string
}

func (a realAgent) Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode) (agent.Result, error) {
	allow, deny := agent.PolicyFor(mode)
	return agent.Run(ctx, ws, task, agent.Options{
		Model:   resolveModel(ctx, a.store),
		Allow:   allow,
		Deny:    deny,
		Isolate: a.isolate,
		Image:   a.image,
	})
}

// tierToModel maps the Settings "model tier" to a concrete claude model id.
var tierToModel = map[string]string{
	"opus-4.8":  "claude-opus-4-8",
	"sonnet-5":  "claude-sonnet-5",
	"haiku-4.5": "claude-haiku-4-5-20251001",
}

const defaultModel = "claude-haiku-4-5-20251001"

// resolveModel picks the agent model, read per run so the UI takes effect live:
// the CLAUDE_MODEL env override wins (so `make run CLAUDE_MODEL=…` still works),
// else the model tier the user picked in Settings, else the Haiku default.
func resolveModel(ctx context.Context, s *store.Store) string {
	if m := os.Getenv("CLAUDE_MODEL"); m != "" {
		return m
	}
	if s != nil {
		if st, err := s.GetSettings(ctx); err == nil {
			if tier, _ := st["model_tier"].(string); tier != "" {
				if id := tierToModel[tier]; id != "" {
					return id
				}
			}
		}
	}
	return defaultModel
}

// stubAgent is the $0 dev runner: it reports success without calling a model, so
// the UI/graph can be exercised without spending tokens.
type stubAgent struct{}

func (stubAgent) Run(ctx context.Context, ws sandbox.Workspace, task string, mode agent.Mode) (agent.Result, error) {
	return agent.Result{OK: true, Summary: "[stub] node skipped (dev mode)"}, nil
}

// NewRealDeps drives nodes with the REAL Claude Code agent.
func NewRealDeps(s *store.Store) worker.Deps {
	image := os.Getenv("AGENT_IMAGE")
	if image == "" {
		image = "myaudit-sandbox"
	}
	return worker.Deps{
		Store: s, Queue: queue.New(s.DB()), Log: events.New(s.DB()),
		Agent:         realAgent{store: s, isolate: os.Getenv("AGENT_ISOLATE") != "", image: image},
		WorkspaceRoot: "runs", MaxRepairs: 2,
	}
}

// NewStubDeps drives nodes with the $0 stub agent (dev/UI mode).
func NewStubDeps(s *store.Store) worker.Deps {
	return worker.Deps{
		Store: s, Queue: queue.New(s.DB()), Log: events.New(s.DB()),
		Agent:         stubAgent{},
		WorkspaceRoot: "runs", MaxRepairs: 0,
	}
}

// TickAll promotes ready nodes then processes one across all runs. Returns the
// number of nodes processed (0 or 1).
func TickAll(ctx context.Context, deps worker.Deps) (int, error) {
	if _, err := deps.Queue.PromoteReady(ctx); err != nil {
		return 0, err
	}
	did, err := worker.RunOnce(ctx, deps)
	if err != nil {
		return 0, err
	}
	// Mark any drained run finished and announce it (drives the runs list off the
	// perpetual "running" and gives the UI a completion signal to toast).
	if finished, ferr := deps.Store.FinalizeDrainedRuns(ctx); ferr == nil {
		for _, r := range finished {
			deps.Log.Log(ctx, events.Event{RunID: r.ID, Kind: "run." + r.Status, Msg: "audit " + r.Status})
		}
	}
	if did {
		return 1, nil
	}
	return 0, nil
}

// StartRunLoop ticks the graph forward until ctx is cancelled. Ticks run
// sequentially; after a node is processed it waits `every`, else polls gently.
// On start it recovers nodes left 'running' by a prior crash so restarts self-heal.
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
