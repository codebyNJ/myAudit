package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/codebyNJ/myAudit/internal/agent"
	"github.com/codebyNJ/myAudit/internal/sandbox"
)

func TestDoFlowsRoutesThroughRunAgentRetry(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")

	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "x.go"), []byte("package x\n"), 0o644)
	root := t.TempDir()
	if _, err := sandbox.Import(ctx, root, run.String(), src); err != nil {
		t.Fatal(err)
	}

	flowsID, _ := s.AddNode(ctx, run, "flows", nil)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, flowsID)

	fake := &recordingAgent{result: agent.Result{OK: false, Err: "model refused"}}
	deps := newDeps(s, fake, root)
	if _, err := RunOnce(ctx, deps); err != nil {
		t.Fatal(err)
	}

	if fake.calls != 3 {
		t.Fatalf("expected runAgent to retry MaxRepairs+1=3 times, got %d calls", fake.calls)
	}
	nodes := nodesOfType(t, s, run, "flows")
	if len(nodes) != 1 {
		t.Fatalf("expected 1 flows node, got %d", len(nodes))
	}
	if nodes[0].Status != "blocked" {
		t.Fatalf("exhausted retries should raise a checkpoint (status=blocked), got %q", nodes[0].Status)
	}
	if fake.lastMode != agent.ReadOnly {
		t.Fatalf("flows must stay ReadOnly mode, got %v", fake.lastMode)
	}
}

func TestDoFlowsSucceedsAndParses(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	run, _ := s.CreateRun(ctx, "proj")

	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "x.go"), []byte("package x\n"), 0o644)
	root := t.TempDir()
	if _, err := sandbox.Import(ctx, root, run.String(), src); err != nil {
		t.Fatal(err)
	}

	flowsID, _ := s.AddNode(ctx, run, "flows", nil)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='ready' WHERE id=?`, flowsID)

	fake := &recordingAgent{result: agent.Result{OK: true,
		Summary: `{"persistence":"sqlite","data_flows":[{"name":"A"}]}`}}
	deps := newDeps(s, fake, root)
	if _, err := RunOnce(ctx, deps); err != nil {
		t.Fatal(err)
	}

	if fake.calls != 1 {
		t.Fatalf("a successful first attempt should not retry, got %d calls", fake.calls)
	}
	nodes := nodesOfType(t, s, run, "flows")
	if len(nodes) != 1 || nodes[0].Status != "done" {
		t.Fatalf("expected 1 done flows node, got %+v", nodes)
	}
}

func TestParseFlowsPlainJSON(t *testing.T) {
	doc, err := parseFlows(`{"persistence":"none — static JSON","data_flows":[{"name":"Playlist","store":"static file","steps":[{"label":"load","file":"lib/p.ts:3","kind":"source"}]}],"product_flows":[{"name":"Play","steps":[{"label":"click","file":"app/page.tsx:9"}]}]}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Persistence == "" || len(doc.DataFlows) != 1 || len(doc.ProductFlows) != 1 {
		t.Fatalf("bad parse: %+v", doc)
	}
	if doc.DataFlows[0].Steps[0].File != "lib/p.ts:3" {
		t.Fatalf("step file lost: %+v", doc.DataFlows[0])
	}
}

func TestParseFlowsFencedAndProse(t *testing.T) {
	in := "Here is the map:\n```json\n{\"data_flows\":[{\"name\":\"A\"}]}\n```\nHope that helps."
	doc, err := parseFlows(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.DataFlows) != 1 || doc.DataFlows[0].Name != "A" {
		t.Fatalf("bad parse: %+v", doc)
	}
}

func TestParseFlowsRejectsNonJSON(t *testing.T) {
	if _, err := parseFlows("[stub] node skipped (dev mode)"); err == nil {
		t.Fatal("expected an error for non-JSON agent output")
	}
}

func TestParseFlowsRejectsEmptyDoc(t *testing.T) {
	if _, err := parseFlows(`{"persistence":"sqlite"}`); err == nil {
		t.Fatal("expected an error when no flows were identified")
	}
}
