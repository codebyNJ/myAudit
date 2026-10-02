package worker

import (
	"context"
	"math"
	"testing"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/sandbox"
)

// scripted returns its results in order, one per attempt.
type scripted struct {
	results []agent.Result
	calls   int
}

func (a *scripted) Run(context.Context, sandbox.Workspace, string, agent.Mode, func(string)) (agent.Result, error) {
	r := a.results[a.calls]
	a.calls++
	return r, nil
}

func attempts(ok ...bool) *scripted {
	a := &scripted{}
	for _, o := range ok {
		a.results = append(a.results, agent.Result{OK: o, Err: "max turns", Summary: "x", CostUSD: 0.10, Tokens: 100})
	}
	return a
}

func TestRunAgentBillsEveryAttempt(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	d := newDeps(s, attempts(false, false, true), t.TempDir())
	c := claimQA(t, s, d)

	r, ok := d.runAgent(ctx, c, sandbox.Workspace{Dir: t.TempDir()}, "task", agent.Live)
	if !ok || math.Abs(r.CostUSD-0.30) > 1e-9 || r.Tokens != 300 {
		t.Fatalf("want ok with $0.30 / 300 tokens across attempts, got ok=%v %+v", ok, r)
	}
	if spent, _ := s.RunCostUSD(ctx, c.RunID); math.Abs(spent-0.30) > 1e-9 {
		t.Fatalf("run cost %v, want 0.30: the two failed attempts must be billed", spent)
	}
}

// A node that ends in a checkpoint writes no output; its spend must still count.
func TestCheckpointedNodeIsStillBilled(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	d := newDeps(s, attempts(false, false, false), t.TempDir())
	c := claimQA(t, s, d)

	if _, ok := d.runAgent(ctx, c, sandbox.Workspace{Dir: t.TempDir()}, "task", agent.Live); ok {
		t.Fatal("three failed attempts must not succeed")
	}
	if spent, _ := s.RunCostUSD(ctx, c.RunID); math.Abs(spent-0.30) > 1e-9 {
		t.Fatalf("run cost %v, want 0.30", spent)
	}
}
