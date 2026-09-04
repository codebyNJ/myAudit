package api

import (
	"context"
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
	model   string
	isolate bool
	image   string
}

func (a realAgent) Run(ctx context.Context, ws sandbox.Workspace, task string, readOnly bool) (agent.Result, error) {
	allow, deny := agent.DefaultAllow, agent.DefaultDeny
	if readOnly {
		allow, deny = agent.ReadOnlyAllow, agent.ReadOnlyDeny
	}
	return agent.Run(ctx, ws, task, agent.Options{
		Model:   a.model,
		Allow:   allow,
		Deny:    deny,
		Isolate: a.isolate,
		Image:   a.image,
	})
}

// stubAgent is the $0 dev runner: it reports success without calling a model, so
// the UI/graph can be exercised without spending tokens.
type stubAgent struct{}

func (stubAgent) Run(ctx context.Context, ws sandbox.Workspace, task string, readOnly bool) (agent.Result, error) {
	return agent.Result{OK: true, Summary: "[stub] node skipped (dev mode)"}, nil
}

// NewRealDeps drives nodes with the REAL Claude Code agent.
func NewRealDeps(s *store.Store) worker.Deps {
	model := os.Getenv("CLAUDE_MODEL")
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	image := os.Getenv("AGENT_IMAGE")
	if image == "" {
		image = "myaudit-sandbox"
	}
	return worker.Deps{
		Store: s, Queue: queue.New(s.DB()), Log: events.New(s.DB()),
		Agent:         realAgent{model: model, isolate: os.Getenv("AGENT_ISOLATE") != "", image: image},
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
	if did {
		return 1, nil
	}
	return 0, nil
}

// StartRunLoop ticks the graph forward until ctx is cancelled. Ticks run
// sequentially; after a node is processed it waits `every`, else polls gently.
func StartRunLoop(ctx context.Context, deps worker.Deps, every time.Duration) {
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
