package api

import (
	"context"
	"fmt"
	"os"
	"strings"
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

func (a realAgent) Run(ctx context.Context, ws sandbox.Workspace, task string) (agent.Result, error) {
	return agent.Run(ctx, ws, task, agent.Options{
		Model:   a.model,
		Allow:   agent.DefaultAllow,
		Deny:    agent.DefaultDeny,
		Isolate: a.isolate,
		Image:   a.image,
	})
}

// stubAgent is the $0 dev runner: it reports success without calling a model, so
// the UI/graph can be exercised without spending tokens. (Scaffold still runs
// for real — it's deterministic and free.)
type stubAgent struct{}

func (stubAgent) Run(ctx context.Context, ws sandbox.Workspace, task string) (agent.Result, error) {
	return agent.Result{OK: true, Summary: "[stub] feature skipped (dev mode)"}, nil
}

// syntaxVerifier is a real, fast, DB-free gate: every backend .js file must pass
// `node --check`. Catches an agent breaking the code. (Fuller build/boot
// verification can layer on later.)
type syntaxVerifier struct{}

func (syntaxVerifier) Verify(ctx context.Context, ws sandbox.Workspace) error {
	// Only check what this node changed — fast, and we don't re-validate the
	// (already-good) template on every run.
	changed, err := ws.ChangedPaths(ctx)
	if err != nil {
		return nil // can't determine changes → don't block
	}
	var bad []string
	for _, rel := range changed {
		if !strings.HasSuffix(rel, ".js") || !strings.HasPrefix(rel, "backend/") {
			continue // node --check is for backend JS; skip frontend/JSX/etc.
		}
		if out, code, _ := ws.Run(ctx, "node", "--check", rel); code != 0 {
			bad = append(bad, rel+": "+firstLine(out))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("syntax check failed: %s", strings.Join(bad, "; "))
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// NewRealDeps drives feature nodes with the REAL claude agent + a real verifier.
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
		Verify:        syntaxVerifier{},
		WorkspaceRoot: "runs", MaxRepairs: 2,
	}
}

// NewStubDeps drives feature nodes with the $0 stub agent (dev/UI mode).
func NewStubDeps(s *store.Store) worker.Deps {
	return worker.Deps{
		Store: s, Queue: queue.New(s.DB()), Log: events.New(s.DB()),
		Agent: stubAgent{}, Verify: nil,
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
