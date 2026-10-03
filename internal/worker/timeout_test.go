package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/queue"
	"github.com/codebyNJ/myAudit/internal/sandbox"
	"github.com/codebyNJ/myAudit/internal/store"
)

// untilDone stands in for an agent CLI still working when its context ends:
// killed, it returns the context's error.
type untilDone struct{}

func (untilDone) Run(ctx context.Context, _ sandbox.Workspace, _ string, _ agent.Mode, _ func(string)) (agent.Result, error) {
	<-ctx.Done()
	return agent.Result{}, ctx.Err()
}

func claimQA(t *testing.T, s *store.Store, d Deps) *queue.ClaimedNode {
	t.Helper()
	ctx := context.Background()
	run, _ := s.CreateRun(ctx, "p")
	if _, err := s.AddNodeFull(ctx, run, "qa", nil, map[string]any{"module": "m", "path": "."}, "ready"); err != nil {
		t.Fatal(err)
	}
	c, err := d.Queue.Claim(ctx)
	if err != nil || c == nil {
		t.Fatalf("claim: %v", err)
	}
	return c
}

func eventsOf(t *testing.T, s *store.Store, c *queue.ClaimedNode, kind string) (int, string) {
	t.Helper()
	var n int
	var msg string
	if err := s.DB().QueryRow(`SELECT count(*), coalesce(max(msg),'') FROM events WHERE node_id=? AND kind=?`,
		c.ID, kind).Scan(&n, &msg); err != nil {
		t.Fatal(err)
	}
	return n, msg
}

func TestTimedOutNodeIsRecordedAsFailed(t *testing.T) {
	s := newStore(t)
	d := newDeps(s, untilDone{}, t.TempDir())
	c := claimQA(t, s, d)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, ok := d.runAgent(ctx, c, sandbox.Workspace{Dir: t.TempDir()}, "task", agent.Live); ok {
		t.Fatal("a timed-out agent must not report success")
	}

	n, _ := s.GetNode(context.Background(), c.ID)
	if n.Status != "failed" {
		t.Fatalf("timed-out node left %q; it must be failed so the run's fixes are not blocked", n.Status)
	}
	if count, msg := eventsOf(t, s, c, "node.fail"); count != 1 || !strings.HasPrefix(msg, "timed out") {
		t.Fatalf("want one node.fail saying it timed out, got %d %q", count, msg)
	}
}

func TestUserCancelledNodeIsRecordedAsCancelled(t *testing.T) {
	s := newStore(t)
	d := newDeps(s, untilDone{}, t.TempDir())
	c := claimQA(t, s, d)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	d.runAgent(ctx, c, sandbox.Workspace{Dir: t.TempDir()}, "task", agent.Live)

	n, _ := s.GetNode(context.Background(), c.ID)
	if n.Status != "cancelled" {
		t.Fatalf("want cancelled, got %q", n.Status)
	}
	if count, _ := eventsOf(t, s, c, "node.cancelled"); count != 1 {
		t.Fatalf("want one node.cancelled event, got %d", count)
	}
}
