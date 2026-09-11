package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"myaudit/internal/store"
)

func TestListRunsEndpoint(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	s.CreateRun(ctx, "acme-saas")

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var runs []store.RunSummary
	json.NewDecoder(resp.Body).Decode(&runs)
	if len(runs) != 1 || runs[0].Project != "acme-saas" {
		t.Fatalf("expected 1 run acme-saas, got %+v", runs)
	}
}

func TestRunDetailEndpoint(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	nid, _ := s.AddNode(ctx, run, "implement", nil)
	s.DB().ExecContext(ctx, `INSERT INTO events(run_id,node_id,level,kind,msg) VALUES(?,?,'info','node.start','go')`, run, nid)

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/runs/" + run.String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var d RunDetail
	json.NewDecoder(resp.Body).Decode(&d)
	if len(d.Nodes) != 1 || d.Nodes[0].Type != "implement" {
		t.Fatalf("expected 1 node, got %+v", d.Nodes)
	}
	if len(d.Events) != 1 || d.Events[0].Kind != "node.start" {
		t.Fatalf("expected 1 event, got %+v", d.Events)
	}
}

func TestRunDetailIncludesCheckpoints(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	defer s.Close()
	run, _ := s.CreateRun(ctx, "acme")
	nid, _ := s.AddNode(ctx, run, "implement", nil)
	s.DB().ExecContext(ctx, `UPDATE nodes SET status='running' WHERE id=?`, nid)
	s.RaiseCheckpoint(ctx, run, nid, "Google-only?", nil)

	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/api/runs/" + run.String())
	var d RunDetail
	json.NewDecoder(resp.Body).Decode(&d)
	if len(d.Checkpoints) != 1 || d.Checkpoints[0].Question != "Google-only?" {
		t.Fatalf("checkpoints: %+v", d.Checkpoints)
	}
}

func TestRunDetailBadID(t *testing.T) {
	s := newStore(t)
	defer s.Close()
	srv := httptest.NewServer(NewMux(s, nil))
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/api/runs/not-a-uuid")
	if resp.StatusCode != 400 {
		t.Fatalf("bad id should 400, got %d", resp.StatusCode)
	}
}
