package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

// gated holds the QA for module "slow" until release closes; every other
// module's QA finishes at once with no findings.
type gated struct{ release chan struct{} }

func (g gated) Run(ctx context.Context, _ sandbox.Workspace, task string, _ agent.Mode, _ func(string)) (agent.Result, error) {
	if strings.Contains(task, `module "slow"`) {
		select {
		case <-g.release:
		case <-ctx.Done():
		}
	}
	return agent.Result{OK: true, Summary: "[]"}, nil
}

func waitStatus(t *testing.T, s *store.Store, id uuid.UUID, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if n, _ := s.GetNode(context.Background(), id); n.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	n, _ := s.GetNode(context.Background(), id)
	t.Fatalf("node %s: want %q, still %q", id, want, n.Status)
}

func TestRunLoopReusesAFreedSlotWithoutWaitingForTheBatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")
	qa := func(module string) uuid.UUID {
		id, err := s.AddNodeFull(ctx, run, "qa", nil, map[string]any{"module": module, "path": "."}, "ready")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	slow, fast := qa("slow"), qa("fast")

	g := gated{release: make(chan struct{})}
	defer close(g.release)
	deps := NewStubDeps(s)
	deps.Agent = g
	deps.WorkspaceRoot = t.TempDir()
	deps.MaxConcurrent = 2
	go StartRunLoop(ctx, deps, 10*time.Millisecond)

	waitStatus(t, s, fast, "done")
	late := qa("late") // ready after the first two were claimed
	waitStatus(t, s, late, "done")
	if n, _ := s.GetNode(ctx, slow); n.Status != "running" {
		t.Fatalf("slow should still hold its slot, got %q", n.Status)
	}
}
